package service

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type lanAddressCandidate struct {
	interfaceName string
	ip            string
	score         int
}

func (s *Server) handleNetworkInfo(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "METHOD_NOT_ALLOWED", "error": "method not allowed"})
		return
	}

	settings := s.settings.snapshot()
	port := strconv.Itoa(settings.ProtocolPort)
	ips := privateIPv4Addresses()
	if settings.AdvertisedHost != "" {
		ips = []string{settings.AdvertisedHost}
	}
	addresses := make([]string, 0, len(ips))
	for _, ip := range ips {
		addresses = append(addresses, "http://"+net.JoinHostPort(ip, port))
	}
	preferred := ""
	if len(addresses) > 0 {
		preferred = addresses[0]
	}
	gateway, gatewayErr := s.api.call(request.Context(), http.MethodGet, "/gateway/diagnostics", "", nil, nil)
	gatewayData := object(gateway)
	if gatewayErr != nil {
		gatewayData = map[string]any{"status": "检测失败", "error": gatewayErr.Error(), "candidates": []any{}}
	}
	preferredHost := ""
	if len(ips) > 0 {
		preferredHost = ips[0]
	}
	fnosBase := fnosWebBaseURL(settings, preferredHost, stringValue(gatewayData, "selectedOrigin"))
	accountURL := ""
	if fnosBase != "" {
		accountURL = strings.TrimRight(fnosBase, "/") + "/music/settings?key=user-management"
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": true, "address": preferred, "addresses": addresses, "port": port,
		"management": map[string]any{"port": listenPort(s.config.ListenAddr), "listen": s.config.ListenAddr},
		"apiBridge":  map[string]any{"url": s.config.APIBaseURL, "port": urlPort(s.config.APIBaseURL)},
		"fnos": map[string]any{
			"gateway": gatewayData, "webBaseUrl": fnosBase, "accountManagementUrl": accountURL,
			"manualGatewayUrl": settings.FnosGatewayURL, "manualWebUrl": settings.FnosWebURL,
		},
	})
}

func fnosWebBaseURL(settings ServiceSettings, preferredHost, detectedGateway string) string {
	for _, candidate := range []string{settings.FnosWebURL, settings.FnosGatewayURL, detectedGateway} {
		parsed, err := url.Parse(strings.TrimSpace(candidate))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			continue
		}
		hostname := strings.ToLower(parsed.Hostname())
		if preferredHost != "" && (hostname == "127.0.0.1" || hostname == "localhost" || hostname == "::1") {
			port := parsed.Port()
			if port == "" {
				if parsed.Scheme == "https" {
					port = "443"
				} else {
					port = "80"
				}
			}
			parsed.Host = net.JoinHostPort(preferredHost, port)
		}
		parsed.Path, parsed.RawQuery, parsed.Fragment = "", "", ""
		return strings.TrimRight(parsed.String(), "/")
	}
	return ""
}

func urlPort(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Port()
}

func listenPort(address string) string {
	_, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err == nil && port != "" {
		return port
	}
	return "14040"
}

func privateIPv4Addresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	candidates := make([]lanAddressCandidate, 0, len(interfaces))
	seen := map[string]bool{}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 || ignoredInterface(networkInterface.Name) {
			continue
		}
		addresses, addressErr := networkInterface.Addrs()
		if addressErr != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, parseErr := net.ParseCIDR(address.String())
			if parseErr != nil || ip.To4() == nil || !ip.IsPrivate() {
				continue
			}
			value := ip.To4().String()
			if seen[value] {
				continue
			}
			seen[value] = true
			candidates = append(candidates, lanAddressCandidate{
				interfaceName: networkInterface.Name,
				ip:            value,
				score:         privateAddressScore(value),
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		if candidates[i].interfaceName != candidates[j].interfaceName {
			return candidates[i].interfaceName < candidates[j].interfaceName
		}
		return candidates[i].ip < candidates[j].ip
	})
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.ip)
	}
	return result
}

func ignoredInterface(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"docker", "veth", "virbr", "br-", "tailscale", "tun", "tap", "wg", "zt"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func privateAddressScore(ip string) int {
	switch {
	case strings.HasPrefix(ip, "192.168."):
		return 0
	case strings.HasPrefix(ip, "10."):
		return 1
	default:
		return 2
	}
}
