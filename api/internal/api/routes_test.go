package api

import "testing"

func TestStableRoutesCoverAllLegacyOperations(t *testing.T) {
	routes := StableRoutes()
	if got, want := len(routes), 85; got != want {
		t.Fatalf("stable route count = %d, want %d", got, want)
	}

	external := make(map[string]string, len(routes))
	upstream := make(map[string]string, len(routes))
	for _, route := range routes {
		externalKey := route.Method + " " + route.Path
		if prior, exists := external[externalKey]; exists {
			t.Fatalf("duplicate external route %s (%s and %s)", externalKey, prior, route.ID)
		}
		external[externalKey] = route.ID

		upstreamKey := route.UpstreamMethod + " " + route.UpstreamPath
		if prior, exists := upstream[upstreamKey]; exists {
			t.Fatalf("duplicate upstream operation %s (%s and %s)", upstreamKey, prior, route.ID)
		}
		upstream[upstreamKey] = route.ID
	}
	if got, want := len(upstream), 85; got != want {
		t.Fatalf("unique upstream operation count = %d, want %d", got, want)
	}
}
