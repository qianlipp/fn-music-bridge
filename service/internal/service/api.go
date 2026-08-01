package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type apiClient struct {
	baseURL      string
	client       *http.Client
	streamClient *http.Client
}

type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("api %s (HTTP %d): %s", e.Code, e.Status, e.Message)
}

type apiEnvelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type loginResult struct {
	UserToken string         `json:"userToken"`
	User      map[string]any `json:"user"`
}

func (c *apiClient) call(ctx context.Context, method, path, token string, query url.Values, body any) (any, error) {
	target := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("invalid API response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.OK {
		message := envelope.Error.Message
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		return nil, &apiError{Status: response.StatusCode, Code: envelope.Error.Code, Message: message}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return map[string]any{}, nil
	}
	var data any
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *apiClient) login(ctx context.Context, username, password, deviceID string) (loginResult, error) {
	// The official fnmusic frontend sends SHA-256 rather than the plaintext
	// password. Sending the same representation also avoids ambiguity when a
	// real plaintext password happens to be 64 hexadecimal characters long.
	digest := sha256.Sum256([]byte(password))
	data, err := c.call(ctx, http.MethodPost, "/auth/login/password", "", nil, map[string]any{
		"username": username,
		"password": hex.EncodeToString(digest[:]),
		"deviceId": deviceID,
	})
	if err != nil {
		return loginResult{}, err
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return loginResult{}, err
	}
	var result loginResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		return loginResult{}, err
	}
	if strings.TrimSpace(result.UserToken) == "" {
		return loginResult{}, errors.New("fnmusic login did not return userToken")
	}
	return result, nil
}

func (c *apiClient) raw(ctx context.Context, path, token string, query url.Values, incoming http.Header) (*http.Response, error) {
	target := c.baseURL + path
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	for _, header := range []string{"Range", "If-None-Match", "If-Modified-Since", "User-Agent"} {
		if value := incoming.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	client := c.streamClient
	if client == nil {
		client = c.client
	}
	return client.Do(request)
}
