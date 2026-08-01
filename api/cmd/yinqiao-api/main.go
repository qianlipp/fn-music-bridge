package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"yinqiao/api/internal/api"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config := api.ConfigFromEnv()
	server, err := api.NewServer(config, logger)
	if err != nil {
		logger.Error("failed to configure Yinqiao API", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              config.ListenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	listener, cleanup, err := listen(config)
	if err != nil {
		logger.Error("failed to open Yinqiao API listener", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("Yinqiao API listening", "listener", listener.Addr().String(), "prefix", config.PublicPrefix, "upstream", config.UpstreamOrigin)
	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		logger.Error("Yinqiao API stopped", "error", err)
		os.Exit(1)
	}
}

func listen(config api.Config) (net.Listener, func(), error) {
	if config.ListenUnixSocket == "" {
		listener, err := net.Listen("tcp", config.ListenAddr)
		return listener, func() {}, err
	}

	socketPath := filepath.Clean(config.ListenUnixSocket)
	if !filepath.IsAbs(socketPath) {
		return nil, func() {}, errors.New("YINQIAO_API_UNIX_SOCKET must be an absolute path")
	}
	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, func() {}, errors.New("refusing to replace a non-socket listener path")
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, func() {}, err
		}
	} else if !os.IsNotExist(err) {
		return nil, func() {}, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}
	return listener, cleanup, nil
}
