package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testAPI(t *testing.T, upstream http.Handler) (*Server, *httptest.Server) {
	t.Helper()
	upstreamServer := httptest.NewServer(upstream)
	t.Cleanup(upstreamServer.Close)
	config := Config{
		ListenAddr:      "127.0.0.1:0",
		PublicPrefix:    "/yinqiao/v1",
		UpstreamOrigin:  upstreamServer.URL,
		UpstreamAPIBase: "/music/api/v1",
		SigningPrefix:   defaultSigningPrefix,
		ClientKey:       defaultClientKey,
		MaxBodyBytes:    1 << 20,
		UpstreamTimeout: 5 * time.Second,
		ProxyMusicRoot:  false,
	}
	server, err := NewServer(config, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return server, upstreamServer
}

func TestPasswordLoginAdaptsPasswordAndBearerCookie(t *testing.T) {
	upstream := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/music/api/v1/user/password-login" {
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
		if request.Header.Get("authx") == "" {
			t.Fatal("missing upstream authx signature")
		}
		if got, want := request.Header.Get("Cookie"), "other=value; music-token=native-token"; got != want {
			t.Fatalf("cookie = %q, want %q", got, want)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte("secret123"))
		if got, want := body["password"], hex.EncodeToString(digest[:]); got != want {
			t.Fatalf("password = %v, want %v", got, want)
		}
		writer.Header().Add("Set-Cookie", "music-token=upstream-token; Path=/; SameSite=Strict")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"code":0,"msg":"","data":{"userToken":"upstream-token","user":{"guid":"u1","role":"admin"}}}`)
	})
	server, _ := testAPI(t, upstream)

	request := httptest.NewRequest(http.MethodPost, "/yinqiao/v1/auth/login/password", strings.NewReader(`{"username":"admin","password":"secret123","deviceId":"device"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", "other=value; music-token=stale-token")
	request.Header.Set("Authorization", "Bearer native-token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if len(response.Header().Values("Set-Cookie")) != 1 {
		t.Fatalf("Set-Cookie not forwarded: %#v", response.Header())
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			UserToken string `json:"userToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data.UserToken != "upstream-token" {
		t.Fatalf("unexpected api response: %s", response.Body.String())
	}
	var metadata struct {
		Meta responseMeta `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Header().Get("X-Request-ID"), metadata.Meta.RequestID; got != want {
		t.Fatalf("header request ID = %q, body request ID = %q", got, want)
	}
}

func TestReadinessChecksUpstreamAndAPIRootDoesNotFallThrough(t *testing.T) {
	upstreamCalls := 0
	upstream := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamCalls++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"code":0,"data":{"initialized":true}}`)
	})
	server, _ := testAPI(t, upstream)

	readyRequest := httptest.NewRequest(http.MethodGet, "/yinqiao/v1/ready", nil)
	readyResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(readyResponse, readyRequest)
	if readyResponse.Code != http.StatusOK || upstreamCalls != 1 {
		t.Fatalf("ready status/calls = %d/%d, body=%s", readyResponse.Code, upstreamCalls, readyResponse.Body.String())
	}

	rootRequest := httptest.NewRequest(http.MethodPost, "/yinqiao/v1", strings.NewReader(`{}`))
	rootResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(rootResponse, rootRequest)
	if rootResponse.Code != http.StatusNotFound || upstreamCalls != 1 {
		t.Fatalf("root status/calls = %d/%d, body=%s", rootResponse.Code, upstreamCalls, rootResponse.Body.String())
	}
}

func TestGenericUpstreamPassthroughSignsAndStreamsUnknownAPI(t *testing.T) {
	upstream := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.Path != "/music/api/v1/future/private-action" {
			t.Fatalf("unexpected upstream request %s %s", request.Method, request.URL.String())
		}
		if request.URL.Query().Get("mode") != "full" || request.Header.Get("authx") == "" {
			t.Fatalf("missing query or signature: %s %#v", request.URL.RawQuery, request.Header)
		}
		writer.Header().Set("Content-Type", "application/octet-stream")
		writer.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(writer, "future-response")
	})
	server, _ := testAPI(t, upstream)

	request := httptest.NewRequest(http.MethodPatch, "/yinqiao/v1/upstream/future/private-action?mode=full", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted || response.Body.String() != "future-response" {
		t.Fatalf("status/body = %d/%q", response.Code, response.Body.String())
	}
}

func TestGenericUpstreamPassthroughRejectsTraversal(t *testing.T) {
	server, _ := testAPI(t, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/yinqiao/v1/upstream/a%5Csecret", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRawTrackStreamForwardsRangeAndPathParameter(t *testing.T) {
	upstream := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/music/api/v1/track/stream" || request.URL.Query().Get("guid") != "track-1" {
			t.Fatalf("unexpected upstream URL %s", request.URL.String())
		}
		if got, want := request.Header.Get("Range"), "bytes=2-5"; got != want {
			t.Fatalf("Range = %q, want %q", got, want)
		}
		writer.Header().Set("Content-Type", "audio/flac")
		writer.Header().Set("Content-Range", "bytes 2-5/10")
		writer.Header().Set("Accept-Ranges", "bytes")
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(writer, "2345")
	})
	server, _ := testAPI(t, upstream)

	request := httptest.NewRequest(http.MethodGet, "/yinqiao/v1/media/tracks/track-1/stream", nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent || response.Body.String() != "2345" {
		t.Fatalf("status/body = %d/%q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("Content-Range not forwarded: %#v", response.Header())
	}
}

func TestCapabilitiesAndSameOriginCORS(t *testing.T) {
	server, _ := testAPI(t, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "http://nas.local/yinqiao/v1/capabilities", nil)
	request.Host = "nas.local"
	request.Header.Set("Origin", "http://nas.local")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://nas.local" {
		t.Fatalf("CORS origin = %q", got)
	}
	var body struct {
		Data struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Count != 85 {
		t.Fatalf("capability count = %d", body.Data.Count)
	}
}
