package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const jellyfinServerID = "6ab8d6c47b5349509db107eda8d25f38"

func isJellyfinPath(requestPath string) bool {
	requestPath = normalizeJellyfinPath(requestPath)
	return requestPath == "/" || isJellyfinAPIPath(requestPath)
}

func isJellyfinAPIPath(requestPath string) bool {
	lowerPath := strings.ToLower(requestPath)
	for _, prefix := range []string{"/system", "/users", "/userviews", "/useritems", "/userfavoriteitems", "/userplayeditems", "/playingitems", "/items", "/albums", "/songs", "/artists", "/genres", "/musicgenres", "/audio", "/playlists", "/sessions", "/library", "/libraries", "/scheduledtasks", "/branding", "/displaypreferences", "/quickconnect", "/search", "/auth", "/livestreams", "/playback", "/providers"} {
		if lowerPath == prefix || strings.HasPrefix(lowerPath, prefix+"/") {
			return true
		}
	}
	return false
}

func normalizeJellyfinPath(requestPath string) string {
	requestPath = strings.TrimSuffix(requestPath, "/")
	for {
		changed := false
		for _, prefix := range []string{"/audio", "/emby", "/jellyfin"} {
			lowerPath := strings.ToLower(requestPath)
			if lowerPath == prefix {
				return "/"
			}
			if !strings.HasPrefix(lowerPath, prefix+"/") {
				continue
			}
			nestedPath := requestPath[len(prefix):]
			if isJellyfinAPIPath(nestedPath) || hasJellyfinNamespace(nestedPath) {
				requestPath = nestedPath
				changed = true
				break
			}
		}
		if !changed {
			return requestPath
		}
	}
}

func hasJellyfinNamespace(requestPath string) bool {
	lowerPath := strings.ToLower(requestPath)
	for _, prefix := range []string{"/audio", "/emby", "/jellyfin"} {
		if lowerPath == prefix || strings.HasPrefix(lowerPath, prefix+"/") {
			return true
		}
	}
	return false
}

func (s *Server) handleJellyfin(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	observed := &jellyfinResponseWriter{ResponseWriter: writer}
	writer = observed
	defer func() {
		s.logger.Printf("jellyfin response method=%s path=%s status=%d bytes=%d duration_ms=%d", request.Method, request.URL.Path, observed.statusCode(), observed.bytes, time.Since(startedAt).Milliseconds())
	}()
	s.logger.Printf("jellyfin request method=%s path=%s query=%q client=%q", request.Method, request.URL.Path, redactedJellyfinQuery(request.URL.Query()), jellyfinClientValue(request, "Client"))
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err := parseProtocolForm(request); err != nil {
		s.writeJellyfinError(writer, http.StatusBadRequest, "Invalid request body")
		return
	}
	requestPath := normalizeJellyfinPath(request.URL.Path)
	lowerPath := strings.ToLower(requestPath)
	if requestPath == "/" {
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinPublicSystemInfo())
		return
	}
	if lowerPath == "/system/ping" {
		if request.Method != http.MethodGet && request.Method != http.MethodPost {
			s.writeJellyfinError(writer, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("Jellyfin Server"))
		return
	}
	if lowerPath == "/system/info/public" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinPublicSystemInfo())
		return
	}
	if lowerPath == "/system/endpoint" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"IsLocal": true, "IsInNetwork": true})
		return
	}
	if lowerPath == "/branding/configuration" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"LoginDisclaimer": "", "CustomCss": "", "SplashscreenEnabled": false})
		return
	}
	if lowerPath == "/quickconnect/enabled" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, false)
		return
	}
	if strings.EqualFold(requestPath, "/Users/AuthenticateByName") {
		s.handleJellyfinLogin(writer, request)
		return
	}
	if lowerPath == "/users/public" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, []any{})
		return
	}

	session, token, err := s.jellyfinAuthenticate(request)
	if err != nil {
		s.writeJellyfinError(writer, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := s.dispatchJellyfin(writer, request, requestPath, token, session); err != nil {
		s.logger.Printf("jellyfin method=%s path=%s user=%q error=%v", request.Method, requestPath, session.Username, err)
		s.writeJellyfinAPIError(writer, err)
	}
}

func (s *Server) handleJellyfinLogin(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		s.writeJellyfinError(writer, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	username := strings.TrimSpace(firstNonEmpty(request.Form.Get("Username"), request.Form.Get("username")))
	password := firstNonEmpty(request.Form.Get("Pw"), request.Form.Get("Password"), request.Form.Get("password"))
	if username == "" || password == "" {
		s.writeJellyfinError(writer, http.StatusBadRequest, "Username and password are required")
		return
	}
	session, err := s.auth.loginAndRemember(request.Context(), username, password)
	if err != nil {
		s.logger.Printf("jellyfin login user=%q error=%v", username, err)
		s.writeJellyfinError(writer, http.StatusUnauthorized, "Invalid username or password")
		return
	}
	token, err := randomAmpacheToken()
	if err != nil {
		s.writeJellyfinError(writer, http.StatusInternalServerError, "Unable to create session")
		return
	}
	if err := s.protocolSessions.putJellyfin(token, session.Username); err != nil {
		s.logger.Printf("persist Jellyfin session user=%q error=%v", session.Username, err)
		s.writeJellyfinError(writer, http.StatusInternalServerError, "Unable to persist session")
		return
	}
	s.jellyfinMu.Lock()
	s.jellyfinSessions[token] = session
	s.jellyfinMu.Unlock()
	user := jellyfinUser(session)
	s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{
		"User": user,
		"SessionInfo": map[string]any{
			"Id": token[:32], "UserId": user["Id"], "UserName": session.Username, "Client": jellyfinClientValue(request, "Client"),
			"DeviceName": jellyfinClientValue(request, "Device"), "DeviceId": jellyfinClientValue(request, "DeviceId"), "ApplicationVersion": jellyfinClientValue(request, "Version"),
			"IsActive": true, "SupportsMediaControl": false, "SupportsRemoteControl": false, "NowPlayingQueue": []any{}, "SupportedCommands": []any{},
		},
		"AccessToken": token, "ServerId": jellyfinServerID,
	})
}

func jellyfinClientValue(request *http.Request, key string) string {
	values := parseJellyfinAuthorization(request.Header.Get("Authorization"))
	if value := values[strings.ToLower(key)]; value != "" {
		return value
	}
	values = parseJellyfinAuthorization(request.Header.Get("X-Emby-Authorization"))
	return values[strings.ToLower(key)]
}

func parseJellyfinAuthorization(header string) map[string]string {
	result := map[string]string{}
	header = strings.TrimSpace(header)
	if index := strings.Index(header, " "); index >= 0 {
		header = header[index+1:]
	}
	for _, part := range strings.Split(header, ",") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) == 2 {
			result[strings.ToLower(strings.TrimSpace(pair[0]))] = strings.Trim(strings.TrimSpace(pair[1]), `"`)
		}
	}
	return result
}

func jellyfinToken(request *http.Request) string {
	for _, header := range []string{"X-Emby-Token", "X-MediaBrowser-Token"} {
		if value := strings.TrimSpace(request.Header.Get(header)); value != "" {
			return value
		}
	}
	for _, header := range []string{"Authorization", "X-Emby-Authorization"} {
		headerValue := strings.TrimSpace(request.Header.Get(header))
		if parts := strings.Fields(headerValue); len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
		if value := parseJellyfinAuthorization(headerValue)["token"]; value != "" {
			return value
		}
	}
	return firstNonEmpty(request.Form.Get("api_key"), request.Form.Get("ApiKey"), request.Form.Get("apiKey"), request.Form.Get("access_token"), request.Form.Get("token"))
}

func (s *Server) jellyfinAuthenticate(request *http.Request) (authSession, string, error) {
	token := jellyfinToken(request)
	if token == "" {
		return authSession{}, "", errors.New("Jellyfin session is missing")
	}
	s.jellyfinMu.RLock()
	session, ok := s.jellyfinSessions[token]
	s.jellyfinMu.RUnlock()
	if ok {
		return session, token, nil
	}
	username, ok := s.protocolSessions.jellyfinUsername(token)
	if !ok {
		return authSession{}, "", errors.New("Jellyfin session is invalid")
	}
	password, ok := s.auth.vault.get(username)
	if !ok {
		_ = s.protocolSessions.deleteJellyfin(token)
		return authSession{}, "", errors.New("Jellyfin session account is no longer bound")
	}
	session, err := s.auth.cachedOrLogin(request.Context(), username, password)
	if err != nil {
		return authSession{}, "", err
	}
	s.jellyfinMu.Lock()
	s.jellyfinSessions[token] = session
	s.jellyfinMu.Unlock()
	return session, token, nil
}

func (s *Server) dispatchJellyfin(writer http.ResponseWriter, request *http.Request, requestPath, token string, session authSession) error {
	userID := jellyfinUserID(session)
	lowerPath := strings.ToLower(requestPath)
	lowerUserID := strings.ToLower(userID)
	if lowerPath == "/system/info" && request.Method == http.MethodGet {
		info := jellyfinPublicSystemInfo()
		info["LocalAddress"] = requestHostURL(request)
		info["OperatingSystem"] = "Linux"
		info["HasPendingRestart"] = false
		s.writeJellyfinJSON(writer, http.StatusOK, info)
		return nil
	}
	if (lowerPath == "/users/me" || lowerPath == "/users/"+lowerUserID) && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUser(session))
		return nil
	}
	if lowerPath == "/users" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, []any{jellyfinUser(session)})
		return nil
	}
	if lowerPath == "/items/counts" && request.Method == http.MethodGet {
		return s.jellyfinItemCounts(writer, request, session)
	}
	if lowerPath == "/items/root" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinLibraryView())
		return nil
	}
	if lowerPath == "/items/suggestions" && request.Method == http.MethodGet {
		return s.jellyfinSuggestions(writer, request, session)
	}
	if (lowerPath == "/users/"+lowerUserID+"/views" || lowerPath == "/userviews") && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"Items": []any{jellyfinLibraryView()}, "TotalRecordCount": 1, "StartIndex": 0})
		return nil
	}
	if lowerPath == "/userviews/groupingoptions" && request.Method == http.MethodGet {
		s.writeJellyfinJSON(writer, http.StatusOK, []any{})
		return nil
	}
	if strings.HasPrefix(lowerPath, "/users/") && strings.Contains(lowerPath, "/favoriteitems/") {
		return s.jellyfinFavorite(writer, request, session, pathTail(requestPath))
	}
	if strings.HasPrefix(lowerPath, "/userfavoriteitems/") {
		return s.jellyfinFavorite(writer, request, session, pathTail(requestPath))
	}
	if strings.HasPrefix(lowerPath, "/useritems/") && strings.HasSuffix(lowerPath, "/userdata") {
		id := requestPath[len("/UserItems/") : len(requestPath)-len("/UserData")]
		return s.jellyfinUserItemData(writer, request, session, id)
	}
	if strings.HasPrefix(lowerPath, "/userplayeditems/") {
		return s.jellyfinPlayedItem(writer, request, session, pathTail(requestPath))
	}
	if strings.HasPrefix(lowerPath, "/playingitems/") {
		return s.jellyfinPlayingItem(writer, request, session, requestPath)
	}
	if strings.HasPrefix(lowerPath, "/users/") && strings.HasSuffix(lowerPath, "/items/latest") {
		return s.jellyfinLatest(writer, request, session)
	}
	if strings.HasPrefix(lowerPath, "/users/") && strings.Contains(lowerPath, "/playeditems/") {
		id := stripJellyfinID(pathTail(requestPath), "track")
		if request.Method != http.MethodPost {
			return &jellyfinUnsupportedError{Message: "PlayedItems method is not supported"}
		}
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/activity/events", nil, map[string]any{"events": []any{map[string]any{"eventType": "track_play", "occurredAt": time.Now().UnixMilli(), "payload": map[string]any{"trackGUID": id}}}})
		if err != nil {
			return err
		}
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(false))
		return nil
	}
	if strings.HasPrefix(lowerPath, "/users/") && strings.Contains(lowerPath, "/items/") {
		return s.jellyfinItemDetail(writer, request, session, pathTail(requestPath))
	}
	if ((strings.HasPrefix(lowerPath, "/users/") && strings.HasSuffix(lowerPath, "/items")) || lowerPath == "/items") && request.Method == http.MethodGet {
		return s.jellyfinItems(writer, request, session)
	}
	if (lowerPath == "/artists" || lowerPath == "/artists/albumartists") && request.Method == http.MethodGet {
		return s.jellyfinArtists(writer, request, session)
	}
	if strings.HasSuffix(lowerPath, "/instantmix") {
		return &jellyfinUnsupportedError{Message: "Jellyfin Instant Mix is not available from fnmusic"}
	}
	if strings.HasPrefix(lowerPath, "/artists/") && strings.HasSuffix(lowerPath, "/similar") {
		return &jellyfinUnsupportedError{Message: "Artist similarity is not available from fnmusic"}
	}
	if strings.HasPrefix(lowerPath, "/artists/") {
		return s.jellyfinArtistByName(writer, request, session, requestPath[len("/Artists/"):])
	}
	if (lowerPath == "/genres" || lowerPath == "/musicgenres") && request.Method == http.MethodGet {
		return s.jellyfinGenres(writer, request, session)
	}
	if strings.HasPrefix(lowerPath, "/genres/") {
		return s.jellyfinGenreByName(writer, request, session, requestPath[len("/Genres/"):])
	}
	if strings.HasPrefix(lowerPath, "/musicgenres/") {
		return s.jellyfinGenreByName(writer, request, session, requestPath[len("/MusicGenres/"):])
	}
	if lowerPath == "/search/hints" {
		return s.jellyfinSearchHints(writer, request, session)
	}
	if lowerPath == "/playlists" {
		switch request.Method {
		case http.MethodGet:
			return s.jellyfinPlaylists(writer, request, session)
		case http.MethodPost:
			return s.jellyfinCreatePlaylist(writer, request, session)
		}
	}
	if strings.HasPrefix(lowerPath, "/playlists/") && strings.HasSuffix(lowerPath, "/items") {
		parts := strings.Split(strings.Trim(requestPath, "/"), "/")
		if len(parts) >= 3 {
			return s.jellyfinPlaylistItems(writer, request, session, parts[1])
		}
	}
	if strings.HasPrefix(lowerPath, "/playlists/") && strings.Contains(lowerPath, "/items/") && strings.Contains(lowerPath, "/move/") {
		return &jellyfinUnsupportedError{Message: "fnmusic does not expose reliable playlist item reordering"}
	}
	if strings.HasPrefix(lowerPath, "/playlists/") && strings.Contains(lowerPath, "/users") {
		return &jellyfinUnsupportedError{Message: "fnmusic playlists do not expose Jellyfin collaborative user permissions"}
	}
	if strings.HasPrefix(lowerPath, "/playlists/") && strings.Count(strings.Trim(requestPath, "/"), "/") == 1 {
		id := requestPath[len("/Playlists/"):]
		if request.Method == http.MethodGet {
			return s.jellyfinItemDetail(writer, request, session, ensureJellyfinID("playlist", id))
		}
		if request.Method == http.MethodPost {
			return s.jellyfinUpdateItem(writer, request, session, ensureJellyfinID("playlist", id))
		}
	}
	if strings.HasPrefix(lowerPath, "/items/") && strings.HasSuffix(lowerPath, "/playbackinfo") {
		if request.Method != http.MethodGet && request.Method != http.MethodPost {
			return &jellyfinUnsupportedError{Message: "PlaybackInfo method is not supported"}
		}
		id := requestPath[len("/Items/") : len(requestPath)-len("/PlaybackInfo")]
		return s.jellyfinPlaybackInfo(writer, request, session, id)
	}
	if lowerPath == "/items/latest" {
		if request.Method != http.MethodGet {
			return &jellyfinUnsupportedError{Message: "Latest-items method is not supported"}
		}
		return s.jellyfinLatest(writer, request, session)
	}
	if strings.HasPrefix(lowerPath, "/items/") && (strings.Contains(lowerPath, "/images/") || strings.HasSuffix(lowerPath, "/primary")) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return &jellyfinUnsupportedError{Message: "Image mutation is not supported"}
		}
		return s.jellyfinImage(writer, request, session, jellyfinImageItemID(requestPath))
	}
	if strings.HasPrefix(lowerPath, "/items/") && strings.HasSuffix(lowerPath, "/download") {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return &jellyfinUnsupportedError{Message: "Download method is not supported"}
		}
		id := requestPath[len("/Items/") : len(requestPath)-len("/Download")]
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(stripJellyfinID(id, "track"))+"/stream", nil)
	}
	if strings.HasPrefix(lowerPath, "/items/") && strings.HasSuffix(lowerPath, "/file") {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return &jellyfinUnsupportedError{Message: "File method is not supported"}
		}
		id := requestPath[len("/Items/") : len(requestPath)-len("/File")]
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(stripJellyfinID(id, "track"))+"/stream", nil)
	}
	if strings.HasPrefix(lowerPath, "/items/") && strings.HasSuffix(lowerPath, "/similar") {
		return &jellyfinUnsupportedError{Message: "Item similarity is not available from fnmusic"}
	}
	for _, suffix := range []string{"/ancestors", "/criticreviews", "/thememedia", "/themesongs", "/themevideos"} {
		if strings.HasPrefix(lowerPath, "/items/") && strings.HasSuffix(lowerPath, suffix) {
			return &jellyfinUnsupportedError{Message: "Jellyfin extended item metadata is not available from fnmusic"}
		}
	}
	if strings.HasPrefix(lowerPath, "/items/") {
		id := requestPath[len("/Items/"):]
		if request.Method == http.MethodDelete {
			return s.jellyfinDeleteItem(writer, request, session, id)
		}
		if request.Method == http.MethodPost {
			return s.jellyfinUpdateItem(writer, request, session, id)
		}
		return s.jellyfinItemDetail(writer, request, session, id)
	}
	if (strings.HasPrefix(lowerPath, "/audio/") && strings.Contains(lowerPath, "/remotesearch/lyrics")) || strings.HasPrefix(lowerPath, "/providers/lyrics/") {
		return &jellyfinUnsupportedError{Message: "Remote Jellyfin lyric providers are not available"}
	}
	if strings.HasPrefix(lowerPath, "/audio/") && strings.HasSuffix(lowerPath, "/lyrics") {
		if request.Method != http.MethodGet {
			return &jellyfinUnsupportedError{Message: "Lyric upload and deletion are not supported"}
		}
		id := requestPath[len("/Audio/") : len(requestPath)-len("/Lyrics")]
		return s.jellyfinLyrics(writer, request, session, id)
	}
	if strings.HasPrefix(lowerPath, "/audio/") && strings.HasSuffix(lowerPath, "/universal") {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return &jellyfinUnsupportedError{Message: "Universal audio method is not supported"}
		}
		id := requestPath[len("/Audio/") : len(requestPath)-len("/universal")]
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(stripJellyfinID(id, "track"))+"/stream", nil)
	}
	if strings.HasPrefix(lowerPath, "/audio/") && strings.Contains(lowerPath, "/stream") {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return &jellyfinUnsupportedError{Message: "Audio stream method is not supported"}
		}
		id := strings.SplitN(requestPath[len("/Audio/"):], "/", 2)[0]
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(stripJellyfinID(id, "track"))+"/stream", nil)
	}
	if lowerPath == "/sessions/playing" || lowerPath == "/sessions/playing/progress" || lowerPath == "/sessions/playing/stopped" {
		return s.jellyfinPlaybackEvent(writer, request, session, requestPath)
	}
	if lowerPath == "/sessions/playing/ping" || lowerPath == "/sessions/viewing" {
		writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if lowerPath == "/sessions/capabilities" || lowerPath == "/sessions/capabilities/full" {
		if request.Method != http.MethodPost {
			return &jellyfinUnsupportedError{Message: "Session capabilities method is not supported"}
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if lowerPath == "/sessions/logout" {
		if request.Method != http.MethodPost {
			return &jellyfinUnsupportedError{Message: "Session logout method is not supported"}
		}
		if err := s.protocolSessions.deleteJellyfin(token); err != nil {
			return err
		}
		s.jellyfinMu.Lock()
		delete(s.jellyfinSessions, token)
		s.jellyfinMu.Unlock()
		writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if lowerPath == "/scheduledtasks" && request.Method == http.MethodGet {
		return s.jellyfinScheduledTasks(writer, request, session)
	}
	if strings.HasPrefix(lowerPath, "/scheduledtasks/") && request.Method == http.MethodGet {
		return s.jellyfinScheduledTask(writer, request, session, pathTail(requestPath))
	}
	if strings.HasPrefix(lowerPath, "/scheduledtasks/running/") {
		if request.Method != http.MethodPost {
			return &jellyfinUnsupportedError{Message: "Cancelling a running music scan is not supported"}
		}
		if !strings.EqualFold(pathTail(requestPath), "yinqiao-scan") {
			return &authFailure{Code: 70, Message: "Scheduled task not found"}
		}
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/admin/libraries/scan-all", nil, map[string]any{})
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if lowerPath == "/library/refresh" && request.Method == http.MethodPost {
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/admin/libraries/scan-all", nil, map[string]any{})
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if strings.HasPrefix(lowerPath, "/displaypreferences/") {
		s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"Id": pathTail(requestPath), "ViewType": "Music", "RememberIndexing": false, "IndexBy": "None", "SortBy": "SortName", "SortOrder": "Ascending", "CustomPrefs": map[string]any{}})
		return nil
	}
	return &jellyfinUnsupportedError{Message: "Jellyfin interface is not supported: " + requestPath}
}

func redactedJellyfinQuery(values url.Values) string {
	redacted := make(url.Values, len(values))
	for key, entries := range values {
		lowerKey := strings.ToLower(key)
		if strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "password") || strings.Contains(lowerKey, "api_key") || lowerKey == "pw" {
			redacted[key] = []string{"[redacted]"}
			continue
		}
		redacted[key] = append([]string(nil), entries...)
	}
	return redacted.Encode()
}

func (s *Server) jellyfinItemCounts(writer http.ResponseWriter, request *http.Request, session authSession) error {
	type countSource struct {
		key      string
		endpoint string
		query    url.Values
	}
	sources := []countSource{
		{key: "SongCount", endpoint: "/catalog/tracks", query: url.Values{"page": {"1"}, "size": {"1"}}},
		{key: "AlbumCount", endpoint: "/catalog/albums", query: url.Values{"page": {"1"}, "size": {"1"}}},
		{key: "ArtistCount", endpoint: "/catalog/artists", query: url.Values{"page": {"1"}, "size": {"1"}}},
		{key: "PlaylistCount", endpoint: "/me/playlists"},
	}
	counts := map[string]any{
		"MovieCount": 0, "SeriesCount": 0, "EpisodeCount": 0, "ProgramCount": 0,
		"TrailerCount": 0, "MusicVideoCount": 0, "BoxSetCount": 0, "BookCount": 0,
	}
	total := int64(0)
	updated := session
	for _, source := range sources {
		data, next, err := s.call(request.Context(), updated, http.MethodGet, source.endpoint, source.query, nil)
		if err != nil {
			return err
		}
		updated = next
		count := intValue(object(data), "total", "TotalRecordCount", "count")
		if count == 0 {
			count = int64(len(list(data)))
		}
		counts[source.key] = count
		total += count
	}
	counts["ItemCount"] = total
	s.writeJellyfinJSON(writer, http.StatusOK, counts)
	return nil
}

func (s *Server) jellyfinPlaylists(writer http.ResponseWriter, request *http.Request, session authSession) error {
	start, limit := jellyfinPageParams(request.Form)
	page, err := s.jellyfinPlaylistValues(request, session, start, limit)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(page.Items, page.Total, start))
	return nil
}

func (s *Server) jellyfinSuggestions(writer http.ResponseWriter, request *http.Request, session authSession) error {
	start, limit := jellyfinPageParams(request.Form)
	if jellyfinFormValue(request.Form, "StartIndex") == "" {
		start = 0
	}
	probe, err := s.jellyfinTrackValues(request, session, "", "", 0, 1)
	if err != nil {
		return err
	}
	if probe.Total == 0 {
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollection(nil, 0))
		return nil
	}
	if start == 0 && probe.Total > limit {
		start = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(probe.Total - limit + 1)
	}
	page, err := s.jellyfinTrackValues(request, session, "", "", start, limit)
	if err != nil {
		return err
	}
	randomShuffle(page.Items)
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(page.Items, page.Total, start))
	return nil
}

func (s *Server) jellyfinLatest(writer http.ResponseWriter, request *http.Request, session authSession) error {
	request.Form.Set("IncludeItemTypes", firstNonEmpty(request.Form.Get("IncludeItemTypes"), "MusicAlbum,Audio"))
	request.Form.Set("SortBy", "DateCreated")
	request.Form.Set("SortOrder", "Descending")
	collector := newCaptureResponseWriter()
	if err := s.jellyfinItems(collector, request, session); err != nil {
		return err
	}
	var payload map[string]any
	if json.Unmarshal(collector.body, &payload) != nil {
		return errors.New("cannot create latest-items response")
	}
	s.writeJellyfinJSON(writer, http.StatusOK, payload["Items"])
	return nil
}

func (s *Server) jellyfinSearchHints(writer http.ResponseWriter, request *http.Request, session authSession) error {
	query := strings.TrimSpace(firstNonEmpty(request.Form.Get("SearchTerm"), request.Form.Get("searchTerm")))
	limit := boundedInt(request.Form.Get("Limit"), 50, 1, 500)
	result := make([]any, 0)
	types := []string{"MusicArtist", "MusicAlbum", "Audio"}
	for _, itemType := range types {
		request.Form.Set("IncludeItemTypes", itemType)
		request.Form.Set("SearchTerm", query)
		collector := newCaptureResponseWriter()
		if err := s.jellyfinItems(collector, request, session); err != nil {
			return err
		}
		var payload map[string]any
		if json.Unmarshal(collector.body, &payload) != nil {
			continue
		}
		for _, value := range list(payload["Items"]) {
			item := object(value)
			result = append(result, map[string]any{"ItemId": item["Id"], "Id": item["Id"], "Name": item["Name"], "Type": item["Type"], "MatchedTerm": query, "PrimaryImageTag": object(item["ImageTags"])["Primary"]})
			if len(result) >= limit {
				break
			}
		}
		if len(result) >= limit {
			break
		}
	}
	s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"SearchHints": result, "TotalRecordCount": len(result)})
	return nil
}

type jellyfinUnsupportedError struct{ Message string }

func (e *jellyfinUnsupportedError) Error() string { return e.Message }

func jellyfinPublicSystemInfo() map[string]any {
	return map[string]any{"LocalAddress": "", "ServerName": "Yinqiao", "Version": "10.10.7", "ProductName": "Jellyfin Server", "OperatingSystem": "Linux", "Id": jellyfinServerID, "StartupWizardCompleted": true}
}

func jellyfinUser(session authSession) map[string]any {
	role := strings.ToLower(stringValue(session.User, "role"))
	admin := role == "admin" || role == "administrator"
	return map[string]any{
		"Name": session.Username, "ServerId": jellyfinServerID, "Id": jellyfinUserID(session), "HasPassword": true, "HasConfiguredPassword": true, "EnableAutoLogin": false,
		"Configuration": map[string]any{"PlayDefaultAudioTrack": true, "GroupedFolders": []any{}, "RememberAudioSelections": true, "EnableNextEpisodeAutoPlay": false},
		"Policy":        map[string]any{"IsAdministrator": admin, "IsHidden": false, "IsDisabled": false, "EnableMediaPlayback": true, "EnableAudioPlaybackTranscoding": false, "EnableVideoPlaybackTranscoding": false, "EnablePlaybackRemuxing": false, "EnableContentDeletion": false, "EnableContentDownloading": true, "EnableRemoteControlOfOtherUsers": false, "EnableSharedDeviceControl": false, "EnableAllDevices": true, "EnableAllFolders": true, "BlockedTags": []any{}, "EnabledFolders": []any{}},
	}
}

func jellyfinUserID(session authSession) string {
	if id := stringValue(session.User, "guid", "id"); id != "" {
		return jellyfinID("user", id)
	}
	return jellyfinID("user", normalizeUsername(session.Username))
}

func jellyfinLibraryView() map[string]any {
	return map[string]any{"Name": "飞牛音乐", "ServerId": jellyfinServerID, "Id": "yinqiao-library", "Type": "CollectionFolder", "CollectionType": "music", "IsFolder": true, "CanDelete": false, "ImageTags": map[string]any{}, "UserData": jellyfinUserData(false)}
}

func (s *Server) jellyfinItems(writer http.ResponseWriter, request *http.Request, session authSession) error {
	types := commaValues(jellyfinFormValue(request.Form, "IncludeItemTypes"))
	if len(types) == 0 {
		types = []string{"MusicAlbum", "Audio"}
	}
	searchTerm := strings.TrimSpace(jellyfinFormValue(request.Form, "SearchTerm"))
	parentID := jellyfinFormValue(request.Form, "ParentId")
	favoriteOnly := jellyfinHasFilter(request.Form, "IsFavorite") || strings.EqualFold(jellyfinFormValue(request.Form, "IsFavorite"), "true")
	randomOrder := jellyfinHasSort(request.Form, "Random")
	start, limit := jellyfinPageParams(request.Form)
	fetchStart, fetchLimit := start, limit
	if len(types) > 1 {
		fetchStart, fetchLimit = 0, min(start+limit, 10_000)
	}
	result := make([]any, 0, fetchLimit)
	total := 0
	for _, itemType := range types {
		var page jellyfinResultPage
		var err error
		switch strings.ToLower(itemType) {
		case "musicalbum":
			if favoriteOnly {
				page, err = s.jellyfinFavoriteAlbumValues(request, session, fetchStart, fetchLimit)
			} else {
				page, err = s.jellyfinAlbumValues(request, session, searchTerm, fetchStart, fetchLimit)
			}
		case "musicartist":
			if favoriteOnly {
				page, err = s.jellyfinFavoriteArtistValues(request, session, fetchStart, fetchLimit)
			} else {
				page, err = s.jellyfinArtistValues(request, session, searchTerm, fetchStart, fetchLimit)
			}
		case "musicgenre":
			page, err = s.jellyfinGenreValues(request, session, fetchStart, fetchLimit)
		case "playlist":
			page, err = s.jellyfinPlaylistValues(request, session, fetchStart, fetchLimit)
		case "audio":
			if randomOrder && searchTerm == "" && parentID == "" && !favoriteOnly {
				page, err = s.jellyfinRandomTrackValues(request, session, fetchLimit)
			} else {
				page, err = s.jellyfinTrackValues(request, session, parentID, searchTerm, fetchStart, fetchLimit)
			}
		}
		if err != nil {
			return err
		}
		result = append(result, page.Items...)
		total += page.Total
	}
	if randomOrder {
		randomShuffle(result)
	}
	if len(types) > 1 {
		from := min(start, len(result))
		result = result[from:min(from+limit, len(result))]
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(result, total, start))
	return nil
}

type jellyfinResultPage struct {
	Items []any
	Total int
}

const jellyfinUpstreamPageSize = 50

func (s *Server) jellyfinPagedMapped(request *http.Request, session authSession, endpoint string, baseQuery url.Values, start, limit int, mapper func(map[string]any) any) (jellyfinResultPage, error) {
	query := make(url.Values, len(baseQuery)+2)
	for key, values := range baseQuery {
		query[key] = append([]string(nil), values...)
	}
	firstPage := start/jellyfinUpstreamPageSize + 1
	firstOffset := start % jellyfinUpstreamPageSize
	result := make([]any, 0, limit)
	total := 0
	totalKnown := false
	current := session
	for page := firstPage; len(result) < limit; page++ {
		query.Set("page", strconv.Itoa(page))
		query.Set("size", strconv.Itoa(jellyfinUpstreamPageSize))
		data, updated, err := s.call(request.Context(), current, http.MethodGet, endpoint, query, nil)
		if err != nil {
			return jellyfinResultPage{}, err
		}
		current = updated
		values := list(data)
		if value, ok := jellyfinTotal(data); ok {
			total, totalKnown = value, true
		}
		if totalKnown && start >= total {
			return jellyfinResultPage{Items: []any{}, Total: total}, nil
		}

		// Some fnmusic search/list endpoints ignore page and size and return the
		// complete result. Slice those responses once instead of repeating them.
		if len(values) > jellyfinUpstreamPageSize || (totalKnown && page == 1 && total == len(values)) {
			from := min(start, len(values))
			to := min(from+limit, len(values))
			for _, value := range values[from:to] {
				result = append(result, mapper(object(value)))
			}
			if !totalKnown {
				total = len(values)
			}
			return jellyfinResultPage{Items: result, Total: total}, nil
		}

		from := 0
		if page == firstPage {
			from = min(firstOffset, len(values))
		}
		for _, value := range values[from:] {
			result = append(result, mapper(object(value)))
			if len(result) >= limit {
				break
			}
		}
		if len(values) == 0 || len(values) < jellyfinUpstreamPageSize || (totalKnown && page*jellyfinUpstreamPageSize >= total) {
			break
		}
	}
	if !totalKnown {
		total = start + len(result)
	}
	return jellyfinResultPage{Items: result, Total: total}, nil
}

func (s *Server) jellyfinSearchMapped(request *http.Request, session authSession, endpoint, searchTerm string, start, limit int, mapper func(map[string]any) any) (jellyfinResultPage, error) {
	data, _, err := s.searchItems(request.Context(), session, endpoint, searchTerm, start, limit)
	if err != nil {
		return jellyfinResultPage{}, err
	}
	values := list(data)
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, mapper(object(value)))
	}
	total, ok := jellyfinTotal(data)
	if !ok {
		total = start + len(result)
	}
	return jellyfinResultPage{Items: result, Total: total}, nil
}

func jellyfinTotal(data any) (int, bool) {
	value := object(data)
	for _, key := range []string{"total", "TotalRecordCount", "count"} {
		if _, ok := value[key]; ok {
			return int(intValue(value, key)), true
		}
	}
	return 0, false
}

func (s *Server) jellyfinAlbumValues(request *http.Request, session authSession, searchTerm string, start, limit int) (jellyfinResultPage, error) {
	if searchTerm != "" {
		return s.jellyfinSearchMapped(request, session, "/search/albums", searchTerm, start, limit, func(value map[string]any) any { return jellyfinAlbum(value) })
	}
	endpoint := "/catalog/albums"
	query := url.Values{}
	artistIDs := commaValues(jellyfinFormValue(request.Form, "ArtistIds"))
	if len(artistIDs) > 0 {
		endpoint = "/catalog/albums/by-artist"
		query.Set("artistGUID", stripJellyfinID(artistIDs[0], "artist"))
	}
	addJellyfinSort(query, request.Form, "album")
	return s.jellyfinPagedMapped(request, session, endpoint, query, start, limit, func(value map[string]any) any { return jellyfinAlbum(value) })
}

func (s *Server) jellyfinArtistValues(request *http.Request, session authSession, searchTerm string, start, limit int) (jellyfinResultPage, error) {
	if searchTerm != "" {
		return s.jellyfinSearchMapped(request, session, "/search/artists", searchTerm, start, limit, func(value map[string]any) any { return jellyfinArtist(value) })
	}
	query := url.Values{}
	addJellyfinSort(query, request.Form, "artist")
	return s.jellyfinPagedMapped(request, session, "/catalog/artists", query, start, limit, func(value map[string]any) any { return jellyfinArtist(value) })
}

func (s *Server) jellyfinGenreValues(request *http.Request, session authSession, start, limit int) (jellyfinResultPage, error) {
	return s.jellyfinPagedMapped(request, session, "/catalog/genres", nil, start, limit, func(value map[string]any) any { return jellyfinGenre(value) })
}

func (s *Server) jellyfinPlaylistValues(request *http.Request, session authSession, start, limit int) (jellyfinResultPage, error) {
	return s.jellyfinPagedMapped(request, session, "/me/playlists", nil, start, limit, func(value map[string]any) any { return jellyfinPlaylist(value) })
}

func (s *Server) jellyfinTrackValues(request *http.Request, session authSession, parentID, searchTerm string, start, limit int) (jellyfinResultPage, error) {
	if searchTerm != "" {
		return s.jellyfinSearchMapped(request, session, "/search/tracks", searchTerm, start, limit, func(value map[string]any) any {
			s.rememberTrack(value)
			return jellyfinTrack(value)
		})
	}
	endpoint := "/catalog/tracks"
	query := url.Values{}
	switch {
	case strings.HasPrefix(strings.ToLower(parentID), "fnm-album-"):
		endpoint = "/catalog/tracks/by-album"
		query.Set("albumGUID", stripJellyfinID(parentID, "album"))
	case strings.HasPrefix(strings.ToLower(parentID), "fnm-genre-"):
		endpoint = "/catalog/tracks/by-genre"
		query.Set("genreGUID", stripJellyfinID(parentID, "genre"))
	case strings.HasPrefix(strings.ToLower(parentID), "fnm-artist-"):
		endpoint = "/catalog/tracks/by-artist"
		query.Set("artistGUID", stripJellyfinID(parentID, "artist"))
	case strings.HasPrefix(strings.ToLower(parentID), "fnm-playlist-"):
		endpoint = "/catalog/tracks/by-playlist"
		query.Set("playlistGUID", stripJellyfinID(parentID, "playlist"))
	case jellyfinHasFilter(request.Form, "IsFavorite") || strings.EqualFold(jellyfinFormValue(request.Form, "IsFavorite"), "true"):
		endpoint = "/me/favorites"
	case jellyfinHasSort(request.Form, "DatePlayed") || jellyfinHasSort(request.Form, "PlayCount") || jellyfinHasFilter(request.Form, "IsPlayed"):
		endpoint = "/me/history"
	default:
		artistIDs := commaValues(jellyfinFormValue(request.Form, "ArtistIds"))
		if len(artistIDs) > 0 {
			endpoint = "/catalog/tracks/by-artist"
			query.Set("artistGUID", stripJellyfinID(artistIDs[0], "artist"))
		}
	}
	if endpoint == "/catalog/tracks" {
		addJellyfinSort(query, request.Form, "track")
	}
	return s.jellyfinPagedMapped(request, session, endpoint, query, start, limit, func(value map[string]any) any {
		track := jellyfinTrackRecord(value)
		s.rememberTrack(track)
		return jellyfinTrackFromRecord(track, value, endpoint)
	})
}

func (s *Server) jellyfinRandomTrackValues(request *http.Request, session authSession, limit int) (jellyfinResultPage, error) {
	probe, err := s.jellyfinPagedMapped(request, session, "/catalog/tracks", nil, 0, 1, func(value map[string]any) any { return value })
	if err != nil || probe.Total == 0 {
		return probe, err
	}
	start := 0
	if probe.Total > limit {
		start = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(probe.Total - limit + 1)
	}
	return s.jellyfinPagedMapped(request, session, "/catalog/tracks", nil, start, limit, func(value map[string]any) any {
		track := jellyfinTrackRecord(value)
		s.rememberTrack(track)
		return jellyfinTrack(track)
	})
}

func (s *Server) jellyfinFavoriteAlbumValues(request *http.Request, session authSession, start, limit int) (jellyfinResultPage, error) {
	return s.jellyfinFavoriteRelatedValues(request, session, "album", start, limit)
}

func (s *Server) jellyfinFavoriteArtistValues(request *http.Request, session authSession, start, limit int) (jellyfinResultPage, error) {
	return s.jellyfinFavoriteRelatedValues(request, session, "artist", start, limit)
}

func (s *Server) jellyfinFavoriteRelatedValues(request *http.Request, session authSession, kind string, start, limit int) (jellyfinResultPage, error) {
	page, err := s.jellyfinPagedMapped(request, session, "/me/favorites", nil, 0, 10_000, func(value map[string]any) any { return value })
	if err != nil {
		return jellyfinResultPage{}, err
	}
	seen := map[string]bool{}
	items := make([]any, 0)
	for _, raw := range page.Items {
		track := jellyfinTrackRecord(object(raw))
		if kind == "album" {
			album := firstObject(track, "album")
			id := entityID(album)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			items = append(items, jellyfinAlbum(album))
			continue
		}
		for _, rawArtist := range nestedList(track, "artists") {
			artist := object(rawArtist)
			id := entityID(artist)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			items = append(items, jellyfinArtist(artist))
		}
	}
	from := min(start, len(items))
	return jellyfinResultPage{Items: items[from:min(from+limit, len(items))], Total: len(items)}, nil
}

func (s *Server) jellyfinArtists(writer http.ResponseWriter, request *http.Request, session authSession) error {
	start, limit := jellyfinPageParams(request.Form)
	page, err := s.jellyfinArtistValues(request, session, jellyfinFormValue(request.Form, "SearchTerm"), start, limit)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(page.Items, page.Total, start))
	return nil
}

func (s *Server) jellyfinGenres(writer http.ResponseWriter, request *http.Request, session authSession) error {
	start, limit := jellyfinPageParams(request.Form)
	page, err := s.jellyfinGenreValues(request, session, start, limit)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(page.Items, page.Total, start))
	return nil
}

func (s *Server) jellyfinItemDetail(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	var data any
	var err error
	switch kind {
	case "track", "":
		var track map[string]any
		track, _, err = s.findTrack(request.Context(), session, raw)
		if err == nil {
			s.writeJellyfinJSON(writer, http.StatusOK, jellyfinTrack(track))
			return nil
		}
		if kind == "track" {
			return err
		}
	case "album":
		data, _, err = s.call(request.Context(), session, http.MethodGet, "/catalog/albums/detail", url.Values{"guid": {raw}}, nil)
	case "artist":
		data, _, err = s.call(request.Context(), session, http.MethodGet, "/catalog/artists/detail", url.Values{"guid": {raw}}, nil)
	case "playlist":
		data, _, err = s.call(request.Context(), session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {raw}}, nil)
	case "genre":
		data = map[string]any{"guid": raw, "name": raw}
	case "library":
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinLibraryView())
		return nil
	}
	if err != nil {
		return err
	}
	var item map[string]any
	switch kind {
	case "album":
		item = jellyfinAlbum(object(data))
	case "artist":
		item = jellyfinArtist(object(data))
	case "playlist":
		item = jellyfinPlaylist(object(data))
	case "genre":
		item = jellyfinGenre(object(data))
	}
	if item == nil {
		return &authFailure{Code: 70, Message: "Item not found"}
	}
	s.writeJellyfinJSON(writer, http.StatusOK, item)
	return nil
}

func (s *Server) jellyfinImage(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	coverID := ""
	var data any
	var err error
	switch kind {
	case "track", "":
		var track map[string]any
		track, session, err = s.findTrack(request.Context(), session, raw)
		if err == nil {
			coverID = extractCoverID(track)
			if coverID == "" {
				coverID = extractCoverID(firstObject(track, "album"))
			}
		}
	case "album":
		data, session, err = s.call(request.Context(), session, http.MethodGet, "/catalog/albums/detail", url.Values{"guid": {raw}}, nil)
		if err == nil {
			coverID = extractCoverID(object(data))
		}
	case "artist":
		data, session, err = s.call(request.Context(), session, http.MethodGet, "/catalog/artists/detail", url.Values{"guid": {raw}}, nil)
		if err == nil {
			coverID = extractCoverID(object(data))
		}
	case "playlist":
		data, session, err = s.call(request.Context(), session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {raw}}, nil)
		if err == nil {
			coverID = extractCoverID(object(data))
		}
	}
	if err != nil {
		return err
	}
	if coverID == "" {
		return &authFailure{Code: 70, Message: "Image not found"}
	}
	return s.proxyBinary(writer, request, session, "/media/covers/"+url.PathEscape(coverID), nil)
}

func (s *Server) jellyfinLyrics(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	text, _, err := s.lyricText(request.Context(), session, stripJellyfinID(id, "track"))
	if err != nil {
		return err
	}
	lines, synced := parseLyricLines(text)
	values := make([]any, 0, len(lines))
	for _, line := range lines {
		entry := map[string]any{"Text": fmt.Sprint(line.Value)}
		if synced {
			entry["Start"] = intValue(line.Attr, "start") * 10_000
		}
		values = append(values, entry)
	}
	s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"Metadata": map[string]any{}, "Lyrics": values})
	return nil
}

func (s *Server) jellyfinFavorite(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	if kind != "track" && kind != "" {
		return &jellyfinUnsupportedError{Message: "Only song favorites are supported"}
	}
	endpoint := "/me/favorites"
	if request.Method == http.MethodDelete {
		endpoint += "/remove"
	} else if request.Method != http.MethodPost {
		return &jellyfinUnsupportedError{Message: "Favorite method is not supported"}
	}
	_, _, err := s.call(request.Context(), session, http.MethodPost, endpoint, nil, map[string]any{"trackGUID": raw})
	if err != nil {
		return err
	}
	track, _, findErr := s.findTrack(request.Context(), session, raw)
	if findErr == nil {
		track["isFavorite"] = request.Method == http.MethodPost
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(request.Method == http.MethodPost))
		return nil
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) jellyfinUserItemData(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	if kind != "track" && kind != "" {
		if request.Method != http.MethodGet {
			return &jellyfinUnsupportedError{Message: "Only song user data can be updated"}
		}
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(false))
		return nil
	}
	if request.Method == http.MethodPost {
		favoriteValue := jellyfinFormValue(request.Form, "IsFavorite")
		playedValue := jellyfinFormValue(request.Form, "Played")
		position := intValue(map[string]any{"position": jellyfinFormValue(request.Form, "PlaybackPositionTicks")}, "position")
		if position > 0 || strings.EqualFold(playedValue, "false") {
			return &jellyfinUnsupportedError{Message: "Playback resume position and removing playback history are not supported"}
		}
		if favoriteValue == "" && !strings.EqualFold(playedValue, "true") {
			return &jellyfinUnsupportedError{Message: "No supported Jellyfin user-data field was supplied"}
		}
		if favoriteValue != "" {
			endpoint := "/me/favorites"
			if !strings.EqualFold(favoriteValue, "true") {
				endpoint += "/remove"
			}
			if _, _, err := s.call(request.Context(), session, http.MethodPost, endpoint, nil, map[string]any{"trackGUID": raw}); err != nil {
				return err
			}
		}
		if strings.EqualFold(playedValue, "true") {
			if _, _, err := s.call(request.Context(), session, http.MethodPost, "/activity/events", nil, map[string]any{"events": []any{map[string]any{"eventType": "track_play", "occurredAt": time.Now().UnixMilli(), "payload": map[string]any{"trackGUID": raw}}}}); err != nil {
				return err
			}
		}
		favorite := strings.EqualFold(favoriteValue, "true")
		if favoriteValue == "" {
			track, _, err := s.findTrack(request.Context(), session, raw)
			if err != nil {
				return err
			}
			favorite = boolValue(track, "isFavorite")
		}
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(favorite))
		return nil
	}
	if request.Method != http.MethodGet {
		return &jellyfinUnsupportedError{Message: "User-data method is not supported"}
	}
	track, _, err := s.findTrack(request.Context(), session, raw)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(boolValue(track, "isFavorite")))
	return nil
}

func (s *Server) jellyfinPlayedItem(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	if request.Method != http.MethodPost {
		return &jellyfinUnsupportedError{Message: "Removing fnmusic playback history is not supported"}
	}
	raw := stripJellyfinID(id, "track")
	_, _, err := s.call(request.Context(), session, http.MethodPost, "/activity/events", nil, map[string]any{"events": []any{map[string]any{"eventType": "track_play", "occurredAt": time.Now().UnixMilli(), "payload": map[string]any{"trackGUID": raw}}}})
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, jellyfinUserData(false))
	return nil
}

func (s *Server) jellyfinPlayingItem(writer http.ResponseWriter, request *http.Request, session authSession, requestPath string) error {
	value := requestPath[len("/PlayingItems/"):]
	if index := strings.Index(value, "/"); index >= 0 {
		value = value[:index]
	}
	if request.Method == http.MethodDelete {
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/activity/events", nil, map[string]any{"events": []any{map[string]any{"eventType": "track_play", "occurredAt": time.Now().UnixMilli(), "payload": map[string]any{"trackGUID": stripJellyfinID(value, "track")}}}})
		if err != nil {
			return err
		}
	} else if request.Method != http.MethodPost {
		return &jellyfinUnsupportedError{Message: "PlayingItems method is not supported"}
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) jellyfinArtistByName(writer http.ResponseWriter, request *http.Request, session authSession, encodedName string) error {
	name, _ := url.PathUnescape(encodedName)
	if kind, raw := splitJellyfinID(name); kind == "artist" {
		data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/artists/detail", url.Values{"guid": {raw}}, nil)
		if err != nil {
			return err
		}
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinArtist(object(data)))
		return nil
	}
	page, err := s.jellyfinArtistValues(request, session, name, 0, 500)
	if err != nil {
		return err
	}
	for _, value := range page.Items {
		item := object(value)
		if strings.EqualFold(stringValue(item, "Name"), name) {
			s.writeJellyfinJSON(writer, http.StatusOK, item)
			return nil
		}
	}
	return &authFailure{Code: 70, Message: "Artist not found"}
}

func (s *Server) jellyfinGenreByName(writer http.ResponseWriter, request *http.Request, session authSession, encodedName string) error {
	name, _ := url.PathUnescape(encodedName)
	page, err := s.jellyfinGenreValues(request, session, 0, 1000)
	if err != nil {
		return err
	}
	for _, value := range page.Items {
		item := object(value)
		if strings.EqualFold(stringValue(item, "Name"), name) || strings.EqualFold(stringValue(item, "Id"), name) {
			s.writeJellyfinJSON(writer, http.StatusOK, item)
			return nil
		}
	}
	return &authFailure{Code: 70, Message: "Genre not found"}
}

func (s *Server) jellyfinCreatePlaylist(writer http.ResponseWriter, request *http.Request, session authSession) error {
	ids := jellyfinRequestIDs(request.Form, "Ids")
	coverID, session := s.playlistCoverID(request.Context(), session, ids)
	data, session, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists", nil, map[string]any{"name": required(request.Form, "Name"), "coverId": nullableString(coverID)})
	if err != nil {
		return err
	}
	id := entityID(object(data))
	if len(ids) > 0 {
		_, _, err = s.call(request.Context(), session, http.MethodPost, "/me/playlists/tracks/add", nil, map[string]any{"guid": id, "trackGUIDs": ids})
		if err != nil {
			return err
		}
	}
	s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"Id": jellyfinID("playlist", id), "Name": stringValue(object(data), "name", "title")})
	return nil
}

func (s *Server) jellyfinPlaylistItems(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	raw := stripJellyfinID(id, "playlist")
	switch request.Method {
	case http.MethodGet:
		start, limit := jellyfinPageParams(request.Form)
		page, err := s.jellyfinPagedMapped(request, session, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {raw}}, start, limit, func(value map[string]any) any {
			s.rememberTrack(value)
			item := jellyfinTrack(value)
			item["PlaylistItemId"] = item["Id"]
			return item
		})
		if err != nil {
			return err
		}
		s.writeJellyfinJSON(writer, http.StatusOK, jellyfinCollectionAt(page.Items, page.Total, start))
		return nil
	case http.MethodPost:
		ids := jellyfinRequestIDs(request.Form, "Ids")
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/tracks/add", nil, map[string]any{"guid": raw, "trackGUIDs": ids})
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	case http.MethodDelete:
		ids := jellyfinRequestIDs(request.Form, "EntryIds")
		if len(ids) == 0 {
			ids = jellyfinRequestIDs(request.Form, "entryIds")
		}
		if len(ids) > 0 {
			resolved := make([]string, 0, len(ids))
			for _, id := range ids {
				if index, parseErr := strconv.Atoi(id); parseErr == nil && index >= 0 {
					page, pageErr := s.jellyfinPagedMapped(request, session, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {raw}}, index, 1, func(value map[string]any) any { return value })
					if pageErr != nil {
						return pageErr
					}
					if len(page.Items) > 0 {
						resolved = append(resolved, entityID(object(page.Items[0])))
					}
				} else {
					resolved = append(resolved, stripJellyfinID(id, "track"))
				}
			}
			ids = resolved
		}
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/tracks/remove", nil, map[string]any{"guid": raw, "trackGUIDs": ids})
		if err != nil {
			return err
		}
		writer.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return &jellyfinUnsupportedError{Message: "Playlist method is not supported"}
	}
}

func (s *Server) jellyfinDeleteItem(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	if kind != "playlist" {
		return &jellyfinUnsupportedError{Message: "Only playlists can be deleted"}
	}
	_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/delete", nil, map[string]any{"guid": raw})
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) jellyfinUpdateItem(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	kind, raw := splitJellyfinID(id)
	if kind != "playlist" {
		return &jellyfinUnsupportedError{Message: "Only playlist metadata can be updated"}
	}
	_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/update", nil, map[string]any{"guid": raw, "name": required(request.Form, "Name")})
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) jellyfinPlaybackInfo(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	track, _, err := s.findTrack(request.Context(), session, stripJellyfinID(id, "track"))
	if err != nil {
		return err
	}
	item := jellyfinTrack(track)
	sources, _ := item["MediaSources"].([]any)
	s.writeJellyfinJSON(writer, http.StatusOK, map[string]any{"MediaSources": sources, "PlaySessionId": jellyfinID("play", entityID(track)), "ErrorCode": nil})
	return nil
}

func (s *Server) jellyfinPlaybackEvent(writer http.ResponseWriter, request *http.Request, session authSession, requestPath string) error {
	if strings.HasSuffix(strings.ToLower(requestPath), "/stopped") {
		id := stripJellyfinID(firstNonEmpty(request.Form.Get("ItemId"), request.Form.Get("itemId")), "track")
		if id != "" {
			_, _, err := s.call(request.Context(), session, http.MethodPost, "/activity/events", nil, map[string]any{"events": []any{map[string]any{"eventType": "track_play", "occurredAt": time.Now().UnixMilli(), "payload": map[string]any{"trackGUID": id}}}})
			if err != nil {
				return err
			}
		}
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) jellyfinScheduledTasks(writer http.ResponseWriter, request *http.Request, session authSession) error {
	task, err := s.jellyfinScheduledTaskValue(request, session)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, []any{task})
	return nil
}

func (s *Server) jellyfinScheduledTask(writer http.ResponseWriter, request *http.Request, session authSession, id string) error {
	if !strings.EqualFold(id, "yinqiao-scan") {
		return &authFailure{Code: 70, Message: "Scheduled task not found"}
	}
	task, err := s.jellyfinScheduledTaskValue(request, session)
	if err != nil {
		return err
	}
	s.writeJellyfinJSON(writer, http.StatusOK, task)
	return nil
}

func (s *Server) jellyfinScheduledTaskValue(request *http.Request, session authSession) (map[string]any, error) {
	data, _, err := s.call(request.Context(), session, http.MethodGet, "/admin/tasks", nil, nil)
	if err != nil {
		return nil, err
	}
	running := false
	for _, value := range list(data) {
		if strings.Contains(strings.ToLower(stringValue(object(value), "status", "state")), "run") {
			running = true
			break
		}
	}
	state := "Idle"
	if running {
		state = "Running"
	}
	return map[string]any{"Name": "Scan Media Library", "Key": "RefreshLibrary", "State": state, "Id": "yinqiao-scan", "CurrentProgressPercentage": 0}, nil
}

func jellyfinAlbum(album map[string]any) map[string]any {
	artists := nestedList(album, "artists")
	artistNames := stringSliceNames(artists)
	artistItems := jellyfinNameIDItems(artists, "artist")
	cover := extractCoverID(album)
	result := jellyfinBaseItem("album", album, "MusicAlbum", true)
	result["Artists"] = artistNames
	result["ArtistItems"] = artistItems
	result["AlbumArtist"] = strings.Join(artistNames, " / ")
	result["AlbumArtists"] = artistItems
	result["ChildCount"] = intValue(album, "trackCount", "totalTrack")
	result["ProductionYear"] = intValue(album, "originalReleaseYear", "year")
	addJellyfinImage(result, cover)
	return result
}

func jellyfinArtist(artist map[string]any) map[string]any {
	result := jellyfinBaseItem("artist", artist, "MusicArtist", false)
	result["ChildCount"] = intValue(artist, "trackCount")
	result["AlbumCount"] = intValue(artist, "albumCount")
	addJellyfinImage(result, extractCoverID(artist))
	return result
}

func jellyfinGenre(genre map[string]any) map[string]any {
	result := jellyfinBaseItem("genre", genre, "MusicGenre", false)
	result["MediaType"] = "Unknown"
	result["ChildCount"] = intValue(genre, "trackCount")
	addJellyfinImage(result, extractCoverID(genre))
	return result
}

func jellyfinPlaylist(playlist map[string]any) map[string]any {
	result := jellyfinBaseItem("playlist", playlist, "Playlist", true)
	result["MediaType"] = "Audio"
	result["CanDelete"] = true
	result["ChildCount"] = intValue(playlist, "trackCount", "songCount")
	addJellyfinImage(result, extractCoverID(playlist))
	return result
}

func jellyfinTrack(track map[string]any) map[string]any {
	result := jellyfinBaseItem("track", track, "Audio", false)
	audio := firstObject(track, "audioSpec")
	album := firstObject(track, "album")
	artists := nestedList(track, "artists")
	duration := intValue(audio, "duration")
	if duration == 0 {
		duration = intValue(track, "duration")
	}
	container := stringValue(audio, "container", "extension", "format")
	codec := stringValue(audio, "codec", "format", "extension")
	result["MediaType"] = "Audio"
	result["RunTimeTicks"] = duration * 10_000
	result["IndexNumber"] = intValue(track, "trackNo")
	result["ParentIndexNumber"] = intValue(track, "discNo")
	result["ProductionYear"] = intValue(track, "year")
	result["Artists"] = stringSliceNames(artists)
	result["ArtistItems"] = jellyfinNameIDItems(artists, "artist")
	result["Album"] = stringValue(album, "name", "title")
	result["AlbumId"] = jellyfinID("album", entityID(album))
	result["AlbumArtist"] = joinedNames(artists)
	result["AlbumArtists"] = jellyfinNameIDItems(artists, "artist")
	stream := map[string]any{"Codec": codec, "Type": "Audio", "Index": 0, "BitRate": intValue(audio, "bitrate"), "BitDepth": intValue(audio, "bitDepth"), "Channels": intValue(audio, "channel", "channels"), "SampleRate": intValue(audio, "sampleRate"), "IsDefault": true, "IsExternal": false}
	source := map[string]any{"Protocol": "File", "Id": result["Id"], "Path": stringValue(audio, "path"), "Type": "Default", "Container": container, "Size": intValue(audio, "size"), "Name": result["Name"], "RunTimeTicks": result["RunTimeTicks"], "SupportsTranscoding": false, "SupportsDirectStream": true, "SupportsDirectPlay": true, "MediaStreams": []any{stream}, "Bitrate": intValue(audio, "bitrate"), "RequiredHttpHeaders": map[string]any{}}
	result["MediaSources"] = []any{source}
	result["MediaStreams"] = []any{stream}
	result["UserData"] = jellyfinUserData(boolValue(track, "isFavorite"))
	cover := extractCoverID(track)
	if cover == "" {
		cover = extractCoverID(album)
	}
	addJellyfinImage(result, cover)
	if cover != "" {
		result["AlbumPrimaryImageTag"] = jellyfinTag(cover)
	}
	return result
}

func jellyfinTrackRecord(value map[string]any) map[string]any {
	for _, key := range []string{"track", "song", "item"} {
		if track := firstObject(value, key); entityID(track) != "" {
			return track
		}
	}
	return value
}

func jellyfinTrackFromRecord(track, record map[string]any, endpoint string) map[string]any {
	result := jellyfinTrack(track)
	userData := object(result["UserData"])
	if endpoint == "/me/favorites" {
		userData["IsFavorite"] = true
	}
	if endpoint == "/me/history" {
		playCount := intValue(record, "playCount")
		if playCount == 0 {
			playCount = intValue(track, "playCount")
		}
		userData["PlayCount"] = playCount
		userData["Played"] = playCount > 0
		if playedAt := jellyfinRecordTime(record, "playedAt", "playAt", "updatedAt"); playedAt != "" {
			userData["LastPlayedDate"] = playedAt
		}
	}
	result["UserData"] = userData
	return result
}

func jellyfinRecordTime(value map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := value[key]
		if !ok {
			continue
		}
		if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
		milliseconds := intValue(value, key)
		if milliseconds == 0 {
			continue
		}
		if milliseconds < 100_000_000_000 {
			milliseconds *= 1000
		}
		return time.UnixMilli(milliseconds).UTC().Format(time.RFC3339Nano)
	}
	return ""
}

func jellyfinBaseItem(kind string, data map[string]any, itemType string, folder bool) map[string]any {
	id := entityID(data)
	name := stringValue(data, "name", "title")
	return map[string]any{"Name": name, "ServerId": jellyfinServerID, "Id": jellyfinID(kind, id), "Etag": jellyfinTag(kind + ":" + id), "SortName": name, "Type": itemType, "IsFolder": folder, "CanDelete": kind == "playlist", "CanDownload": kind == "track", "LocationType": "FileSystem", "UserData": jellyfinUserData(false), "ImageTags": map[string]any{}, "BackdropImageTags": []any{}}
}

func jellyfinUserData(favorite bool) map[string]any {
	return map[string]any{"PlaybackPositionTicks": 0, "PlayCount": 0, "IsFavorite": favorite, "Played": false}
}
func addJellyfinImage(item map[string]any, coverID string) {
	if coverID != "" {
		item["ImageTags"] = map[string]any{"Primary": jellyfinTag(coverID)}
		item["PrimaryImageAspectRatio"] = 1
	}
}
func jellyfinTag(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}
func jellyfinID(kind, id string) string {
	if id == "" {
		return ""
	}
	return "fnm-" + kind + "-" + url.PathEscape(id)
}
func ensureJellyfinID(kind, id string) string {
	if existingKind, _ := splitJellyfinID(id); existingKind != "" {
		return id
	}
	return jellyfinID(kind, id)
}
func splitJellyfinID(id string) (string, string) {
	for _, kind := range []string{"track", "album", "artist", "genre", "playlist", "user", "library", "play"} {
		prefix := "fnm-" + kind + "-"
		if strings.HasPrefix(id, prefix) {
			raw, _ := url.PathUnescape(strings.TrimPrefix(id, prefix))
			return kind, raw
		}
	}
	return "", id
}
func stripJellyfinID(id, expected string) string {
	kind, raw := splitJellyfinID(id)
	if kind == "" || kind == expected {
		return raw
	}
	return raw
}

func jellyfinNameIDItems(values []any, kind string) []any {
	result := make([]any, 0)
	for _, value := range values {
		entry := object(value)
		result = append(result, map[string]any{"Name": stringValue(entry, "name", "title"), "Id": jellyfinID(kind, entityID(entry))})
	}
	return result
}
func stringSliceNames(values []any) []string {
	result := make([]string, 0)
	for _, value := range values {
		if name := stringValue(object(value), "name", "title"); name != "" {
			result = append(result, name)
		}
	}
	return result
}
func jellyfinCollection(items []any, total int) map[string]any {
	return jellyfinCollectionAt(items, total, 0)
}
func jellyfinCollectionAt(items []any, total, start int) map[string]any {
	if items == nil {
		items = []any{}
	}
	return map[string]any{"Items": items, "TotalRecordCount": total, "StartIndex": start}
}
func jellyfinPageParams(values url.Values) (int, int) {
	return boundedInt(jellyfinFormValue(values, "StartIndex"), 0, 0, 1_000_000), boundedInt(jellyfinFormValue(values, "Limit"), 100, 1, 10_000)
}

func jellyfinHasFilter(values url.Values, expected string) bool {
	return jellyfinHasListValue(values, "Filters", expected)
}

func jellyfinHasSort(values url.Values, expected string) bool {
	return jellyfinHasListValue(values, "SortBy", expected)
}

func jellyfinHasListValue(values url.Values, key, expected string) bool {
	for actualKey, entries := range values {
		if !strings.EqualFold(actualKey, key) {
			continue
		}
		for _, entry := range entries {
			for _, value := range commaValues(entry) {
				if strings.EqualFold(value, expected) {
					return true
				}
			}
		}
	}
	return false
}

func addJellyfinSort(query url.Values, values url.Values, kind string) {
	order := "asc"
	if strings.EqualFold(jellyfinFormValue(values, "SortOrder"), "Descending") {
		order = "desc"
	}
	for _, value := range commaValues(jellyfinFormValue(values, "SortBy")) {
		field := ""
		switch strings.ToLower(value) {
		case "sortname", "name":
			field = "name"
		case "datecreated":
			field = "createdAt"
		case "productionyear":
			if kind == "album" {
				field = "originalReleaseYear"
			} else {
				field = "year"
			}
		case "album":
			field = "album"
		case "artist":
			field = "artist"
		case "indexnumber":
			field = "trackNo"
		}
		if field != "" {
			query.Set("sort", field+","+order)
			return
		}
	}
}

func jellyfinFormValue(values url.Values, expected string) string {
	if value := values.Get(expected); value != "" {
		return value
	}
	for key, entries := range values {
		if strings.EqualFold(key, expected) && len(entries) > 0 {
			return entries[0]
		}
	}
	return ""
}
func commaValues(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
func jellyfinRequestIDs(values url.Values, key string) []string {
	raw := values[key]
	if len(raw) == 0 {
		raw = commaValues(values.Get(key))
	}
	result := make([]string, 0)
	for _, value := range raw {
		for _, id := range commaValues(value) {
			result = append(result, stripJellyfinID(id, "track"))
		}
	}
	return result
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func requestHostURL(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := firstNonEmpty(request.Header.Get("X-Forwarded-Host"), request.Host)
	return scheme + "://" + host
}
func pathTail(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
func jellyfinImageItemID(requestPath string) string {
	value := requestPath
	if len(value) >= len("/Items/") {
		value = value[len("/Items/"):]
	}
	if index := strings.Index(value, "/"); index >= 0 {
		return value[:index]
	}
	return value
}
func albumHasArtist(album map[string]any, ids []string) bool {
	for _, artist := range nestedList(album, "artists") {
		candidate := entityID(object(artist))
		for _, id := range ids {
			if candidate == stripJellyfinID(id, "artist") {
				return true
			}
		}
	}
	return false
}
func trackHasArtist(track map[string]any, id string) bool {
	for _, artist := range nestedList(track, "artists") {
		if entityID(object(artist)) == id {
			return true
		}
	}
	return false
}
func trackHasArtistName(track map[string]any, names []string) bool {
	for _, artist := range nestedList(track, "artists") {
		for _, name := range names {
			if strings.EqualFold(stringValue(object(artist), "name", "title"), name) {
				return true
			}
		}
	}
	return false
}
func randomShuffle(values []any) {
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(values), func(i, j int) { values[i], values[j] = values[j], values[i] })
}

type captureResponseWriter struct {
	header http.Header
	body   []byte
	status int
}

type jellyfinResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *jellyfinResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *jellyfinResponseWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	written, err := w.ResponseWriter.Write(value)
	w.bytes += int64(written)
	return written, err
}

func (w *jellyfinResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func newCaptureResponseWriter() *captureResponseWriter {
	return &captureResponseWriter{header: make(http.Header)}
}
func (w *captureResponseWriter) Header() http.Header    { return w.header }
func (w *captureResponseWriter) WriteHeader(status int) { w.status = status }
func (w *captureResponseWriter) Write(value []byte) (int, error) {
	w.body = append(w.body, value...)
	return len(value), nil
}

func (s *Server) writeJellyfinAPIError(writer http.ResponseWriter, err error) {
	var unsupported *jellyfinUnsupportedError
	if errors.As(err, &unsupported) {
		s.writeJellyfinError(writer, http.StatusNotImplemented, unsupported.Message)
		return
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		status := apiErr.Status
		if status < 400 {
			status = http.StatusBadGateway
		}
		s.writeJellyfinError(writer, status, apiErr.Message)
		return
	}
	var authErr *authFailure
	if errors.As(err, &authErr) {
		status := http.StatusBadRequest
		if authErr.Code == 40 {
			status = http.StatusUnauthorized
		}
		if authErr.Code == 70 {
			status = http.StatusNotFound
		}
		s.writeJellyfinError(writer, status, authErr.Message)
		return
	}
	s.writeJellyfinError(writer, http.StatusBadGateway, err.Error())
}

func (s *Server) writeJellyfinError(writer http.ResponseWriter, status int, message string) {
	s.writeJellyfinJSON(writer, status, map[string]any{"type": "https://jellyfin.org/docs/general/networking/", "title": http.StatusText(status), "status": status, "detail": message})
}
func (s *Server) writeJellyfinJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
