package api

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const fnosGatewayHeader = "X-Yinqiao-Fnos-Gateway"

type GatewayCandidate struct {
	Origin    string `json:"origin"`
	Scheme    string `json:"scheme"`
	Port      int    `json:"port"`
	Source    string `json:"source"`
	Available bool   `json:"available"`
	Selected  bool   `json:"selected"`
	LatencyMS int64  `json:"latencyMs,omitempty"`
	Status    string `json:"status"`
}

type GatewayDiagnostics struct {
	SelectedOrigin string             `json:"selectedOrigin"`
	Candidates     []GatewayCandidate `json:"candidates"`
	SocketPath     string             `json:"socketPath"`
	SocketReady    bool               `json:"socketReady"`
	SocketMode     string             `json:"socketMode,omitempty"`
	DetectedAt     string             `json:"detectedAt"`
}

type gatewayProbeTarget struct {
	origin  *url.URL
	source  string
	timeout time.Duration
}

func localGatewayTLSConfig() *tls.Config {
	// Certificate names rarely contain 127.0.0.1. Verification is skipped only
	// for loopback gateway probes; non-loopback configured origins use the
	// default verified transport.
	return &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true} //nolint:gosec
}

func (c *UpstreamClient) clientForGateway(gateway *url.URL) *http.Client {
	if gateway != nil {
		host := strings.ToLower(gateway.Hostname())
		if host == "127.0.0.1" || host == "localhost" || host == "::1" {
			return c.localGatewayClient
		}
	}
	return c.gatewayClient
}

func parseLocalGatewayOrigin(value string) *url.URL {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil
	}
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return nil
	}
	return &url.URL{Scheme: parsed.Scheme, Host: net.JoinHostPort("127.0.0.1", port)}
}

func (c *UpstreamClient) GatewayDiagnostics(ctx context.Context, manualOrigin string) GatewayDiagnostics {
	targets := c.gatewayProbeTargets(manualOrigin)
	results := make([]GatewayCandidate, len(targets))
	semaphore := make(chan struct{}, 12)
	var wait sync.WaitGroup
	for index, target := range targets {
		index, target := index, target
		wait.Add(1)
		go func() {
			defer wait.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			resolved, latency, err := c.probeGateway(ctx, target.origin, target.timeout)
			candidate := GatewayCandidate{
				Origin: target.origin.String(), Scheme: target.origin.Scheme,
				Port: gatewayPort(target.origin), Source: target.source,
				LatencyMS: latency, Status: "不可用",
			}
			if err == nil && resolved != nil {
				candidate.Origin = resolved.String()
				candidate.Scheme = resolved.Scheme
				candidate.Port = gatewayPort(resolved)
				candidate.Available = true
				candidate.Status = "可用"
			}
			results[index] = candidate
		}()
	}
	wait.Wait()

	selected := ""
	for index := range results {
		if results[index].Available && selected == "" {
			selected = results[index].Origin
			results[index].Selected = true
		}
	}
	if selected != "" {
		if parsed, err := url.Parse(selected); err == nil {
			c.gatewayMu.Lock()
			copy := *parsed
			c.selectedGateway = &copy
			c.gatewayMu.Unlock()
		}
	}

	// Listener-derived failures are implementation details and would make the
	// panel noisy. Manual and built-in candidates remain visible when unavailable.
	visible := results[:0]
	for _, candidate := range results {
		if candidate.Available || candidate.Source != "本机监听端口" {
			visible = append(visible, candidate)
		}
	}
	report := GatewayDiagnostics{
		SelectedOrigin: selected,
		Candidates:     visible,
		SocketPath:     c.socketPath,
		DetectedAt:     time.Now().Format(time.RFC3339),
	}
	if info, err := os.Stat(c.socketPath); err == nil && info.Mode()&os.ModeSocket != 0 {
		report.SocketReady = true
		report.SocketMode = info.Mode().Perm().String()
	}
	return report
}

func (c *UpstreamClient) gatewayProbeTargets(manualOrigin string) []gatewayProbeTarget {
	seen := map[string]bool{}
	result := make([]gatewayProbeTarget, 0)
	appendTarget := func(origin *url.URL, source string, timeout time.Duration) {
		if origin == nil {
			return
		}
		key := strings.ToLower(origin.String())
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, gatewayProbeTarget{origin: origin, source: source, timeout: timeout})
	}
	appendTarget(parseLocalGatewayOrigin(manualOrigin), "手动配置", 2*time.Second)
	c.gatewayMu.Lock()
	if c.selectedGateway != nil {
		copy := *c.selectedGateway
		appendTarget(&copy, "当前网关", 2*time.Second)
	}
	c.gatewayMu.Unlock()
	for _, origin := range c.gatewayOrigins {
		copy := *origin
		appendTarget(&copy, "fnOS 默认端口", 1200*time.Millisecond)
	}
	for _, port := range c.listeningPorts() {
		for _, scheme := range []string{"http", "https"} {
			origin, _ := url.Parse(fmt.Sprintf("%s://127.0.0.1:%d", scheme, port))
			appendTarget(origin, "本机监听端口", 650*time.Millisecond)
		}
	}
	return result
}

func gatewayPort(origin *url.URL) int {
	if origin == nil {
		return 0
	}
	if value, err := strconv.Atoi(origin.Port()); err == nil {
		return value
	}
	if origin.Scheme == "https" {
		return 443
	}
	return 80
}

func localListeningTCPPorts() []int {
	seen := map[int]bool{}
	for _, pathname := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(pathname)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 4 || fields[3] != "0A" {
				continue
			}
			parts := strings.Split(fields[1], ":")
			if len(parts) != 2 {
				continue
			}
			value, parseErr := strconv.ParseInt(parts[1], 16, 32)
			if parseErr == nil && value > 0 && value <= 65535 {
				seen[int(value)] = true
			}
		}
		_ = file.Close()
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}
