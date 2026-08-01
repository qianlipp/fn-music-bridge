package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPasswordLoginUsesLocalFNOSGateway(t *testing.T) {
	directCalls := 0
	direct := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		directCalls++
		http.Error(writer, "direct upstream must not receive a discovered gateway login", http.StatusInternalServerError)
	}))
	t.Cleanup(direct.Close)

	probeCalls := 0
	loginCalls := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("authx") == "" {
			t.Error("missing authx signature")
		}
		switch request.URL.Path {
		case "/music/api/v1/initialization/state":
			probeCalls++
			_, _ = io.WriteString(writer, `{"code":0,"data":{"initialized":true}}`)
		case "/music/api/v1/user/password-login":
			loginCalls++
			if request.Header.Get("X-Music-API") != "" {
				t.Error("gateway login must match the browser request surface")
			}
			_, _ = io.WriteString(writer, `{"code":0,"data":{"userToken":"token"}}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(gateway.Close)

	client := newTestUpstreamClient(t, direct.URL, []string{gateway.URL})
	route := Route{ID: "auth.login.password", UpstreamMethod: http.MethodPost}
	incoming := httptest.NewRequest(http.MethodPost, "/", nil)
	for range 2 {
		response, err := client.Do(context.Background(), incoming, route, "/user/password-login", nil, []byte(`{"username":"user"}`))
		if err != nil {
			t.Fatal(err)
		}
		if got := response.Header.Get("X-Yinqiao-Upstream"); got != "fnos-web-gateway" {
			t.Fatalf("upstream marker = %q", got)
		}
		_ = response.Body.Close()
	}

	if probeCalls != 1 || loginCalls != 2 || directCalls != 0 {
		t.Fatalf("probe/login/direct calls = %d/%d/%d", probeCalls, loginCalls, directCalls)
	}
}

func TestNonLoginRouteKeepsDirectUpstream(t *testing.T) {
	directCalls := 0
	direct := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		directCalls++
		_, _ = io.WriteString(writer, `{"code":0,"data":{}}`)
	}))
	t.Cleanup(direct.Close)

	gatewayCalls := 0
	gateway := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gatewayCalls++
		_, _ = io.WriteString(writer, `{"code":0,"data":{}}`)
	}))
	t.Cleanup(gateway.Close)

	client := newTestUpstreamClient(t, direct.URL, []string{gateway.URL})
	route := Route{ID: "tracks.list", UpstreamMethod: http.MethodGet}
	response, err := client.Do(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil), route, "/track/list", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	if directCalls != 1 || gatewayCalls != 0 {
		t.Fatalf("direct/gateway calls = %d/%d", directCalls, gatewayCalls)
	}
}

func newTestUpstreamClient(t *testing.T, origin string, gateways []string) *UpstreamClient {
	t.Helper()
	client, err := NewUpstreamClient(Config{
		UpstreamOrigin:   origin,
		UpstreamAPIBase:  "/music/api/v1",
		UpstreamGateways: gateways,
		SigningPrefix:    defaultSigningPrefix,
		ClientKey:        defaultClientKey,
		UpstreamTimeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
