package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func list(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	data := object(value)
	for _, key := range []string{"list", "items", "tracks", "albums", "artists", "genres", "playlists", "libraries"} {
		if result, ok := data[key].([]any); ok {
			return result
		}
	}
	return nil
}

func stringValue(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func intValue(data map[string]any, keys ...string) int64 {
	for _, key := range keys {
		switch value := data[key].(type) {
		case float64:
			return int64(value)
		case float32:
			return int64(value)
		case int:
			return int64(value)
		case int64:
			return value
		case jsonNumber:
			parsed, _ := strconv.ParseInt(string(value), 10, 64)
			return parsed
		case string:
			parsed, _ := strconv.ParseInt(value, 10, 64)
			return parsed
		}
	}
	return 0
}

type jsonNumber string

func boolValue(data map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch value := data[key].(type) {
		case bool:
			return value
		case float64:
			return value != 0
		case string:
			parsed, _ := strconv.ParseBool(value)
			return parsed
		}
	}
	return false
}

func firstObject(data map[string]any, key string) map[string]any {
	if direct := object(data[key]); direct != nil {
		return direct
	}
	if values, ok := data[key].([]any); ok && len(values) > 0 {
		return object(values[0])
	}
	return nil
}

func nestedList(data map[string]any, key string) []any {
	if values, ok := data[key].([]any); ok {
		return values
	}
	return nil
}

func millisToSeconds(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return int64(math.Round(float64(value) / 1000))
}

func unixToRFC3339(value int64) string {
	if value <= 0 {
		return ""
	}
	if value > 10_000_000_000 {
		value /= 1000
	}
	return time.Unix(value, 0).UTC().Format(time.RFC3339)
}

func joinedNames(values []any) string {
	var names []string
	for _, value := range values {
		if name := stringValue(object(value), "name", "title"); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, " / ")
}
