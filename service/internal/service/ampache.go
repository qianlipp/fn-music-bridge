package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const ampacheAPIVersion = "6.0.0"

type ampacheSession struct {
	Auth      authSession
	ExpiresAt time.Time
}

func isAmpachePath(path string) bool {
	switch path {
	case "/server/xml.server.php", "/server/json.server.php", "/play/index.php", "/image.php":
		return true
	default:
		return false
	}
}

func (s *Server) handleAmpache(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		s.writeAmpacheError(writer, request, http.StatusMethodNotAllowed, 405, "Method not allowed")
		return
	}
	if err := parseAmpacheForm(request); err != nil {
		s.writeAmpacheError(writer, request, http.StatusBadRequest, 400, "Invalid request parameters")
		return
	}

	switch request.URL.Path {
	case "/play/index.php":
		s.handleAmpacheStream(writer, request)
		return
	case "/image.php":
		s.handleAmpacheImage(writer, request)
		return
	}

	action := strings.ToLower(strings.TrimSpace(request.Form.Get("action")))
	if action == "handshake" {
		s.handleAmpacheHandshake(writer, request)
		return
	}
	if action == "" {
		s.writeAmpacheError(writer, request, http.StatusBadRequest, 400, "Action is required")
		return
	}
	session, token, err := s.ampacheAuthenticate(request)
	if err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
		return
	}
	if action == "goodbye" {
		s.ampacheMu.Lock()
		delete(s.ampacheSessions, token)
		s.ampacheMu.Unlock()
		s.writeAmpache(writer, request, n("success", nil))
		return
	}
	if action == "ping" {
		s.writeAmpache(writer, request,
			valueNode("server", "yinqiao"),
			valueNode("version", serverVersion),
			valueNode("api", ampacheAPIVersion),
		)
		return
	}

	nodes, err := s.dispatchAmpache(request.Context(), request, session, token, action)
	if err != nil {
		s.logger.Printf("ampache action=%s user=%q error=%v", action, session.Username, err)
		s.writeAmpacheProtocolError(writer, request, err)
		return
	}
	s.writeAmpache(writer, request, nodes...)
}

func parseAmpacheForm(request *http.Request) error {
	if err := request.ParseForm(); err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(request.Header.Get("Content-Type")), "application/json") || request.Body == nil {
		return nil
	}
	var body map[string]any
	if err := json.NewDecoder(io.LimitReader(request.Body, 1<<20)).Decode(&body); err != nil {
		return err
	}
	for key, value := range body {
		request.Form.Set(key, fmt.Sprint(value))
	}
	return nil
}

func (s *Server) handleAmpacheHandshake(writer http.ResponseWriter, request *http.Request) {
	username := strings.TrimSpace(request.Form.Get("user"))
	if username == "" {
		s.writeAmpacheError(writer, request, http.StatusUnauthorized, 401, "Username is required")
		return
	}
	var (
		session authSession
		err     error
	)
	if password := request.Form.Get("password"); password != "" {
		session, err = s.auth.loginAndRemember(request.Context(), username, password)
	} else {
		password, ok := s.auth.vault.get(username)
		if !ok {
			s.writeAmpacheError(writer, request, http.StatusUnauthorized, 401, "Credential enrollment is required before Ampache handshake")
			return
		}
		if !validAmpachePassphrase(password, request.Form.Get("timestamp"), request.Form.Get("auth")) {
			s.writeAmpacheError(writer, request, http.StatusUnauthorized, 401, "Invalid username or passphrase")
			return
		}
		session, err = s.auth.cachedOrLogin(request.Context(), username, password)
	}
	if err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
		return
	}
	token, err := randomAmpacheToken()
	if err != nil {
		s.writeAmpacheError(writer, request, http.StatusInternalServerError, 500, "Could not create session")
		return
	}
	expires := time.Now().Add(12 * time.Hour)
	s.ampacheMu.Lock()
	s.ampacheSessions[token] = ampacheSession{Auth: session, ExpiresAt: expires}
	s.pruneAmpacheSessionsLocked(time.Now())
	s.ampacheMu.Unlock()

	s.writeAmpache(writer, request,
		valueNode("auth", token),
		valueNode("api", ampacheAPIVersion),
		valueNode("session_expire", expires.UTC().Format(time.RFC3339)),
		valueNode("update", time.Now().UTC().Format(time.RFC3339)),
		valueNode("add", time.Now().UTC().Format(time.RFC3339)),
		valueNode("clean", time.Now().UTC().Format(time.RFC3339)),
		valueNode("songs", 0), valueNode("albums", 0), valueNode("artists", 0),
		valueNode("genres", 0), valueNode("playlists", 0),
	)
}

func validAmpachePassphrase(password, timestamp, provided string) bool {
	if timestamp == "" || provided == "" {
		return false
	}
	if parsed, err := strconv.ParseInt(timestamp, 10, 64); err != nil || time.Since(time.Unix(parsed, 0)) > 24*time.Hour || time.Until(time.Unix(parsed, 0)) > 5*time.Minute {
		return false
	}
	passwordDigest := sha256.Sum256([]byte(password))
	expectedDigest := sha256.Sum256([]byte(timestamp + hex.EncodeToString(passwordDigest[:])))
	expected := hex.EncodeToString(expectedDigest[:])
	provided = strings.ToLower(provided)
	return len(provided) == len(expected) && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func randomAmpacheToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func (s *Server) ampacheAuthenticate(request *http.Request) (authSession, string, error) {
	token := strings.TrimSpace(request.Form.Get("auth"))
	if token == "" {
		token = strings.TrimSpace(request.Form.Get("ssid"))
	}
	if authorization := strings.Fields(request.Header.Get("Authorization")); len(authorization) == 2 && strings.EqualFold(authorization[0], "Bearer") {
		token = authorization[1]
	}
	if token == "" {
		return authSession{}, "", &authFailure{Code: 40, Message: "Ampache session token is required"}
	}
	s.ampacheMu.RLock()
	entry, ok := s.ampacheSessions[token]
	s.ampacheMu.RUnlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return authSession{}, "", &authFailure{Code: 40, Message: "Ampache session has expired"}
	}
	return entry.Auth, token, nil
}

func (s *Server) pruneAmpacheSessionsLocked(now time.Time) {
	for token, session := range s.ampacheSessions {
		if now.After(session.ExpiresAt) {
			delete(s.ampacheSessions, token)
		}
	}
}

func (s *Server) dispatchAmpache(ctx context.Context, request *http.Request, session authSession, token, action string) ([]node, error) {
	filter := strings.TrimSpace(request.Form.Get("filter"))
	switch action {
	case "artists":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/artists/all", nil, nil)
		return ampacheArtistNodes(list(data), request, token), err
	case "artist":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/artists/detail", url.Values{"guid": {filter}}, nil)
		return []node{ampacheArtistNode(object(data), request, token)}, err
	case "artist_albums":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums/by-artist", ampachePageQuery(request.Form, map[string]string{"artistGUID": filter}), nil)
		return ampacheAlbumNodes(list(data), request, token), err
	case "albums":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums", ampachePageQuery(request.Form, nil), nil)
		return ampacheAlbumNodes(list(data), request, token), err
	case "album":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums/detail", url.Values{"guid": {filter}}, nil)
		return []node{ampacheAlbumNode(object(data), request, token)}, err
	case "album_songs":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-album", ampachePageQuery(request.Form, map[string]string{"albumGUID": filter}), nil)
		return s.ampacheTrackNodes(list(data), request, token), err
	case "songs":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks", ampachePageQuery(request.Form, nil), nil)
		return s.ampacheTrackNodes(list(data), request, token), err
	case "song":
		track, _, err := s.findTrack(ctx, session, filter)
		if err != nil {
			return nil, err
		}
		return []node{s.ampacheTrackNode(track, request, token)}, nil
	case "genres":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/genres", ampachePageQuery(request.Form, nil), nil)
		return ampacheGenreNodes(list(data)), err
	case "playlists":
		data, _, err := s.call(ctx, session, http.MethodGet, "/me/playlists", ampachePageQuery(request.Form, nil), nil)
		return ampachePlaylistNodes(list(data), session.Username), err
	case "playlist":
		data, _, err := s.call(ctx, session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {filter}}, nil)
		return []node{ampachePlaylistNode(object(data), session.Username)}, err
	case "playlist_songs":
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-playlist", ampachePageQuery(request.Form, map[string]string{"playlistGUID": filter}), nil)
		return s.ampacheTrackNodes(list(data), request, token), err
	case "search", "advanced_search":
		query := ampachePageQuery(request.Form, map[string]string{"keyword": filter})
		if filter == "" {
			query.Set("keyword", request.Form.Get("rule_1_input"))
		}
		data, _, err := s.call(ctx, session, http.MethodGet, "/search/tracks", query, nil)
		return s.ampacheTrackNodes(list(data), request, token), err
	case "stats":
		return []node{valueNode("songs", 0), valueNode("albums", 0), valueNode("artists", 0)}, nil
	default:
		return nil, &authFailure{Code: 70, Message: "Ampache action is not implemented: " + action}
	}
}

func ampachePageQuery(values url.Values, extra map[string]string) url.Values {
	limit := boundedInt(values.Get("limit"), 50, 1, 5000)
	offset := boundedInt(values.Get("offset"), 0, 0, 10_000_000)
	query := url.Values{"page": {strconv.Itoa(offset/limit + 1)}, "size": {strconv.Itoa(limit)}}
	for key, value := range extra {
		if value != "" {
			query.Set(key, value)
		}
	}
	return query
}

func ampacheArtistNodes(values []any, request *http.Request, token string) []node {
	result := make([]node, 0, len(values))
	for _, value := range values {
		result = append(result, ampacheArtistNode(object(value), request, token))
	}
	return result
}

func ampacheArtistNode(artist map[string]any, request *http.Request, token string) node {
	id := entityID(artist)
	return n("artist", map[string]any{"id": id},
		valueNode("name", stringValue(artist, "name", "title")),
		valueNode("albums", intValue(artist, "albumCount")),
		valueNode("songs", intValue(artist, "trackCount")),
		valueNode("art", ampacheArtURL(request, token, extractCoverID(artist))),
	)
}

func ampacheAlbumNodes(values []any, request *http.Request, token string) []node {
	result := make([]node, 0, len(values))
	for _, value := range values {
		result = append(result, ampacheAlbumNode(object(value), request, token))
	}
	return result
}

func ampacheAlbumNode(album map[string]any, request *http.Request, token string) node {
	artists := nestedList(album, "artists")
	artist := map[string]any{}
	if len(artists) > 0 {
		artist = object(artists[0])
	}
	return n("album", map[string]any{"id": entityID(album)},
		valueNode("name", stringValue(album, "name", "title")),
		valueNode("artist", stringValue(artist, "name", "title")),
		valueNode("artist_id", stringValue(artist, "guid", "id")),
		valueNode("year", intValue(album, "originalReleaseYear", "year")),
		valueNode("tracks", intValue(album, "trackCount", "totalTrack")),
		valueNode("disk", intValue(album, "discCount")),
		valueNode("art", ampacheArtURL(request, token, extractCoverID(album))),
	)
}

func (s *Server) ampacheTrackNodes(values []any, request *http.Request, token string) []node {
	result := make([]node, 0, len(values))
	for _, value := range values {
		track := object(value)
		s.rememberTrack(track)
		result = append(result, s.ampacheTrackNode(track, request, token))
	}
	return result
}

func (s *Server) ampacheTrackNode(track map[string]any, request *http.Request, token string) node {
	id := entityID(track)
	album := firstObject(track, "album")
	artists := nestedList(track, "artists")
	artist := map[string]any{}
	if len(artists) > 0 {
		artist = object(artists[0])
	}
	audio := firstObject(track, "audioSpec")
	extension := stringValue(audio, "extension", "format")
	mimeType := "audio/" + extension
	if extension == "mp3" {
		mimeType = "audio/mpeg"
	} else if extension == "" {
		mimeType = "application/octet-stream"
	}
	coverID := extractCoverID(track)
	if coverID == "" {
		coverID = extractCoverID(album)
	}
	return n("song", map[string]any{"id": id},
		valueNode("title", stringValue(track, "title", "name")),
		valueNode("name", stringValue(track, "title", "name")),
		valueNode("album", stringValue(album, "name", "title")),
		valueNode("album_id", stringValue(album, "guid", "id")),
		valueNode("artist", stringValue(artist, "name", "title")),
		valueNode("artist_id", stringValue(artist, "guid", "id")),
		valueNode("track", intValue(track, "trackNo")),
		valueNode("disk", intValue(track, "discNo")),
		valueNode("time", millisToSeconds(intValue(audio, "duration"))),
		valueNode("size", intValue(audio, "size")),
		valueNode("bitrate", intValue(audio, "bitrate")/1000),
		valueNode("rate", intValue(audio, "sampleRate")),
		valueNode("mime", mimeType),
		valueNode("url", ampacheStreamURL(request, token, id)),
		valueNode("art", ampacheArtURL(request, token, coverID)),
	)
}

func ampacheGenreNodes(values []any) []node {
	result := make([]node, 0, len(values))
	for _, value := range values {
		genre := object(value)
		result = append(result, n("genre", map[string]any{"id": entityID(genre)},
			valueNode("name", stringValue(genre, "name", "title")),
			valueNode("songs", intValue(genre, "trackCount")),
		))
	}
	return result
}

func ampachePlaylistNodes(values []any, username string) []node {
	result := make([]node, 0, len(values))
	for _, value := range values {
		result = append(result, ampachePlaylistNode(object(value), username))
	}
	return result
}

func ampachePlaylistNode(playlist map[string]any, username string) node {
	return n("playlist", map[string]any{"id": entityID(playlist)},
		valueNode("name", stringValue(playlist, "name", "title")),
		valueNode("owner", username),
		valueNode("items", intValue(playlist, "trackCount", "songCount")),
		valueNode("type", "private"),
	)
}

func valueNode(name string, value any) node {
	return node{Name: name, Value: value}
}

func ampacheBaseURL(request *http.Request) string {
	scheme := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(request.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = request.Host
	}
	return scheme + "://" + host
}

func ampacheStreamURL(request *http.Request, token, id string) string {
	query := url.Values{"ssid": {token}, "type": {"song"}, "oid": {id}, "player": {"api"}}
	return ampacheBaseURL(request) + "/play/index.php?" + query.Encode()
}

func ampacheArtURL(request *http.Request, token, coverID string) string {
	if coverID == "" {
		return ""
	}
	query := url.Values{"auth": {token}, "object_type": {"cover"}, "object_id": {coverID}}
	return ampacheBaseURL(request) + "/image.php?" + query.Encode()
}

func (s *Server) handleAmpacheStream(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.ampacheAuthenticate(request)
	if err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
		return
	}
	id := strings.TrimSpace(request.Form.Get("oid"))
	if id == "" {
		id = strings.TrimSpace(request.Form.Get("object_id"))
	}
	if id == "" {
		s.writeAmpacheError(writer, request, http.StatusBadRequest, 400, "Song id is required")
		return
	}
	if err := s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(id)+"/stream", nil); err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
	}
}

func (s *Server) handleAmpacheImage(writer http.ResponseWriter, request *http.Request) {
	session, _, err := s.ampacheAuthenticate(request)
	if err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
		return
	}
	coverID := strings.TrimSpace(request.Form.Get("object_id"))
	if coverID == "" {
		s.writeAmpacheError(writer, request, http.StatusBadRequest, 400, "Image id is required")
		return
	}
	if err := s.proxyBinary(writer, request, session, "/media/covers/"+url.PathEscape(coverID), nil); err != nil {
		s.writeAmpacheProtocolError(writer, request, err)
	}
}

func (s *Server) writeAmpacheProtocolError(writer http.ResponseWriter, request *http.Request, err error) {
	var authErr *authFailure
	if errors.As(err, &authErr) {
		code := authErr.Code
		if code == 40 {
			code = 401
		} else if code == 50 {
			code = 403
		} else if code == 70 {
			code = 404
		}
		s.writeAmpacheError(writer, request, http.StatusOK, code, authErr.Message)
		return
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		s.writeAmpacheError(writer, request, http.StatusOK, apiErr.Status, apiErr.Message)
		return
	}
	s.writeAmpacheError(writer, request, http.StatusOK, 500, "fnmusic service error")
}

func (s *Server) writeAmpacheError(writer http.ResponseWriter, request *http.Request, status, code int, message string) {
	s.writeAmpacheStatus(writer, request, status, []node{n("error", map[string]any{"code": code}, valueNode("message", message))})
}

func (s *Server) writeAmpache(writer http.ResponseWriter, request *http.Request, values ...node) {
	s.writeAmpacheStatus(writer, request, http.StatusOK, values)
}

func (s *Server) writeAmpacheStatus(writer http.ResponseWriter, request *http.Request, status int, values []node) {
	if strings.Contains(request.URL.Path, "json.server.php") || strings.EqualFold(request.Form.Get("format"), "json") {
		payload := map[string]any{}
		grouped := map[string][]any{}
		for _, value := range values {
			grouped[value.Name] = append(grouped[value.Name], ampacheNodeJSON(value))
		}
		for name, entries := range grouped {
			if len(entries) == 1 && name != "artist" && name != "album" && name != "song" && name != "genre" && name != "playlist" {
				payload[name] = entries[0]
			} else {
				payload[name] = entries
			}
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(payload)
		return
	}

	writer.Header().Set("Content-Type", "text/xml; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(xml.Header))
	encoder := xml.NewEncoder(writer)
	root := xml.StartElement{Name: xml.Name{Local: "root"}}
	_ = encoder.EncodeToken(root)
	for _, value := range values {
		_ = encodeNodeXML(encoder, value)
	}
	_ = encoder.EncodeToken(root.End())
	_ = encoder.Flush()
}

func ampacheNodeJSON(value node) any {
	if len(value.Children) == 0 && len(value.Attr) == 0 {
		return value.Value
	}
	result := map[string]any{}
	for key, item := range value.Attr {
		result[key] = item
	}
	if value.Value != nil {
		result["value"] = value.Value
	}
	for _, child := range value.Children {
		result[child.Name] = ampacheNodeJSON(child)
	}
	return result
}
