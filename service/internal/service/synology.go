package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type synologyAPIInfo struct {
	MinVersion int    `json:"minVersion"`
	MaxVersion int    `json:"maxVersion"`
	Path       string `json:"path"`
}

var synologyAPIs = map[string]synologyAPIInfo{
	"SYNO.API.Info":              {1, 1, "query.cgi"},
	"SYNO.API.Auth":              {1, 7, "entry.cgi"},
	"SYNO.AudioStation.Info":     {1, 6, "AudioStation/info.cgi"},
	"SYNO.AudioStation.Album":    {1, 3, "AudioStation/album.cgi"},
	"SYNO.AudioStation.Artist":   {1, 4, "AudioStation/artist.cgi"},
	"SYNO.AudioStation.Song":     {1, 3, "AudioStation/song.cgi"},
	"SYNO.AudioStation.Genre":    {1, 3, "AudioStation/genre.cgi"},
	"SYNO.AudioStation.Folder":   {1, 3, "AudioStation/folder.cgi"},
	"SYNO.AudioStation.Search":   {1, 1, "AudioStation/search.cgi"},
	"SYNO.AudioStation.Cover":    {1, 3, "AudioStation/cover.cgi"},
	"SYNO.AudioStation.Stream":   {1, 2, "AudioStation/stream.cgi"},
	"SYNO.AudioStation.Download": {1, 1, "AudioStation/download.cgi"},
	"SYNO.AudioStation.Lyrics":   {1, 2, "AudioStation/lyrics.cgi"},
	"SYNO.AudioStation.Playlist": {1, 3, "AudioStation/playlist.cgi"},
}

func isSynologyPath(requestPath string) bool {
	return requestPath == "/webman/pingpong.cgi" || strings.HasPrefix(requestPath, "/webapi/")
}

func (s *Server) handleSynology(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		s.writeSynologyError(writer, http.StatusMethodNotAllowed, 101, "不支持的请求方法")
		return
	}
	if request.URL.Path == "/webman/pingpong.cgi" {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("pong"))
		return
	}
	if err := parseProtocolForm(request); err != nil {
		s.writeSynologyError(writer, http.StatusBadRequest, 101, "请求参数无效")
		return
	}

	api := strings.TrimSpace(request.Form.Get("api"))
	method := strings.ToLower(strings.TrimSpace(request.Form.Get("method")))
	if request.URL.Path == "/webapi/query.cgi" || api == "SYNO.API.Info" {
		s.handleSynologyAPIInfo(writer, request, method)
		return
	}
	if api == "SYNO.API.Auth" || request.URL.Path == "/webapi/auth.cgi" {
		s.handleSynologyAuth(writer, request, method)
		return
	}
	if api == "" {
		api = synologyAPIFromPath(request.URL.Path)
	}
	if _, ok := synologyAPIs[api]; !ok {
		s.writeSynologyError(writer, http.StatusBadRequest, 102, "接口不支持")
		return
	}

	session, err := s.synologyAuthenticate(request)
	if err != nil {
		s.writeSynologyError(writer, http.StatusUnauthorized, 106, "会话已失效，请重新登录")
		return
	}
	if err := s.dispatchSynology(writer, request, api, method, session); err != nil {
		s.logger.Printf("synology api=%s method=%s user=%q error=%v", api, method, session.Username, err)
		s.writeSynologyAPIError(writer, err)
	}
}

func parseProtocolForm(request *http.Request) error {
	if err := request.ParseForm(); err != nil {
		return err
	}
	if request.Body == nil || request.Body == http.NoBody || request.ContentLength == 0 || !strings.Contains(strings.ToLower(request.Header.Get("Content-Type")), "application/json") {
		return nil
	}
	var body map[string]any
	if err := json.NewDecoder(io.LimitReader(request.Body, 2<<20)).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	for key, value := range body {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				request.Form.Add(key, fmt.Sprint(item))
			}
		default:
			request.Form.Set(key, fmt.Sprint(value))
		}
	}
	return nil
}

func synologyAPIFromPath(requestPath string) string {
	base := path.Base(strings.TrimSuffix(requestPath, "/0.mp3"))
	return map[string]string{
		"info.cgi": "SYNO.AudioStation.Info", "album.cgi": "SYNO.AudioStation.Album",
		"artist.cgi": "SYNO.AudioStation.Artist", "song.cgi": "SYNO.AudioStation.Song",
		"genre.cgi": "SYNO.AudioStation.Genre", "folder.cgi": "SYNO.AudioStation.Folder",
		"search.cgi": "SYNO.AudioStation.Search", "cover.cgi": "SYNO.AudioStation.Cover",
		"stream.cgi": "SYNO.AudioStation.Stream", "download.cgi": "SYNO.AudioStation.Download",
		"lyrics.cgi": "SYNO.AudioStation.Lyrics", "playlist.cgi": "SYNO.AudioStation.Playlist",
	}[base]
}

func (s *Server) handleSynologyAPIInfo(writer http.ResponseWriter, request *http.Request, method string) {
	if method != "" && method != "query" {
		s.writeSynologyError(writer, http.StatusBadRequest, 103, "接口方法不支持")
		return
	}
	query := strings.TrimSpace(request.Form.Get("query"))
	data := map[string]synologyAPIInfo{}
	if query == "" || query == "all" {
		for name, info := range synologyAPIs {
			data[name] = info
		}
	} else {
		for _, name := range strings.Split(query, ",") {
			name = strings.TrimSpace(name)
			if strings.HasSuffix(name, "*") {
				prefix := strings.TrimSuffix(name, "*")
				for candidate, info := range synologyAPIs {
					if strings.HasPrefix(candidate, prefix) {
						data[candidate] = info
					}
				}
				continue
			}
			if info, ok := synologyAPIs[name]; ok {
				data[name] = info
			}
		}
	}
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": data})
}

func (s *Server) handleSynologyAuth(writer http.ResponseWriter, request *http.Request, method string) {
	switch method {
	case "login":
		username := strings.TrimSpace(request.Form.Get("account"))
		password := request.Form.Get("passwd")
		if username == "" || password == "" {
			s.writeSynologyError(writer, http.StatusBadRequest, 400, "账号和密码不能为空")
			return
		}
		session, err := s.auth.loginAndRemember(request.Context(), username, password)
		if err != nil {
			s.writeSynologyError(writer, http.StatusUnauthorized, 404, "飞牛音乐账号或密码错误")
			return
		}
		sid, err := randomAmpacheToken()
		if err != nil {
			s.writeSynologyError(writer, http.StatusInternalServerError, 100, "无法创建会话")
			return
		}
		didHash := sha256.Sum256([]byte("fnmusic-synology:" + username))
		did := hex.EncodeToString(didHash[:12])
		s.synologyMu.Lock()
		s.synologySessions[sid] = session
		s.synologyMu.Unlock()
		http.SetCookie(writer, &http.Cookie{Name: "id", Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		http.SetCookie(writer, &http.Cookie{Name: "did", Value: did, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"sid": sid, "did": did, "is_portal_port": false}})
	case "logout":
		sid := synologySID(request)
		s.synologyMu.Lock()
		delete(s.synologySessions, sid)
		s.synologyMu.Unlock()
		s.writeSynologyJSON(writer, map[string]any{"success": true})
	default:
		s.writeSynologyError(writer, http.StatusBadRequest, 103, "认证方法不支持")
	}
}

func synologySID(request *http.Request) string {
	if sid := strings.TrimSpace(request.Form.Get("_sid")); sid != "" {
		return sid
	}
	if cookie, err := request.Cookie("id"); err == nil {
		return cookie.Value
	}
	return ""
}

func (s *Server) synologyAuthenticate(request *http.Request) (authSession, error) {
	s.synologyMu.RLock()
	session, ok := s.synologySessions[synologySID(request)]
	s.synologyMu.RUnlock()
	if !ok {
		return authSession{}, &authFailure{Code: 106, Message: "Synology session is invalid"}
	}
	return session, nil
}

func (s *Server) dispatchSynology(writer http.ResponseWriter, request *http.Request, api, method string, session authSession) error {
	switch api {
	case "SYNO.AudioStation.Info":
		if method != "getinfo" {
			return s.synologyUnsupportedMethod(writer)
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{
			"path": "/yinqiao/ui/", "version": map[string]any{"major": "7", "minor": "2", "build": serverVersion},
			"yinqiao": true,
		}})
		return nil
	case "SYNO.AudioStation.Album":
		if method != "list" {
			return s.synologyUnsupportedMethod(writer)
		}
		return s.synologyAlbums(writer, request, session)
	case "SYNO.AudioStation.Artist":
		if method != "list" {
			return s.synologyUnsupportedMethod(writer)
		}
		return s.synologyArtists(writer, request, session)
	case "SYNO.AudioStation.Song":
		return s.synologySongs(writer, request, method, session)
	case "SYNO.AudioStation.Genre":
		if method != "list" {
			return s.synologyUnsupportedMethod(writer)
		}
		return s.synologyGenres(writer, request, session)
	case "SYNO.AudioStation.Folder":
		if method != "list" && method != "getinfo" {
			return s.synologyUnsupportedMethod(writer)
		}
		return s.synologyFolders(writer, request, session)
	case "SYNO.AudioStation.Search":
		if method != "list" {
			return s.synologyUnsupportedMethod(writer)
		}
		return s.synologySearch(writer, request, session)
	case "SYNO.AudioStation.Cover":
		return s.synologyCover(writer, request, session)
	case "SYNO.AudioStation.Stream", "SYNO.AudioStation.Download":
		id := stripSynologyTrackID(required(request.Form, "id"))
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(id)+"/stream", nil)
	case "SYNO.AudioStation.Lyrics":
		if method != "getlyrics" {
			return s.synologyUnsupportedMethod(writer)
		}
		lyrics, _, err := s.lyricText(request.Context(), session, stripSynologyTrackID(required(request.Form, "id")))
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"lyrics": lyrics}})
		return nil
	case "SYNO.AudioStation.Playlist":
		return s.synologyPlaylists(writer, request, method, session)
	default:
		return s.synologyUnsupportedMethod(writer)
	}
}

func (s *Server) synologyUnsupportedMethod(writer http.ResponseWriter) error {
	s.writeSynologyError(writer, http.StatusBadRequest, 103, "接口方法不支持")
	return nil
}

func (s *Server) synologyAlbums(writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/albums", url.Values{"page": {"1"}, "size": {"5000"}}, nil)
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(request.Form.Get("filter")))
	artistFilter := strings.ToLower(strings.TrimSpace(request.Form.Get("artist")))
	items := make([]any, 0)
	for _, value := range list(data) {
		album := object(value)
		name := stringValue(album, "name", "title")
		artist := joinedNames(nestedList(album, "artists"))
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}
		if artistFilter != "" && !strings.Contains(strings.ToLower(artist), artistFilter) {
			continue
		}
		items = append(items, map[string]any{
			"id": "album_" + entityID(album), "name": name, "artist": artist, "album_artist": artist,
			"display_artist": artist, "year": intValue(album, "originalReleaseYear", "year"),
			"additional": map[string]any{"avg_rating": map[string]any{"rating": 0}},
		})
	}
	page, offset, total := synologyPage(items, request.Form)
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"albums": page, "offset": offset, "total": total}})
	return nil
}

func (s *Server) synologyArtists(writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/artists/all", nil, nil)
	if err != nil {
		return err
	}
	filter := strings.ToLower(strings.TrimSpace(request.Form.Get("filter")))
	items := make([]any, 0)
	for _, value := range list(data) {
		artist := object(value)
		name := stringValue(artist, "name", "title")
		if filter != "" && !strings.Contains(strings.ToLower(name), filter) {
			continue
		}
		items = append(items, map[string]any{"id": "artist_" + entityID(artist), "name": name, "additional": map[string]any{"avg_rating": map[string]any{"rating": 0}}})
	}
	page, offset, total := synologyPage(items, request.Form)
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"artists": page, "offset": offset, "total": total}})
	return nil
}

func (s *Server) synologySongs(writer http.ResponseWriter, request *http.Request, method string, session authSession) error {
	if method == "setrating" {
		id := stripSynologyTrackID(required(request.Form, "id"))
		endpoint := "/me/favorites/remove"
		if boundedInt(request.Form.Get("rating"), 0, 0, 5) == 5 {
			endpoint = "/me/favorites"
		}
		_, _, err := s.call(request.Context(), session, http.MethodPost, endpoint, nil, map[string]any{"trackGUID": id})
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true})
		return nil
	}
	if method == "getinfo" {
		track, _, err := s.findTrack(request.Context(), session, stripSynologyTrackID(required(request.Form, "id")))
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"songs": []any{synologySong(track)}}})
		return nil
	}
	if method != "list" && method != "search" {
		return s.synologyUnsupportedMethod(writer)
	}
	return s.synologySongList(writer, request, session)
}

func (s *Server) synologySongList(writer http.ResponseWriter, request *http.Request, session authSession) error {
	keyword := strings.TrimSpace(request.Form.Get("keyword"))
	if keyword == "" {
		keyword = strings.TrimSpace(request.Form.Get("title"))
	}
	var data any
	var err error
	genreFilter := strings.TrimSpace(request.Form.Get("genre"))
	if genreFilter != "" {
		genres, updated, genreErr := s.call(request.Context(), session, http.MethodGet, "/catalog/genres", url.Values{"page": {"1"}, "size": {"1000"}}, nil)
		if genreErr != nil {
			return genreErr
		}
		session = updated
		genreID := ""
		for _, value := range list(genres) {
			entry := object(value)
			if strings.EqualFold(stringValue(entry, "name", "title"), genreFilter) {
				genreID = entityID(entry)
				break
			}
		}
		if genreID == "" {
			data = map[string]any{"list": []any{}, "total": 0}
		} else {
			data, _, err = s.call(request.Context(), session, http.MethodGet, "/catalog/tracks/by-genre", url.Values{"genreGUID": {genreID}, "page": {"1"}, "size": {"5000"}}, nil)
		}
	} else if strings.TrimSpace(request.Form.Get("rating_filter")) == "5" {
		data, _, err = s.call(request.Context(), session, http.MethodGet, "/me/favorites", url.Values{"page": {"1"}, "size": {"5000"}}, nil)
	} else if keyword != "" {
		data, _, err = s.searchItems(request.Context(), session, "/search/tracks", keyword, 0, 5000)
	} else {
		data, _, err = s.call(request.Context(), session, http.MethodGet, "/catalog/tracks", url.Values{"page": {"1"}, "size": {"5000"}}, nil)
	}
	if err != nil {
		return err
	}
	albumFilter := strings.ToLower(strings.TrimSpace(request.Form.Get("album")))
	artistFilter := strings.ToLower(strings.TrimSpace(request.Form.Get("artist")))
	items := make([]any, 0)
	for _, value := range list(data) {
		track := object(value)
		album := stringValue(firstObject(track, "album"), "name", "title")
		artist := joinedNames(nestedList(track, "artists"))
		if albumFilter != "" && !strings.EqualFold(album, albumFilter) {
			continue
		}
		if artistFilter != "" && !strings.Contains(strings.ToLower(artist), artistFilter) {
			continue
		}
		s.rememberTrack(track)
		items = append(items, synologySong(track))
	}
	if strings.EqualFold(request.Form.Get("sort_by"), "random") {
		randomShuffle(items)
	}
	page, offset, total := synologyPage(items, request.Form)
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"songs": page, "offset": offset, "total": total}})
	return nil
}

func synologySong(track map[string]any) map[string]any {
	audio := firstObject(track, "audioSpec")
	album := firstObject(track, "album")
	artists := joinedNames(nestedList(track, "artists"))
	duration := intValue(audio, "duration")
	if duration == 0 {
		duration = intValue(track, "duration")
	}
	pathValue := stringValue(audio, "path")
	if pathValue == "" {
		pathValue = "/music/" + stringValue(track, "title", "name")
	}
	rating := int64(0)
	if boolValue(track, "isFavorite") {
		rating = 5
	}
	return map[string]any{
		"id": "music_" + entityID(track), "path": pathValue, "title": stringValue(track, "title", "name"), "type": "file",
		"additional": map[string]any{
			"song_tag":    map[string]any{"album": stringValue(album, "name", "title"), "album_artist": artists, "artist": artists, "comment": "", "composer": "", "disc": intValue(track, "discNo"), "track": intValue(track, "trackNo"), "year": intValue(track, "year")},
			"song_audio":  map[string]any{"bitrate": intValue(audio, "bitrate"), "channel": intValue(audio, "channel", "channels"), "codec": stringValue(audio, "codec", "format"), "container": stringValue(audio, "container", "extension"), "duration": millisToSeconds(duration), "filesize": intValue(audio, "size"), "frequency": intValue(audio, "sampleRate")},
			"song_rating": map[string]any{"rating": rating},
		},
	}
}

func stripSynologyTrackID(id string) string { return strings.TrimPrefix(id, "music_") }

func (s *Server) synologyGenres(writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/genres", url.Values{"page": {"1"}, "size": {"1000"}}, nil)
	if err != nil {
		return err
	}
	items := make([]any, 0)
	for _, value := range list(data) {
		genre := object(value)
		items = append(items, map[string]any{"id": "genre_" + entityID(genre), "name": stringValue(genre, "name", "title"), "additional": map[string]any{"avg_rating": map[string]any{"rating": 0}}})
	}
	page, offset, total := synologyPage(items, request.Form)
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"genres": page, "offset": offset, "total": total}})
	return nil
}

func (s *Server) synologyFolders(writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := strings.TrimSpace(request.Form.Get("id"))
	items := make([]any, 0)
	if id == "" || id == "root" {
		data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/albums", url.Values{"page": {"1"}, "size": {"5000"}}, nil)
		if err != nil {
			return err
		}
		for _, value := range list(data) {
			album := object(value)
			name := stringValue(album, "name", "title")
			items = append(items, map[string]any{"id": "album_" + entityID(album), "path": "/music/" + name, "title": name, "type": "folder", "is_personal": false})
		}
	} else if strings.HasPrefix(id, "album_") {
		albumID := strings.TrimPrefix(id, "album_")
		data, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/tracks/by-album", url.Values{"albumGUID": {albumID}, "page": {"1"}, "size": {"10000"}}, nil)
		if err != nil {
			return err
		}
		for _, value := range list(data) {
			track := object(value)
			s.rememberTrack(track)
			items = append(items, synologySong(track))
		}
	}
	page, offset, total := synologyPage(items, request.Form)
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"id": id, "items": page, "offset": offset, "total": total, "folder_total": total}})
	return nil
}

func (s *Server) synologySearch(writer http.ResponseWriter, request *http.Request, session authSession) error {
	keyword := strings.TrimSpace(request.Form.Get("keyword"))
	offset := boundedInt(request.Form.Get("offset"), 0, 0, 1_000_000)
	limit := boundedInt(request.Form.Get("limit"), 50, 1, 500)
	artists, session, err := s.searchItems(request.Context(), session, "/search/artists", keyword, offset, limit)
	if err != nil {
		return err
	}
	albums, session, err := s.searchItems(request.Context(), session, "/search/albums", keyword, offset, limit)
	if err != nil {
		return err
	}
	tracks, _, err := s.searchItems(request.Context(), session, "/search/tracks", keyword, offset, limit)
	if err != nil {
		return err
	}
	artistItems := make([]any, 0)
	for _, value := range list(artists) {
		artistItems = append(artistItems, map[string]any{"id": "artist_" + entityID(object(value)), "name": stringValue(object(value), "name", "title")})
	}
	albumItems := make([]any, 0)
	for _, value := range list(albums) {
		album := object(value)
		artist := joinedNames(nestedList(album, "artists"))
		albumItems = append(albumItems, map[string]any{"id": "album_" + entityID(album), "name": stringValue(album, "name", "title"), "artist": artist, "album_artist": artist, "display_artist": artist, "year": intValue(album, "originalReleaseYear", "year")})
	}
	trackItems := make([]any, 0)
	for _, value := range list(tracks) {
		track := object(value)
		s.rememberTrack(track)
		trackItems = append(trackItems, synologySong(track))
	}
	s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{
		"artists": artistItems, "artistTotal": len(artistItems), "albums": albumItems, "albumTotal": len(albumItems), "songs": trackItems, "songTotal": len(trackItems),
	}})
	return nil
}

func (s *Server) synologyCover(writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := stripSynologyTrackID(strings.TrimSpace(request.Form.Get("id")))
	coverID := ""
	if id != "" {
		track, updated, err := s.findTrack(request.Context(), session, id)
		session = updated
		if err == nil {
			coverID = extractCoverID(track)
			if coverID == "" {
				coverID = extractCoverID(firstObject(track, "album"))
			}
		}
	}
	if coverID == "" {
		albumName := strings.TrimSpace(request.Form.Get("album_name"))
		artistName := strings.TrimSpace(request.Form.Get("artist_name"))
		endpoint := "/catalog/albums"
		if albumName == "" && artistName != "" {
			endpoint = "/catalog/artists/all"
		}
		query := url.Values{"page": {"1"}, "size": {"5000"}}
		if endpoint == "/catalog/artists/all" {
			query = nil
		}
		data, updated, err := s.call(request.Context(), session, http.MethodGet, endpoint, query, nil)
		session = updated
		if err == nil {
			for _, value := range list(data) {
				entry := object(value)
				name := stringValue(entry, "name", "title")
				if (albumName != "" && strings.EqualFold(name, albumName)) || (artistName != "" && strings.EqualFold(name, artistName)) {
					coverID = extractCoverID(entry)
					break
				}
			}
		}
	}
	if coverID == "" {
		return &authFailure{Code: 70, Message: "Cover not found"}
	}
	return s.proxyBinary(writer, request, session, "/media/covers/"+url.PathEscape(coverID), nil)
}

func (s *Server) synologyPlaylists(writer http.ResponseWriter, request *http.Request, method string, session authSession) error {
	switch method {
	case "list":
		data, _, err := s.call(request.Context(), session, http.MethodGet, "/me/playlists", nil, nil)
		if err != nil {
			return err
		}
		items := make([]any, 0)
		for _, value := range list(data) {
			item := object(value)
			items = append(items, map[string]any{"id": "playlist_personal_normal/" + entityID(item), "library": "personal", "name": stringValue(item, "name", "title"), "sharing_status": "none", "type": "normal"})
		}
		page, offset, total := synologyPage(items, request.Form)
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"playlists": page, "offset": offset, "total": total}})
	case "getinfo":
		id := stripSynologyPlaylistID(required(request.Form, "id"))
		data, session, err := s.call(request.Context(), session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {id}}, nil)
		if err != nil {
			return err
		}
		tracks, _, err := s.call(request.Context(), session, http.MethodGet, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {id}, "page": {"1"}, "size": {"10000"}}, nil)
		if err != nil {
			return err
		}
		items := make([]any, 0)
		for _, value := range list(tracks) {
			track := object(value)
			s.rememberTrack(track)
			items = append(items, synologySong(track))
		}
		page, offset, total := synologyPage(items, request.Form)
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"id": "playlist_personal_normal/" + id, "name": stringValue(object(data), "name", "title"), "songs": page, "offset": offset, "total": total}})
	case "create":
		data, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists", nil, map[string]any{"name": required(request.Form, "name"), "coverId": nil})
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true, "data": map[string]any{"id": "playlist_personal_normal/" + entityID(object(data))}})
	case "delete":
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/delete", nil, map[string]any{"guid": stripSynologyPlaylistID(required(request.Form, "id"))})
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true})
	case "rename":
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/update", nil, map[string]any{"guid": stripSynologyPlaylistID(required(request.Form, "id")), "name": required(request.Form, "new_name")})
		if err != nil {
			return err
		}
		s.writeSynologyJSON(writer, map[string]any{"success": true})
	case "updatesongs":
		return s.synologyUpdatePlaylist(writer, request, session)
	default:
		return s.synologyUnsupportedMethod(writer)
	}
	return nil
}

func (s *Server) synologyUpdatePlaylist(writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := stripSynologyPlaylistID(required(request.Form, "id"))
	offset := boundedInt(request.Form.Get("offset"), -1, -1, 1_000_000)
	limit := boundedInt(request.Form.Get("limit"), 0, 0, 1_000_000)
	if offset >= 0 && limit > 0 {
		data, updated, err := s.call(request.Context(), session, http.MethodGet, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {id}, "page": {"1"}, "size": {"10000"}}, nil)
		if err != nil {
			return err
		}
		session = updated
		values := list(data)
		end := min(offset+limit, len(values))
		ids := make([]string, 0)
		if offset < len(values) {
			for _, value := range values[offset:end] {
				ids = append(ids, entityID(object(value)))
			}
		}
		if len(ids) > 0 {
			_, updated, err = s.call(request.Context(), session, http.MethodPost, "/me/playlists/tracks/remove", nil, map[string]any{"guid": id, "trackGUIDs": ids})
			if err != nil {
				return err
			}
			session = updated
		}
	}
	ids := synologyIDs(request.Form)
	if len(ids) > 0 {
		_, _, err := s.call(request.Context(), session, http.MethodPost, "/me/playlists/tracks/add", nil, map[string]any{"guid": id, "trackGUIDs": ids})
		if err != nil {
			return err
		}
	}
	s.writeSynologyJSON(writer, map[string]any{"success": true})
	return nil
}

func synologyIDs(values url.Values) []string {
	raw := values["songs"]
	if len(raw) == 1 && strings.HasPrefix(strings.TrimSpace(raw[0]), "[") {
		var parsed []string
		if json.Unmarshal([]byte(raw[0]), &parsed) == nil {
			raw = parsed
		}
	}
	result := make([]string, 0)
	for _, value := range raw {
		for _, id := range strings.Split(value, ",") {
			id = stripSynologyTrackID(strings.Trim(strings.TrimSpace(id), `"`))
			if id != "" {
				result = append(result, id)
			}
		}
	}
	return result
}

func stripSynologyPlaylistID(id string) string {
	return strings.TrimPrefix(id, "playlist_personal_normal/")
}

func synologyPage(values []any, form url.Values) ([]any, int, int) {
	offset := boundedInt(form.Get("offset"), 0, 0, 1_000_000)
	limit := boundedInt(form.Get("limit"), 500, 1, 10_000)
	start := min(offset, len(values))
	end := min(start+limit, len(values))
	return values[start:end], start, len(values)
}

func (s *Server) writeSynologyAPIError(writer http.ResponseWriter, err error) {
	if failure, ok := err.(*authFailure); ok && failure.Code == 70 {
		s.writeSynologyError(writer, http.StatusNotFound, 105, failure.Message)
		return
	}
	if apiErr, ok := err.(*apiError); ok {
		status := apiErr.Status
		code := 100
		if status == http.StatusUnauthorized {
			code = 106
		}
		if status == http.StatusForbidden {
			code = 105
		}
		s.writeSynologyError(writer, status, code, apiErr.Message)
		return
	}
	s.writeSynologyError(writer, http.StatusBadGateway, 100, err.Error())
}

func (s *Server) writeSynologyError(writer http.ResponseWriter, status, code int, message string) {
	s.writeSynologyJSONStatus(writer, http.StatusOK, map[string]any{"success": false, "error": map[string]any{"code": code, "message": message, "httpStatus": status}})
}

func (s *Server) writeSynologyJSON(writer http.ResponseWriter, payload any) {
	s.writeSynologyJSONStatus(writer, http.StatusOK, payload)
}

func (s *Server) writeSynologyJSONStatus(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
