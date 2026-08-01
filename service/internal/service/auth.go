package service

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

type authSession struct {
	Username string
	Password string
	Token    string
	User     map[string]any
}

type authFailure struct {
	Code            int
	Message         string
	UpstreamStatus  int
	UpstreamCode    string
	UpstreamMessage string
}

func (e *authFailure) Error() string { return e.Message }

type authenticator struct {
	vault    *credentialVault
	api      *apiClient
	mu       sync.RWMutex
	sessions map[string]authSession
}

func (a *authenticator) authenticate(ctx context.Context, values url.Values) (authSession, error) {
	username := strings.TrimSpace(values.Get("u"))
	if username == "" {
		return authSession{}, &authFailure{Code: 10, Message: "Required parameter 'u' is missing"}
	}

	if encodedPassword := values.Get("p"); encodedPassword != "" {
		password, err := decodeSubsonicPassword(encodedPassword)
		if err != nil {
			return authSession{}, &authFailure{Code: 40, Message: "Wrong username or password"}
		}
		return a.loginAndRemember(ctx, username, password)
	}

	token, salt := strings.ToLower(values.Get("t")), values.Get("s")
	if token == "" || salt == "" {
		return authSession{}, &authFailure{Code: 10, Message: "Either 'p' or both 't' and 's' are required"}
	}
	password, ok := a.vault.get(username)
	if !ok {
		return authSession{}, &authFailure{Code: 41, Message: "Token login requires one-time fnmusic credential enrollment"}
	}
	digest := md5.Sum([]byte(password + salt))
	expected := hex.EncodeToString(digest[:])
	if len(token) != len(expected) || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return authSession{}, &authFailure{Code: 40, Message: "Wrong username or password"}
	}
	return a.cachedOrLogin(ctx, username, password)
}

func (a *authenticator) loginAndRemember(ctx context.Context, username, password string) (authSession, error) {
	session, err := a.cachedOrLogin(ctx, username, password)
	if err != nil {
		return authSession{}, err
	}
	if err := a.vault.put(username, password); err != nil {
		return authSession{}, fmt.Errorf("save protocol credential: %w", err)
	}
	return session, nil
}

func (a *authenticator) cachedOrLogin(ctx context.Context, username, password string) (authSession, error) {
	cacheKey := sessionKey(username, password)
	a.mu.RLock()
	cached, ok := a.sessions[cacheKey]
	a.mu.RUnlock()
	if ok {
		return cached, nil
	}
	result, err := a.api.login(ctx, username, password, deviceID(username, a.vault.key))
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403 || apiErr.Status == 422) {
			return authSession{}, &authFailure{
				Code:            40,
				Message:         "Wrong username or password",
				UpstreamStatus:  apiErr.Status,
				UpstreamCode:    apiErr.Code,
				UpstreamMessage: apiErr.Message,
			}
		}
		return authSession{}, err
	}
	session := authSession{Username: username, Password: password, Token: result.UserToken, User: result.User}
	a.mu.Lock()
	a.sessions[cacheKey] = session
	a.mu.Unlock()
	return session, nil
}

func (a *authenticator) refresh(ctx context.Context, session authSession) (authSession, error) {
	result, err := a.api.login(ctx, session.Username, session.Password, deviceID(session.Username, a.vault.key))
	if err != nil {
		return authSession{}, err
	}
	refreshed := authSession{Username: session.Username, Password: session.Password, Token: result.UserToken, User: result.User}
	a.mu.Lock()
	a.sessions[sessionKey(session.Username, session.Password)] = refreshed
	a.mu.Unlock()
	return refreshed, nil
}

func (a *authenticator) forget(username string) {
	username = normalizeUsername(username)
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, session := range a.sessions {
		if normalizeUsername(session.Username) == username {
			delete(a.sessions, key)
		}
	}
}

func decodeSubsonicPassword(value string) (string, error) {
	if !strings.HasPrefix(strings.ToLower(value), "enc:") {
		return value, nil
	}
	decoded, err := hex.DecodeString(value[4:])
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func sessionKey(username, password string) string {
	digest := sha256.Sum256([]byte(normalizeUsername(username) + "\x00" + password))
	return hex.EncodeToString(digest[:])
}

func deviceID(username string, installationKey []byte) string {
	digest := hmac.New(sha256.New, installationKey)
	_, _ = digest.Write([]byte("yinqiao-device-v1:" + normalizeUsername(username)))
	return hex.EncodeToString(digest.Sum(nil)[:16])
}
