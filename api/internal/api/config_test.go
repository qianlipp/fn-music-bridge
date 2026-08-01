package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPackagedLANDefaults(t *testing.T) {
	for _, key := range []string{
		"YINQIAO_API_LISTEN",
		"YINQIAO_API_UNIX_SOCKET",
		"YINQIAO_API_PREFIX",
		"YINQIAO_UPSTREAM_ORIGIN",
		"YINQIAO_UPSTREAM_API_BASE",
		"YINQIAO_UPSTREAM_UNIX_SOCKET",
		"YINQIAO_UPSTREAM_GATEWAYS",
		"YINQIAO_UPSTREAM_ROUTE_MAP",
		"YINQIAO_UPSTREAM_SIGNING_PREFIX",
		"YINQIAO_UPSTREAM_CLIENT_KEY",
		"YINQIAO_API_KEY",
		"YINQIAO_API_ALLOWED_ORIGINS",
		"YINQIAO_API_PROXY_MUSIC_ROOT",
	} {
		t.Setenv(key, "")
	}

	config := ConfigFromEnv()
	if config.ListenAddr != "0.0.0.0:18090" {
		t.Fatalf("ListenAddr = %q", config.ListenAddr)
	}
	if config.PublicPrefix != "/yinqiao/v1" {
		t.Fatalf("PublicPrefix = %q", config.PublicPrefix)
	}
	if config.UpstreamUnixSocket != "/var/run/trim_music.socket" {
		t.Fatalf("UpstreamUnixSocket = %q", config.UpstreamUnixSocket)
	}
	if len(config.UpstreamGateways) != 2 || config.UpstreamGateways[0] != "http://127.0.0.1:5666" || config.UpstreamGateways[1] != "http://127.0.0.1:8000" {
		t.Fatalf("UpstreamGateways = %#v", config.UpstreamGateways)
	}
	if config.APIKey != "" {
		t.Fatal("packaged LAN api must not require a Yinqiao API key")
	}
	if len(config.AllowedOrigins) != 1 || config.AllowedOrigins[0] != "*" {
		t.Fatalf("AllowedOrigins = %#v", config.AllowedOrigins)
	}
	if config.ProxyMusicRoot {
		t.Fatal("LAN api must not proxy the original /music frontend")
	}
}

func TestPackagedLANDefaultsAcceptRequestWithoutAPIKey(t *testing.T) {
	t.Setenv("YINQIAO_API_KEY", "")
	t.Setenv("YINQIAO_API_ALLOWED_ORIGINS", "")
	config := ConfigFromEnv()
	server, err := NewServer(config, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/yinqiao/v1/capabilities", nil)
	request.Header.Set("Origin", "http://lan-client.local")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://lan-client.local" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}
