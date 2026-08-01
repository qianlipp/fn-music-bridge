package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const yinqiaoVersion = "1.0.6"

var pathVariablePattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9]*)\}`)

type requestIDContextKey struct{}

type Server struct {
	config   Config
	routes   []Route
	upstream *UpstreamClient
	handler  http.Handler
	logger   *slog.Logger
}

func NewServer(config Config, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	upstream, err := NewUpstreamClient(config)
	if err != nil {
		return nil, err
	}
	routes, err := applyRouteOverrides(StableRoutes(), config.UpstreamRouteMap)
	if err != nil {
		return nil, err
	}
	server := &Server{config: config, routes: routes, upstream: upstream, logger: logger}
	server.handler = server.buildHandler()
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) buildHandler() http.Handler {
	mux := http.NewServeMux()
	prefix := s.config.PublicPrefix

	mux.HandleFunc("GET "+prefix, s.handleInfo)
	mux.HandleFunc("GET "+prefix+"/health", s.handleHealth)
	mux.HandleFunc("GET "+prefix+"/ready", s.handleReady)
	mux.HandleFunc("GET "+prefix+"/capabilities", s.handleCapabilities)
	mux.HandleFunc(prefix+"/upstream/", s.handleUpstreamPassthrough)
	for _, route := range s.routes {
		route := route
		mux.HandleFunc(route.Method+" "+prefix+route.Path, s.handleRoute(route))
	}

	// Do not let unknown api paths fall through to the legacy application.
	notFound := func(writer http.ResponseWriter, request *http.Request) {
		requestID := requestID(request)
		writeAPIError(writer, http.StatusNotFound, requestID, "ROUTE_NOT_FOUND", "api route not found")
	}
	mux.HandleFunc(prefix, notFound)
	mux.HandleFunc(prefix+"/", notFound)

	if s.config.ProxyMusicRoot {
		proxy := httputil.NewSingleHostReverseProxy(s.upstream.Origin())
		proxy.Transport = s.upstream.Transport()
		proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			s.logger.Error("legacy proxy failed", "path", request.URL.Path, "error", err)
			http.Error(writer, "music upstream unavailable", http.StatusBadGateway)
		}
		mux.Handle("/music/", proxy)
	}

	return s.apiMiddleware(mux)
}

func (s *Server) handleUpstreamPassthrough(writer http.ResponseWriter, request *http.Request) {
	requestID := requestID(request)
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodPatch && request.Method != http.MethodDelete {
		writeAPIError(writer, http.StatusMethodNotAllowed, requestID, "METHOD_NOT_ALLOWED", "upstream passthrough supports GET, POST, PUT, PATCH and DELETE")
		return
	}
	upstreamPath := "/" + strings.TrimPrefix(request.URL.Path, s.config.PublicPrefix+"/upstream/")
	if upstreamPath == "/" || path.Clean(upstreamPath) != upstreamPath || strings.Contains(upstreamPath, "\\") {
		writeAPIError(writer, http.StatusBadRequest, requestID, "INVALID_UPSTREAM_PATH", "upstream path must be a normalized API path")
		return
	}
	body, err := readRequestBody(writer, request, s.config.MaxBodyBytes)
	if err != nil {
		status, code := http.StatusBadRequest, "INVALID_REQUEST_BODY"
		if err == errBodyTooLarge {
			status, code = http.StatusRequestEntityTooLarge, "REQUEST_BODY_TOO_LARGE"
		}
		writeAPIError(writer, status, requestID, code, err.Error())
		return
	}

	routeID := "compat.upstream"
	if upstreamPath == "/user/password-login" {
		routeID = "auth.login.password"
	}
	route := Route{ID: routeID, UpstreamMethod: request.Method, Mode: ModeRaw}
	response, err := s.upstream.Do(request.Context(), request, route, upstreamPath, cloneQuery(request.URL.Query()), body)
	if err != nil {
		s.logger.Error("upstream passthrough failed", "path", upstreamPath, "requestId", requestID, "error", err)
		writeAPIError(writer, http.StatusBadGateway, requestID, "UPSTREAM_UNAVAILABLE", "music upstream is unavailable")
		return
	}
	defer response.Body.Close()
	copyResponseHeaders(writer.Header(), response.Header, true)
	writer.Header().Set("X-Request-ID", requestID)
	writer.Header().Set("X-Yinqiao-Version", yinqiaoVersion)
	writer.WriteHeader(response.StatusCode)
	_, _ = io.Copy(writer, response.Body)
}

func (s *Server) handleInfo(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"name":            "yinqiao-api",
			"version":         yinqiaoVersion,
			"apiVersion":      "v1",
			"prefix":          s.config.PublicPrefix,
			"capabilityCount": len(s.routes),
		},
		"meta": responseMeta{RequestID: requestID(request), Adapter: "yinqiao-v1"},
	})
}

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":   true,
		"data": map[string]string{"status": "ok", "upstream": "configured"},
		"meta": responseMeta{RequestID: requestID(request), Adapter: "yinqiao-v1"},
	})
}

func (s *Server) handleReady(writer http.ResponseWriter, request *http.Request) {
	requestID := requestID(request)
	ctx, cancel := context.WithTimeout(request.Context(), min(5*time.Second, s.config.UpstreamTimeout))
	defer cancel()
	route := JSONRoute("readiness", "system", "上游就绪检查", http.MethodGet, "/ready", AccessPublic, "/initialization/state")
	response, err := s.upstream.Do(ctx, request, route, route.UpstreamPath, nil, nil)
	if err != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": errorDetail{Code: "UPSTREAM_NOT_READY", Message: "music upstream is unavailable"},
			"meta":  responseMeta{RequestID: requestID, Adapter: "yinqiao-v1"},
		})
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	status, _ := normalizeUpstreamJSON(response.StatusCode, body, requestID)
	if err != nil || status != http.StatusOK {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{
			"ok":    false,
			"error": errorDetail{Code: "UPSTREAM_NOT_READY", Message: "music upstream did not pass readiness check"},
			"meta":  responseMeta{RequestID: requestID, Adapter: "yinqiao-v1"},
		})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":   true,
		"data": map[string]string{"status": "ready", "upstream": "ready"},
		"meta": responseMeta{RequestID: requestID, Adapter: "yinqiao-v1"},
	})
}

func (s *Server) handleCapabilities(writer http.ResponseWriter, request *http.Request) {
	type publicRoute struct {
		ID           string       `json:"id"`
		Category     string       `json:"category"`
		Summary      string       `json:"summary"`
		Method       string       `json:"method"`
		Path         string       `json:"path"`
		Access       Access       `json:"access"`
		ResponseMode ResponseMode `json:"responseMode"`
	}
	result := make([]publicRoute, 0, len(s.routes))
	for _, route := range s.routes {
		result = append(result, publicRoute{
			ID: route.ID, Category: route.Category, Summary: route.Summary,
			Method: route.Method, Path: s.config.PublicPrefix + route.Path,
			Access: route.Access, ResponseMode: route.Mode,
		})
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok": true,
		"data": map[string]any{
			"count": len(result), "routes": result,
			"upstreamPassthrough": map[string]any{
				"path":    s.config.PublicPrefix + "/upstream/{officialApiPath}",
				"methods": []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
			},
		},
		"meta": responseMeta{RequestID: requestID(request), Adapter: "yinqiao-v1"},
	})
}

func (s *Server) handleRoute(route Route) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		requestID := requestID(request)
		query := cloneQuery(request.URL.Query())
		upstreamPath := substitutePath(route.UpstreamPath, request)
		for key, template := range route.QueryAdd {
			query.Set(key, substitutePath(template, request))
		}

		body, err := readRequestBody(writer, request, s.config.MaxBodyBytes)
		if err != nil {
			status := http.StatusBadRequest
			code := "INVALID_REQUEST_BODY"
			if err == errBodyTooLarge {
				status, code = http.StatusRequestEntityTooLarge, "REQUEST_BODY_TOO_LARGE"
			}
			writeAPIError(writer, status, requestID, code, err.Error())
			return
		}
		if route.Mode == ModeJSON {
			body, err = adaptRequestBody(route.Adapter, body)
			if err != nil {
				writeAPIError(writer, http.StatusBadRequest, requestID, "INVALID_JSON", err.Error())
				return
			}
		}

		ctx := request.Context()
		cancel := func() {}
		if route.Mode != ModeRaw {
			ctx, cancel = context.WithTimeout(ctx, s.config.UpstreamTimeout)
		}
		defer cancel()

		response, err := s.upstream.Do(ctx, request, route, upstreamPath, query, body)
		if err != nil {
			s.logger.Error("upstream request failed", "route", route.ID, "requestId", requestID, "error", err)
			writeAPIError(writer, http.StatusBadGateway, requestID, "UPSTREAM_UNAVAILABLE", "music upstream is unavailable")
			return
		}
		defer response.Body.Close()

		if route.Mode == ModeRaw {
			copyResponseHeaders(writer.Header(), response.Header, true)
			writer.Header().Set("X-Request-ID", requestID)
			writer.Header().Set("X-Yinqiao-Version", yinqiaoVersion)
			writer.WriteHeader(response.StatusCode)
			_, _ = io.Copy(writer, response.Body)
			return
		}

		responseBody, err := io.ReadAll(io.LimitReader(response.Body, s.config.MaxBodyBytes+1))
		if err != nil {
			writeAPIError(writer, http.StatusBadGateway, requestID, "UPSTREAM_READ_ERROR", "failed to read music upstream response")
			return
		}
		if int64(len(responseBody)) > s.config.MaxBodyBytes {
			writeAPIError(writer, http.StatusBadGateway, requestID, "UPSTREAM_RESPONSE_TOO_LARGE", "music upstream response is too large")
			return
		}
		copyResponseHeaders(writer.Header(), response.Header, false)
		status, normalized := normalizeUpstreamJSON(response.StatusCode, responseBody, requestID)
		writeJSON(writer, status, normalized)
	}
}

func (s *Server) apiMiddleware(next http.Handler) http.Handler {
	allowedOrigins := make(map[string]struct{}, len(s.config.AllowedOrigins))
	for _, origin := range s.config.AllowedOrigins {
		allowedOrigins[origin] = struct{}{}
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, s.config.PublicPrefix) {
			next.ServeHTTP(writer, request)
			return
		}
		requestID := newRequestID(request)
		request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, requestID))
		writer.Header().Set("X-Request-ID", requestID)
		writer.Header().Set("X-Yinqiao-Version", yinqiaoVersion)

		origin := request.Header.Get("Origin")
		if origin != "" {
			if !originAllowed(origin, request.Host, allowedOrigins) {
				writeAPIError(writer, http.StatusForbidden, requestID, "ORIGIN_NOT_ALLOWED", "origin is not allowed")
				return
			}
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Access-Control-Allow-Credentials", "true")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Yinqiao-Key, X-Request-ID, Range")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, ETag, X-Request-ID, X-Yinqiao-Version, X-Yinqiao-Upstream")
			writer.Header().Add("Vary", "Origin")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		if s.config.APIKey != "" && request.URL.Path != s.config.PublicPrefix+"/health" && request.Header.Get("X-Yinqiao-Key") != s.config.APIKey {
			writeAPIError(writer, http.StatusUnauthorized, requestID, "API_KEY_REQUIRED", "valid X-Yinqiao-Key is required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func readRequestBody(writer http.ResponseWriter, request *http.Request, limit int64) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	request.Body = http.MaxBytesReader(writer, request.Body, limit)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		var maxError *http.MaxBytesError
		if errors.As(err, &maxError) {
			return nil, errBodyTooLarge
		}
		return nil, err
	}
	return body, nil
}

func cloneQuery(source url.Values) url.Values {
	result := make(url.Values, len(source))
	for key, values := range source {
		result[key] = append([]string(nil), values...)
	}
	return result
}

func originAllowed(origin, requestHost string, allowed map[string]struct{}) bool {
	if _, ok := allowed["*"]; ok {
		return true
	}
	if _, ok := allowed[origin]; ok {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, requestHost)
}

func substitutePath(template string, request *http.Request) string {
	return pathVariablePattern.ReplaceAllStringFunc(template, func(match string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		return url.PathEscape(request.PathValue(name))
	})
}

func requestID(request *http.Request) string {
	if value, ok := request.Context().Value(requestIDContextKey{}).(string); ok && value != "" {
		return value
	}
	return newRequestID(request)
}

func newRequestID(request *http.Request) string {
	if value := strings.TrimSpace(request.Header.Get("X-Request-ID")); value != "" && len(value) <= 128 {
		return value
	}
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("api-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func writeAPIError(writer http.ResponseWriter, status int, requestID, code, message string) {
	writeJSON(writer, status, errorResponse{
		OK:    false,
		Error: errorDetail{Code: code, Message: message},
		Meta:  responseMeta{RequestID: requestID, Adapter: "yinqiao-v1"},
	})
}

func EncodeCapabilityDocument(routes []Route) ([]byte, error) {
	return json.MarshalIndent(routes, "", "  ")
}
