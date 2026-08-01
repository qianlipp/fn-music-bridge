package api

import (
	"os"
	"strings"
	"testing"
)

func TestAPIReferenceMentionsEveryStableExternalPath(t *testing.T) {
	document, err := os.ReadFile("../../API.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(document)
	for _, route := range StableRoutes() {
		if !strings.Contains(text, "`"+route.Path+"`") {
			t.Errorf("API.md does not mention route %s %s (%s)", route.Method, route.Path, route.ID)
		}
	}
}
