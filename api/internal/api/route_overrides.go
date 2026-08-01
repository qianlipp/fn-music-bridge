package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type routeOverride struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

func applyRouteOverrides(routes []Route, filename string) ([]Route, error) {
	if strings.TrimSpace(filename) == "" {
		return routes, nil
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read upstream route map: %w", err)
	}
	var overrides map[string]routeOverride
	if err := json.Unmarshal(content, &overrides); err != nil {
		return nil, fmt.Errorf("parse upstream route map: %w", err)
	}

	known := make(map[string]struct{}, len(routes))
	for index := range routes {
		known[routes[index].ID] = struct{}{}
		override, exists := overrides[routes[index].ID]
		if !exists {
			continue
		}
		if override.Method != "" {
			method := strings.ToUpper(override.Method)
			if method != http.MethodGet && method != http.MethodPost {
				return nil, fmt.Errorf("route %s has unsupported method %q", routes[index].ID, override.Method)
			}
			routes[index].UpstreamMethod = method
		}
		if override.Path != "" {
			if !strings.HasPrefix(override.Path, "/") {
				return nil, fmt.Errorf("route %s path must start with /", routes[index].ID)
			}
			routes[index].UpstreamPath = override.Path
		}
	}
	for id := range overrides {
		if _, exists := known[id]; !exists {
			return nil, fmt.Errorf("route map contains unknown route id %q", id)
		}
	}
	return routes, nil
}
