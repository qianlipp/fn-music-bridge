package api

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadinessThroughUnixSocketUpstream(t *testing.T) {
	tempDir, err := os.MkdirTemp("/tmp", "fm-api-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
	socketPath := filepath.Join(tempDir, "music.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	upstream := &http.Server{
		Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path != "/music/api/v1/initialization/state" {
				t.Errorf("upstream path = %q", request.URL.Path)
			}
			if request.Header.Get("authx") == "" {
				t.Error("missing authx signature")
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{"code":0,"data":{"initialized":true}}`)
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = upstream.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = upstream.Close()
	})

	config := Config{
		PublicPrefix:       "/yinqiao/v1",
		UpstreamOrigin:     "http://unix",
		UpstreamAPIBase:    "/music/api/v1",
		UpstreamUnixSocket: socketPath,
		SigningPrefix:      defaultSigningPrefix,
		ClientKey:          defaultClientKey,
		AllowedOrigins:     []string{"*"},
		MaxBodyBytes:       1 << 20,
		UpstreamTimeout:    5 * time.Second,
	}
	apiServer, err := NewServer(config, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/yinqiao/v1/ready", nil)
	response := httptest.NewRecorder()
	apiServer.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
