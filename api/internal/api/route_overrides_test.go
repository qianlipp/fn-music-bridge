package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyRouteOverrides(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "routes.json")
	if err := os.WriteFile(filename, []byte(`{"tracks.list":{"method":"POST","path":"/v2/tracks/query"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	routes, err := applyRouteOverrides(StableRoutes(), filename)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range routes {
		if route.ID == "tracks.list" {
			if route.UpstreamMethod != "POST" || route.UpstreamPath != "/v2/tracks/query" {
				t.Fatalf("override not applied: %#v", route)
			}
			return
		}
	}
	t.Fatal("tracks.list route not found")
}
