package main

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"yinqiao/api/internal/api"
)

func TestListenUnixSocket(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "api.sock")
	listener, cleanup, err := listen(api.Config{ListenUnixSocket: socketPath})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if listener.Addr().Network() != "unix" {
		t.Fatalf("network = %q, want unix", listener.Addr().Network())
	}
	cleanup()
	if _, err := os.Lstat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("socket still exists after cleanup: %v", err)
	}
}

func TestListenRefusesRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.sock")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if listener, _, err := listen(api.Config{ListenUnixSocket: path}); err == nil {
		_ = listener.Close()
		t.Fatal("listen replaced a regular file")
	}
}

func TestListenTCP(t *testing.T) {
	listener, cleanup, err := listen(api.Config{ListenAddr: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer cleanup()
	defer listener.Close()
	if listener.Addr().Network() != "tcp" {
		t.Fatalf("network = %q, want tcp", listener.Addr().Network())
	}
	if _, ok := listener.Addr().(*net.TCPAddr); !ok {
		t.Fatalf("address type = %T, want *net.TCPAddr", listener.Addr())
	}
}
