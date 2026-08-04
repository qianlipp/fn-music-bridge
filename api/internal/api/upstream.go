package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type UpstreamClient struct {
	origin             *url.URL
	apiBase            string
	signer             Signer
	client             *http.Client
	gatewayClient      *http.Client
	localGatewayClient *http.Client
	gatewayOrigins     []*url.URL
	gatewayMu          sync.Mutex
	selectedGateway    *url.URL
	listeningPorts     func() []int
	socketPath         string
}

func NewUpstreamClient(config Config) (*UpstreamClient, error) {
	origin, err := url.Parse(config.UpstreamOrigin)
	if err != nil || origin.Scheme == "" || origin.Host == "" {
		return nil, fmt.Errorf("invalid YINQIAO_UPSTREAM_ORIGIN: %q", config.UpstreamOrigin)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	if config.UpstreamUnixSocket != "" {
		socket := config.UpstreamUnixSocket
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}
	}

	gateways := make([]*url.URL, 0, len(config.UpstreamGateways))
	for _, value := range config.UpstreamGateways {
		gateway, parseErr := url.Parse(strings.TrimRight(value, "/"))
		if parseErr != nil || gateway.Scheme == "" || gateway.Host == "" {
			return nil, fmt.Errorf("invalid YINQIAO_UPSTREAM_GATEWAYS entry: %q", value)
		}
		gateways = append(gateways, gateway)
	}

	gatewayTransport := http.DefaultTransport.(*http.Transport).Clone()
	gatewayTransport.MaxIdleConns = 10
	gatewayTransport.MaxIdleConnsPerHost = 2
	localGatewayTransport := gatewayTransport.Clone()
	localGatewayTransport.TLSClientConfig = localGatewayTLSConfig()

	return &UpstreamClient{
		origin:  origin,
		apiBase: config.UpstreamAPIBase,
		signer:  NewSigner(config.SigningPrefix, config.ClientKey),
		client:  &http.Client{Transport: transport},
		gatewayClient: &http.Client{
			Transport: gatewayTransport,
		},
		localGatewayClient: &http.Client{
			Transport: localGatewayTransport,
		},
		gatewayOrigins: gateways,
		listeningPorts: localListeningTCPPorts,
		socketPath:     config.UpstreamUnixSocket,
	}, nil
}

func (c *UpstreamClient) Transport() http.RoundTripper {
	return c.client.Transport
}

func (c *UpstreamClient) Origin() *url.URL {
	copy := *c.origin
	return &copy
}

func (c *UpstreamClient) Do(ctx context.Context, incoming *http.Request, route Route, upstreamPath string, query url.Values, body []byte) (*http.Response, error) {
	if route.ID == "auth.login.password" {
		if gateway := c.resolveGateway(ctx, incoming.Header.Get(fnosGatewayHeader)); gateway != nil {
			return c.do(ctx, incoming, route, upstreamPath, query, body, gateway, c.clientForGateway(gateway), false)
		}
	}
	return c.do(ctx, incoming, route, upstreamPath, query, body, c.origin, c.client, true)
}

func (c *UpstreamClient) do(ctx context.Context, incoming *http.Request, route Route, upstreamPath string, query url.Values, body []byte, origin *url.URL, client *http.Client, identifyAPI bool) (*http.Response, error) {
	pathname := c.apiBase + upstreamPath
	target := *origin
	target.Path = strings.TrimRight(origin.Path, "/") + pathname
	target.RawQuery = canonicalQuery(query)

	request, err := http.NewRequestWithContext(ctx, route.UpstreamMethod, target.String(), bytesReader(body))
	if err != nil {
		return nil, err
	}
	copyRequestHeaders(request.Header, incoming.Header)
	applyBearerToken(request.Header, incoming.Header.Get("Authorization"))

	contentType := request.Header.Get("Content-Type")
	multipart := strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data")
	request.Header.Del("authx")
	request.Header.Del("X-Yinqiao-Key")
	request.Header.Del("Authorization")
	request.Header.Set("authx", c.signer.Header(route.UpstreamMethod, pathname, query, body, multipart))
	if identifyAPI {
		request.Header.Set("X-Music-API", "v1")
	} else {
		request.Header.Del("X-Music-API")
	}
	if request.Header.Get("Accept") == "" {
		request.Header.Set("Accept", "application/json")
	}
	if request.Header.Get("Accept-Language") == "" {
		request.Header.Set("Accept-Language", "zh-CN")
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	if identifyAPI {
		response.Header.Set("X-Yinqiao-Upstream", "unix-socket")
	} else {
		response.Header.Set("X-Yinqiao-Upstream", "fnos-web-gateway")
	}
	return response, nil
}

func (c *UpstreamClient) resolveGateway(ctx context.Context, manualOrigin string) *url.URL {
	c.gatewayMu.Lock()
	defer c.gatewayMu.Unlock()
	if strings.TrimSpace(manualOrigin) != "" {
		manual := parseLocalGatewayOrigin(manualOrigin)
		if manual == nil {
			return nil
		}
		if resolved, _, _ := c.probeGateway(ctx, manual, 2*time.Second); resolved != nil {
			c.selectedGateway = resolved
			return c.selectedGateway
		}
		return nil
	}
	if c.selectedGateway != nil {
		return c.selectedGateway
	}

	for _, gateway := range c.gatewayOrigins {
		if resolved, _, _ := c.probeGateway(ctx, gateway, 2*time.Second); resolved != nil {
			c.selectedGateway = resolved
			return c.selectedGateway
		}
	}
	if discovered := c.discoverListeningGateway(ctx); discovered != nil {
		c.selectedGateway = discovered
		return c.selectedGateway
	}
	return nil
}

func (c *UpstreamClient) discoverListeningGateway(ctx context.Context) *url.URL {
	ports := c.listeningPorts()
	if len(ports) == 0 {
		return nil
	}
	discoveryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct{ gateway *url.URL }
	results := make(chan result, len(ports)*2)
	for _, port := range ports {
		for _, scheme := range []string{"http", "https"} {
			candidate, _ := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", scheme, port))
			go func() {
				resolved, _, _ := c.probeGateway(discoveryCtx, candidate, 750*time.Millisecond)
				results <- result{gateway: resolved}
			}()
		}
	}
	for range len(ports) * 2 {
		if resolved := (<-results).gateway; resolved != nil {
			return resolved
		}
	}
	return nil
}

func (c *UpstreamClient) probeGateway(ctx context.Context, gateway *url.URL, timeout time.Duration) (*url.URL, int64, error) {
	const upstreamPath = "/initialization/state"
	startedAt := time.Now()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	pathname := c.apiBase + upstreamPath
	target := *gateway
	target.Path = strings.TrimRight(gateway.Path, "/") + pathname

	request, err := http.NewRequestWithContext(probeCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "zh-CN")
	request.Header.Set("User-Agent", "yinqiao-api/2.0")
	request.Header.Set("authx", c.signer.Header(http.MethodGet, pathname, nil, nil, false))

	response, err := c.clientForGateway(gateway).Do(request)
	if err != nil {
		return nil, time.Since(startedAt).Milliseconds(), err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, time.Since(startedAt).Milliseconds(), fmt.Errorf("HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, time.Since(startedAt).Milliseconds(), err
	}
	var envelope struct {
		Code *int `json:"code"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Code == nil || *envelope.Code != 0 {
		return nil, time.Since(startedAt).Milliseconds(), errors.New("not an fnOS music gateway")
	}
	resolved := *gateway
	if response.Request != nil && response.Request.URL != nil {
		resolved.Scheme = response.Request.URL.Scheme
		resolved.Host = response.Request.URL.Host
	}
	resolved.Path, resolved.RawQuery, resolved.Fragment = "", "", ""
	return &resolved, time.Since(startedAt).Milliseconds(), nil
}

func bytesReader(body []byte) io.Reader {
	if len(body) == 0 {
		return nil
	}
	return bytes.NewReader(body)
}

func copyRequestHeaders(target, source http.Header) {
	allowed := []string{
		"Accept", "Accept-Language", "Content-Type", "Cookie", "Range",
		"If-None-Match", "If-Modified-Since", "User-Agent",
	}
	for _, name := range allowed {
		for _, value := range source.Values(name) {
			target.Add(name, value)
		}
	}
}

func applyBearerToken(header http.Header, authorization string) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return
	}
	var cookies []string
	for _, cookieLine := range header.Values("Cookie") {
		for _, cookie := range strings.Split(cookieLine, ";") {
			cookie = strings.TrimSpace(cookie)
			if cookie == "" || strings.HasPrefix(cookie, "music-token=") {
				continue
			}
			cookies = append(cookies, cookie)
		}
	}
	cookies = append(cookies, "music-token="+url.QueryEscape(parts[1]))
	header.Set("Cookie", strings.Join(cookies, "; "))
}

func copyResponseHeaders(target, source http.Header, raw bool) {
	allowed := []string{"Set-Cookie", "Cache-Control", "ETag", "Last-Modified", "X-Yinqiao-Upstream"}
	if raw {
		allowed = append(allowed,
			"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges",
			"Content-Disposition", "Content-Encoding", "Expires", "Location", "Vary",
		)
	}
	for _, name := range allowed {
		for _, value := range source.Values(name) {
			target.Add(name, value)
		}
	}
}

var errBodyTooLarge = errors.New("request body is too large")
