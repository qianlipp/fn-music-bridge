package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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

func TestPasswordLoginUsesManualCustomGatewayPort(t *testing.T) {
	loginCalls := 0
	gateway := newTestFNOSGateway(t, &loginCalls)
	client := newTestUpstreamClient(t, gateway.URL, nil)
	incoming := httptest.NewRequest(http.MethodPost, "/", nil)
	incoming.Header.Set(fnosGatewayHeader, gateway.URL)
	response, err := client.Do(context.Background(), incoming, Route{ID: "auth.login.password", UpstreamMethod: http.MethodPost}, "/user/password-login", nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if loginCalls != 1 || response.Header.Get("X-Yinqiao-Upstream") != "fnos-web-gateway" {
		t.Fatalf("manual gateway login calls=%d marker=%q", loginCalls, response.Header.Get("X-Yinqiao-Upstream"))
	}
}

func TestPasswordLoginUsesManualCustomHTTPSGateway(t *testing.T) {
	loginCalls := 0
	gateway := httptest.NewTLSServer(testFNOSGatewayHandler(t, &loginCalls))
	t.Cleanup(gateway.Close)
	client := newTestUpstreamClient(t, gateway.URL, nil)
	incoming := httptest.NewRequest(http.MethodPost, "/", nil)
	incoming.Header.Set(fnosGatewayHeader, gateway.URL)
	response, err := client.Do(context.Background(), incoming, Route{ID: "auth.login.password", UpstreamMethod: http.MethodPost}, "/user/password-login", nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if loginCalls != 1 {
		t.Fatalf("HTTPS gateway login calls=%d", loginCalls)
	}
}

func TestPasswordLoginDiscoversCustomizedListeningPort(t *testing.T) {
	loginCalls := 0
	gateway := newTestFNOSGateway(t, &loginCalls)
	parsed, _ := url.Parse(gateway.URL)
	port, _ := strconv.Atoi(parsed.Port())
	client := newTestUpstreamClient(t, gateway.URL, nil)
	client.listeningPorts = func() []int { return []int{port} }
	response, err := client.Do(context.Background(), httptest.NewRequest(http.MethodPost, "/", nil), Route{ID: "auth.login.password", UpstreamMethod: http.MethodPost}, "/user/password-login", nil, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if loginCalls != 1 {
		t.Fatalf("discovered gateway login calls=%d", loginCalls)
	}
}

func TestGatewayDiagnosticsReportsCandidatesAndSelection(t *testing.T) {
	loginCalls := 0
	gateway := newTestFNOSGateway(t, &loginCalls)
	parsed, _ := url.Parse(gateway.URL)
	port, _ := strconv.Atoi(parsed.Port())
	client := newTestUpstreamClient(t, gateway.URL, nil)
	client.listeningPorts = func() []int { return []int{port} }
	report := client.GatewayDiagnostics(context.Background(), "")
	if report.SelectedOrigin == "" || len(report.Candidates) != 1 || !report.Candidates[0].Available || !report.Candidates[0].Selected {
		t.Fatalf("unexpected diagnostics: %#v", report)
	}
}

func newTestFNOSGateway(t *testing.T, loginCalls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(testFNOSGatewayHandler(t, loginCalls))
	t.Cleanup(server.Close)
	return server
}

func testFNOSGatewayHandler(t *testing.T, loginCalls *int) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/music/api/v1/initialization/state":
			_, _ = io.WriteString(writer, `{"code":0,"data":{"initialized":true}}`)
		case "/music/api/v1/user/password-login":
			(*loginCalls)++
			_, _ = io.WriteString(writer, `{"code":0,"data":{"userToken":"token"}}`)
		default:
			http.NotFound(writer, request)
		}
	})
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
