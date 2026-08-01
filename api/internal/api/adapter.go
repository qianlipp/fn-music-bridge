package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func adaptRequestBody(adapter Adapter, body []byte) ([]byte, error) {
	if adapter == AdapterNone || len(body) == 0 {
		return compactJSON(body)
	}

	var object map[string]any
	if err := json.Unmarshal(body, &object); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	delete(object, "passwordEncoding")

	switch adapter {
	case AdapterPassword:
		hashPasswordField(object, "password")
	case AdapterInitializationConfirm:
		if user, ok := object["user"].(map[string]any); ok {
			hashPasswordField(user, "password")
		}
	}
	return json.Marshal(object)
}

func compactJSON(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	return json.Marshal(value)
}

func hashPasswordField(object map[string]any, field string) {
	password, ok := object[field].(string)
	if !ok || password == "" || sha256Pattern.MatchString(password) {
		return
	}
	digest := sha256.Sum256([]byte(password))
	object[field] = hex.EncodeToString(digest[:])
}
