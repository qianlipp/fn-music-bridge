package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (s *Server) handleAccounts(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch request.Method {
	case http.MethodGet:
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": true, "accounts": s.auth.vault.usernames()})
	case http.MethodPost:
		s.handleEnrollment(writer, request)
	case http.MethodDelete:
		s.deleteAccount(writer, request)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "METHOD_NOT_ALLOWED", "error": "method not allowed"})
	}
}

func (s *Server) deleteAccount(writer http.ResponseWriter, request *http.Request) {
	username := strings.TrimSpace(request.URL.Query().Get("username"))
	if username == "" && request.Body != nil {
		var input struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(io.LimitReader(request.Body, 1<<20)).Decode(&input); err == nil {
			username = strings.TrimSpace(input.Username)
		}
	}
	if username == "" {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "INVALID_INPUT", "error": "username is required"})
		return
	}
	removed, err := s.auth.vault.delete(username)
	if err != nil {
		s.logger.Printf("delete credential user=%q error=%v", username, err)
		writer.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "CREDENTIAL_STORE_ERROR", "error": "account could not be removed from the encrypted credential store"})
		return
	}
	if !removed {
		writer.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "ACCOUNT_NOT_FOUND", "error": "account is not bound"})
		return
	}
	s.forgetProtocolSessions(username)
	_ = json.NewEncoder(writer).Encode(map[string]any{"ok": true, "username": normalizeUsername(username), "accounts": s.auth.vault.usernames()})
}

func (s *Server) forgetProtocolSessions(username string) {
	username = normalizeUsername(username)
	s.auth.forget(username)
	if err := s.protocolSessions.deleteJellyfinUser(username); err != nil {
		s.logger.Printf("delete persisted Jellyfin sessions user=%q error=%v", username, err)
	}

	s.ampacheMu.Lock()
	for token, session := range s.ampacheSessions {
		if normalizeUsername(session.Auth.Username) == username {
			delete(s.ampacheSessions, token)
		}
	}
	s.ampacheMu.Unlock()

	s.synologyMu.Lock()
	for token, session := range s.synologySessions {
		if normalizeUsername(session.Username) == username {
			delete(s.synologySessions, token)
		}
	}
	s.synologyMu.Unlock()

	s.jellyfinMu.Lock()
	for token, session := range s.jellyfinSessions {
		if normalizeUsername(session.Username) == username {
			delete(s.jellyfinSessions, token)
		}
	}
	s.jellyfinMu.Unlock()
}
