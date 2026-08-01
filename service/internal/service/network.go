package service

import (
	"encoding/json"
	"net"
	"net/http"
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
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": true, "address": preferred, "addresses": addresses, "port": port,
	})
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
