package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"yinqiao/service/internal/service"
)

type protocolListenerManager struct {
	mu             sync.Mutex
	managementPort int
	handler        http.Handler
	server         *http.Server
	listener       net.Listener
	port           int
	logger         *log.Logger
}

func newProtocolListenerManager(managementAddress string, logger *log.Logger) *protocolListenerManager {
	_, rawPort, _ := net.SplitHostPort(managementAddress)
	port, _ := strconv.Atoi(rawPort)
	if port == 0 {
		port = 14040
	}
	return &protocolListenerManager{managementPort: port, port: port, logger: logger}
}

func (m *protocolListenerManager) setHandler(handler http.Handler) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

func (m *protocolListenerManager) apply(port int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if port == m.managementPort {
		m.closeSecondaryLocked()
		m.port = port
		return nil
	}
	if port == m.port && m.listener != nil {
		return nil
	}
	if m.handler == nil {
		return errors.New("protocol handler is not ready")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	server := &http.Server{
		Handler:           m.handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	oldServer, oldListener := m.server, m.listener
	m.server, m.listener, m.port = server, listener, port
	go func() {
		m.logger.Printf("client protocols listening on 0.0.0.0:%d", port)
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			m.logger.Printf("client protocol listener port=%d error=%v", port, serveErr)
		}
	}()
	if oldServer != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = oldServer.Shutdown(ctx)
		}()
	} else if oldListener != nil {
		_ = oldListener.Close()
	}
	return nil
}

func (m *protocolListenerManager) closeSecondaryLocked() {
	if m.server != nil {
		_ = m.server.Close()
	} else if m.listener != nil {
		_ = m.listener.Close()
	}
	m.server, m.listener = nil, nil
}

func (m *protocolListenerManager) shutdown(ctx context.Context) {
	m.mu.Lock()
	server := m.server
	m.server, m.listener = nil, nil
	m.mu.Unlock()
	if server != nil {
		_ = server.Shutdown(ctx)
	}
}

func main() {
	logger := log.New(os.Stdout, "yinqiao-service ", log.LstdFlags)
	config := service.ConfigFromEnv()
	listenerManager := newProtocolListenerManager(config.ListenAddr, logger)
	config.ApplyProtocolPort = listenerManager.apply
	handler, err := service.NewServer(config, logger)
	if err != nil {
		logger.Fatalf("initialize: %v", err)
	}
	listenerManager.setHandler(handler.Handler())
	if err := listenerManager.apply(handler.ProtocolPort()); err != nil {
		logger.Printf("configured client port %d unavailable: %v; management port remains available", handler.ProtocolPort(), err)
	}
	server := &http.Server{
		Addr:              config.ListenAddr,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		listenerManager.shutdown(ctx)
		_ = server.Shutdown(ctx)
	}()

	logger.Printf("management listening on %s, API %s", config.ListenAddr, config.APIBaseURL)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("serve: %v", fmt.Errorf("management listener: %w", err))
	}
}
