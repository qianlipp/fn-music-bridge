package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type responseCommittedError struct {
	err error
}

func (e *responseCommittedError) Error() string { return e.err.Error() }
func (e *responseCommittedError) Unwrap() error { return e.err }

type Server struct {
	config           Config
	api              *apiClient
	auth             *authenticator
	settings         *settingsStore
	protocolSessions *protocolSessionStore
	logger           *log.Logger
	ui               http.Handler
	settingsApplyMu  sync.Mutex

	trackMu    sync.RWMutex
	trackCache map[string]map[string]any
	coverMu    sync.RWMutex
	coverCache map[string]string

	ampacheMu       sync.RWMutex
	ampacheSessions map[string]ampacheSession

	synologyMu       sync.RWMutex
	synologySessions map[string]authSession

	jellyfinMu       sync.RWMutex
	jellyfinSessions map[string]authSession
	jellyfinImages   map[string]string
}

func NewServer(config Config, logger *log.Logger) (*Server, error) {
	if logger == nil {
		logger = log.Default()
	}
	vault, err := openCredentialVault(config.CredentialFile, config.CredentialKey)
	if err != nil {
		return nil, fmt.Errorf("open credential vault: %w", err)
	}
	settingsPath := strings.TrimSpace(config.SettingsFile)
	if settingsPath == "" {
		settingsPath = filepath.Join(filepath.Dir(config.CredentialFile), "yinqiao-settings.json")
	}
	settings, err := openSettingsStore(settingsPath, defaultServiceSettings(config))
	if err != nil {
		return nil, fmt.Errorf("open service settings: %w", err)
	}
	protocolSessions, err := openProtocolSessionStore(filepath.Join(filepath.Dir(config.CredentialFile), "yinqiao-sessions.enc"), vault.key)
	if err != nil {
		return nil, fmt.Errorf("open protocol sessions: %w", err)
	}
	api := &apiClient{
		baseURL:      strings.TrimRight(config.APIBaseURL, "/"),
		client:       &http.Client{Timeout: config.RequestTimeout},
		streamClient: streamHTTPClient(config.RequestTimeout),
	}
	return &Server{
		config:           config,
		api:              api,
		auth:             &authenticator{vault: vault, api: api, sessions: map[string]authSession{}},
		settings:         settings,
		protocolSessions: protocolSessions,
		logger:           logger,
		ui:               newDashboardHandler(),
		trackCache:       map[string]map[string]any{},
		coverCache:       map[string]string{},
		ampacheSessions:  map[string]ampacheSession{},
		synologySessions: map[string]authSession{},
		jellyfinSessions: map[string]authSession{},
		jellyfinImages:   map[string]string{},
	}, nil
}

func (s *Server) Handler() http.Handler { return s }

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if isSensitiveManagementMutation(request) && !isSameOriginRequest(request) {
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"ok": false, "code": "CROSS_ORIGIN_BLOCKED", "error": "cross-origin management requests are not allowed",
		})
		return
	}
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Authorization, X-Emby-Authorization, X-Emby-Token, X-MediaBrowser-Token")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if request.URL.Path == "/yinqiao/ui" {
		http.Redirect(writer, request, "/yinqiao/ui/", http.StatusTemporaryRedirect)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/yinqiao/ui/") {
		s.ui.ServeHTTP(writer, request)
		return
	}

	if request.URL.Path == "/" || request.URL.Path == "/yinqiao" || request.URL.Path == "/yinqiao/" {
		s.writeLanding(writer)
		return
	}
	if request.URL.Path == "/yinqiao/enroll" {
		s.handleEnrollment(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/status" {
		s.handleCompatibilityStatus(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/network" {
		s.handleNetworkInfo(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/settings" {
		s.handleSettings(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/accounts" {
		s.handleAccounts(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/capabilities" {
		s.handleProtocolCapabilities(writer, request)
		return
	}
	if request.URL.Path == "/yinqiao/api-capabilities" {
		s.handleAPICapabilities(writer, request)
		return
	}
	if s.protocolEnabled("synology") && isSynologyPath(request.URL.Path) {
		s.handleSynology(writer, request)
		return
	}
	if s.protocolEnabled("jellyfin") && isJellyfinPath(request.URL.Path) {
		s.handleJellyfin(writer, request)
		return
	}
	if s.protocolEnabled("ampache") && isAmpachePath(request.URL.Path) {
		s.handleAmpache(writer, request)
		return
	}
	if !s.protocolEnabled("openSubsonic") || !strings.HasPrefix(request.URL.Path, "/rest/") {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		s.writeError(writer, request, http.StatusMethodNotAllowed, 0, "Method not allowed")
		return
	}
	if err := request.ParseForm(); err != nil {
		s.writeError(writer, request, http.StatusOK, 10, "Invalid request parameters")
		return
	}
	method := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/rest/"), ".view")
	if method == "" || strings.Contains(method, "/") {
		s.writeError(writer, request, http.StatusOK, 70, "Requested data was not found")
		return
	}
	if method == "getOpenSubsonicExtensions" {
		s.writeOK(writer, request, node{
			Name: "openSubsonicExtensions",
			DirectJSON: []any{
				map[string]any{"name": "songLyrics", "versions": []int{1}},
				map[string]any{"name": "formPost", "versions": []int{1}},
			},
			Children: []node{n("openSubsonicExtension", map[string]any{
				"name": "songLyrics", "versions": "1",
			}), n("openSubsonicExtension", map[string]any{"name": "formPost", "versions": "1"})},
		})
		return
	}

	session, err := s.auth.authenticate(request.Context(), request.Form)
	if err != nil {
		s.writeProtocolError(writer, request, err)
		return
	}
	if err := s.dispatch(writer, request, method, session); err != nil {
		s.logger.Printf(
			"method=%s user=%q client=%q id=%q query=%q error=%v",
			method, session.Username, request.Form.Get("c"), request.Form.Get("id"), request.Form.Get("query"), err,
		)
		var committedErr *responseCommittedError
		if errors.As(err, &committedErr) {
			return
		}
		s.writeProtocolError(writer, request, err)
	}
}

func isSensitiveManagementMutation(request *http.Request) bool {
	if request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodDelete {
		return false
	}
	switch request.URL.Path {
	case "/yinqiao/enroll", "/yinqiao/accounts", "/yinqiao/settings":
		return true
	default:
		return false
	}
}

func isSameOriginRequest(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		// Native clients and command-line tools do not send Origin. Browser requests do.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}

func (s *Server) dispatch(writer http.ResponseWriter, request *http.Request, method string, session authSession) error {
	switch method {
	case "ping":
		s.writeOK(writer, request)
	case "getLicense":
		s.writeOK(writer, request, n("license", map[string]any{"valid": true}))
	case "getMusicFolders":
		s.writeOK(writer, request, node{Name: "musicFolders", Children: []node{n("musicFolder", map[string]any{"id": "fnmusic", "name": "fnmusic"})}, ArrayChildren: map[string]bool{"musicFolder": true}})
	case "getUser":
		username := request.Form.Get("username")
		if username != "" && subtle.ConstantTimeCompare([]byte(normalizeUsername(username)), []byte(normalizeUsername(session.Username))) != 1 {
			return &authFailure{Code: 50, Message: "User is not authorized for the given operation"}
		}
		role := strings.ToLower(stringValue(session.User, "role"))
		admin := role == "admin" || role == "administrator"
		s.writeOK(writer, request, n("user", map[string]any{
			"username": session.Username, "email": "", "scrobblingEnabled": true,
			"adminRole": admin, "settingsRole": admin, "downloadRole": true,
			"uploadRole": false, "playlistRole": true, "coverArtRole": true,
			"commentRole": false, "podcastRole": false, "streamRole": true,
			"jukeboxRole": false, "shareRole": false, "videoConversionRole": false,
		}))
	case "getArtists", "getIndexes":
		return s.handleArtists(request.Context(), writer, request, session, method)
	case "getArtist":
		return s.handleArtist(request.Context(), writer, request, session)
	case "getAlbum":
		return s.handleAlbum(request.Context(), writer, request, session)
	case "getSong":
		return s.handleSong(request.Context(), writer, request, session)
	case "getMusicDirectory":
		return s.handleMusicDirectory(request.Context(), writer, request, session)
	case "getGenres":
		return s.handleGenres(request.Context(), writer, request, session)
	case "getAlbumList", "getAlbumList2":
		return s.handleAlbumList(request.Context(), writer, request, session)
	case "getRandomSongs":
		return s.handleRandomSongs(request.Context(), writer, request, session)
	case "getSongsByGenre":
		return s.handleSongsByGenre(request.Context(), writer, request, session)
	case "getStarred", "getStarred2":
		return s.handleStarred(request.Context(), writer, request, session)
	case "search2", "search3":
		return s.handleSearch(request.Context(), writer, request, session, method)
	case "getPlaylists":
		return s.handlePlaylists(request.Context(), writer, request, session)
	case "getPlaylist":
		return s.handlePlaylist(request.Context(), writer, request, session)
	case "createPlaylist":
		return s.handleCreatePlaylist(request.Context(), writer, request, session)
	case "updatePlaylist":
		return s.handleUpdatePlaylist(request.Context(), writer, request, session)
	case "deletePlaylist":
		return s.mutate(request.Context(), writer, request, session, "/me/playlists/delete", map[string]any{"guid": required(request.Form, "id")})
	case "star", "unstar":
		return s.handleStar(request.Context(), writer, request, session, method == "star")
	case "scrobble":
		return s.handleScrobble(request.Context(), writer, request, session)
	case "stream", "download":
		return s.proxyBinary(writer, request, session, "/media/tracks/"+url.PathEscape(required(request.Form, "id"))+"/stream", nil)
	case "getCoverArt":
		return s.handleCoverArt(writer, request, session)
	case "getLyrics":
		return s.handleLegacyLyrics(request.Context(), writer, request, session)
	case "getLyricsBySongId":
		return s.handleLyricsBySongID(request.Context(), writer, request, session)
	case "getScanStatus":
		return s.handleScanStatus(request.Context(), writer, request, session)
	case "startScan":
		return s.mutateWithPayload(request.Context(), writer, request, session, "/admin/libraries/scan-all", map[string]any{}, "scanStatus")
	case "getNowPlaying", "getBookmarks", "getInternetRadioStations", "getShares", "getVideos", "getPodcasts", "getNewestPodcasts", "getPlayQueue", "getChatMessages", "getSimilarSongs", "getSimilarSongs2", "getTopSongs", "getArtistInfo", "getArtistInfo2", "getAlbumInfo", "getAlbumInfo2":
		s.writeOK(writer, request, emptyCollection(method))
	case "setRating", "savePlayQueue", "createBookmark", "deleteBookmark":
		s.writeOK(writer, request)
	default:
		return &authFailure{Code: 70, Message: "Subsonic method is not implemented: " + method}
	}
	return nil
}

func (s *Server) handleArtists(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession, method string) error {
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/artists/all", nil, nil)
	if err != nil {
		return err
	}
	artists := list(data)
	groups := map[string][]node{}
	for _, item := range artists {
		artist := object(item)
		name := stringValue(artist, "name", "title")
		letter := "#"
		if name != "" {
			letter = strings.ToUpper(string([]rune(name)[0]))
		}
		groups[letter] = append(groups[letter], artistNode(artist))
	}
	letters := make([]string, 0, len(groups))
	for letter := range groups {
		letters = append(letters, letter)
	}
	sortStrings(letters)
	var indexes []node
	for _, letter := range letters {
		indexes = append(indexes, node{Name: "index", Attr: map[string]any{"name": letter}, Children: groups[letter], ArrayChildren: map[string]bool{"artist": true}})
	}
	rootName := "artists"
	if method == "getIndexes" {
		rootName = "indexes"
	}
	s.writeOK(writer, request, node{Name: rootName, Children: indexes, ArrayChildren: map[string]bool{"index": true}})
	return nil
}

func (s *Server) handleArtist(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	artistData, session, err := s.call(ctx, session, http.MethodGet, "/catalog/artists/detail", url.Values{"guid": {id}}, nil)
	if err != nil {
		return err
	}
	albumsData, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums/by-artist", pageQuery(request.Form, map[string]string{"artistGUID": id}), nil)
	if err != nil {
		return err
	}
	artist := artistNode(object(artistData))
	artist.Name = "artist"
	for _, item := range list(albumsData) {
		artist.Children = append(artist.Children, albumNode(object(item)))
	}
	artist.ArrayChildren = map[string]bool{"album": true}
	s.writeOK(writer, request, artist)
	return nil
}

func (s *Server) handleAlbum(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	albumData, session, err := s.call(ctx, session, http.MethodGet, "/catalog/albums/detail", url.Values{"guid": {id}}, nil)
	if err != nil {
		return err
	}
	tracksData, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-album", url.Values{
		"albumGUID": {id}, "page": {"1"}, "size": {"10000"}, "sort": {"discNo,asc"},
	}, nil)
	if err != nil {
		return err
	}
	album := albumNode(object(albumData))
	for _, item := range list(tracksData) {
		track := object(item)
		s.rememberTrack(track)
		album.Children = append(album.Children, trackNode(track))
	}
	album.ArrayChildren = map[string]bool{"song": true}
	s.writeOK(writer, request, album)
	return nil
}

func (s *Server) handleSong(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	track, _, err := s.findTrack(ctx, session, required(request.Form, "id"))
	if err != nil {
		return err
	}
	s.writeOK(writer, request, trackNode(track))
	return nil
}

func (s *Server) handleMusicDirectory(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	if id == "fnmusic" {
		data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums", url.Values{"page": {"1"}, "size": {"500"}}, nil)
		if err != nil {
			return err
		}
		root := node{Name: "directory", Attr: map[string]any{"id": id, "name": "fnmusic"}, ArrayChildren: map[string]bool{"child": true}}
		for _, item := range list(data) {
			album := albumNode(object(item))
			album.Name = "child"
			album.Attr["isDir"] = true
			title := album.Attr["name"]
			album.Attr["title"] = title
			root.Children = append(root.Children, album)
		}
		s.writeOK(writer, request, root)
		return nil
	}
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-album", url.Values{"albumGUID": {id}, "page": {"1"}, "size": {"10000"}}, nil)
	if err != nil {
		return err
	}
	root := node{Name: "directory", Attr: map[string]any{"id": id, "name": id}, ArrayChildren: map[string]bool{"child": true}}
	for _, item := range list(data) {
		track := object(item)
		s.rememberTrack(track)
		child := trackNode(track)
		child.Name = "child"
		root.Children = append(root.Children, child)
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleGenres(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/genres", url.Values{"page": {"1"}, "size": {"1000"}}, nil)
	if err != nil {
		return err
	}
	root := node{Name: "genres", ArrayChildren: map[string]bool{"genre": true}}
	for _, item := range list(data) {
		genre := object(item)
		genreNode := n("genre", cleanAttr(map[string]any{
			"songCount":  intValue(genre, "trackCount"),
			"albumCount": intValue(genre, "albumCount"), "id": stringValue(genre, "guid", "id"),
		}))
		genreNode.Value = stringValue(genre, "name", "title")
		root.Children = append(root.Children, genreNode)
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleAlbumList(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	size := boundedInt(request.Form.Get("size"), 10, 1, 500)
	offset := boundedInt(request.Form.Get("offset"), 0, 0, 1_000_000)
	typeName := request.Form.Get("type")
	sortValue := "createdAt,desc"
	switch typeName {
	case "alphabeticalByName":
		sortValue = "name,asc"
	case "alphabeticalByArtist":
		sortValue = "artist,asc"
	case "newest", "recent", "frequent", "highest", "starred", "random", "":
	default:
		return &authFailure{Code: 10, Message: "Unsupported album list type: " + typeName}
	}
	query := url.Values{"page": {strconv.Itoa(offset/size + 1)}, "size": {strconv.Itoa(size)}, "sort": {sortValue}}
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/albums", query, nil)
	if err != nil {
		return err
	}
	root := node{Name: "albumList2", ArrayChildren: map[string]bool{"album": true}}
	if strings.HasSuffix(strings.TrimSuffix(request.URL.Path, ".view"), "getAlbumList") {
		root.Name = "albumList"
	}
	for _, item := range list(data) {
		root.Children = append(root.Children, albumNode(object(item)))
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleRandomSongs(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	size := boundedInt(request.Form.Get("size"), 10, 1, 500)
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks", url.Values{"page": {"1"}, "size": {strconv.Itoa(max(size*4, 100))}}, nil)
	if err != nil {
		return err
	}
	items := list(data)
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	root := node{Name: "randomSongs", ArrayChildren: map[string]bool{"song": true}}
	for index, item := range items {
		if index >= size {
			break
		}
		track := object(item)
		s.rememberTrack(track)
		root.Children = append(root.Children, trackNode(track))
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleSongsByGenre(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	genre := required(request.Form, "genre")
	genreID := genre
	genres, updated, err := s.call(ctx, session, http.MethodGet, "/catalog/genres", url.Values{"page": {"1"}, "size": {"1000"}}, nil)
	if err == nil {
		session = updated
		for _, item := range list(genres) {
			candidate := object(item)
			if strings.EqualFold(stringValue(candidate, "name", "title"), genre) {
				genreID = stringValue(candidate, "guid", "id")
				break
			}
		}
	}
	query := pageQuery(request.Form, map[string]string{"genreGUID": genreID})
	data, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-genre", query, nil)
	if err != nil {
		return err
	}
	root := node{Name: "songsByGenre", ArrayChildren: map[string]bool{"song": true}}
	for _, item := range list(data) {
		track := object(item)
		s.rememberTrack(track)
		root.Children = append(root.Children, trackNode(track))
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleStarred(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(ctx, session, http.MethodGet, "/me/favorites", pageQuery(request.Form, nil), nil)
	if err != nil {
		return err
	}
	name := "starred2"
	if strings.Contains(request.URL.Path, "getStarred.view") || strings.HasSuffix(request.URL.Path, "/getStarred") {
		name = "starred"
	}
	root := node{Name: name, ArrayChildren: map[string]bool{"song": true, "album": true, "artist": true}}
	for _, item := range list(data) {
		track := object(item)
		s.rememberTrack(track)
		root.Children = append(root.Children, trackNode(track))
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleSearch(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession, method string) error {
	queryText := strings.TrimSpace(request.Form.Get("query"))
	rootName := "searchResult3"
	if method == "search2" {
		rootName = "searchResult2"
	}
	root := node{Name: rootName, ArrayChildren: map[string]bool{"song": true, "album": true, "artist": true}}
	types := []struct {
		path, countKey, offsetKey string
		mapNode                   func(map[string]any) node
	}{
		{"/search/artists", "artistCount", "artistOffset", artistNode},
		{"/search/albums", "albumCount", "albumOffset", albumNode},
		{"/search/tracks", "songCount", "songOffset", trackNode},
	}
	current := session
	for _, itemType := range types {
		count := boundedInt(request.Form.Get(itemType.countKey), 20, 0, 500)
		if count == 0 {
			continue
		}
		offset := boundedInt(request.Form.Get(itemType.offsetKey), 0, 0, 1_000_000)
		data, updated, err := s.searchItems(ctx, current, itemType.path, queryText, offset, count)
		if err != nil {
			return err
		}
		current = updated
		for _, value := range list(data) {
			entry := object(value)
			if itemType.path == "/search/tracks" {
				s.rememberTrack(entry)
			}
			root.Children = append(root.Children, itemType.mapNode(entry))
		}
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) searchItems(ctx context.Context, session authSession, searchPath, queryText string, offset, count int) (any, authSession, error) {
	if queryText != "" && queryText != "*" {
		query := url.Values{"q": {queryText}, "page": {strconv.Itoa(offset/count + 1)}, "size": {strconv.Itoa(count)}}
		data, updated, err := s.call(ctx, session, http.MethodGet, searchPath, query, nil)
		if err == nil {
			return boundedSearchData(data, queryText, offset, count), updated, nil
		}
		if !isInvalidArguments(err) {
			return nil, updated, err
		}
		session = updated
	}

	browsePath := map[string]string{
		"/search/artists": "/catalog/artists/all",
		"/search/albums":  "/catalog/albums",
		"/search/tracks":  "/catalog/tracks",
	}[searchPath]
	if browsePath == "" {
		return map[string]any{"list": []any{}, "total": 0}, session, nil
	}
	if browsePath == "/catalog/artists/all" {
		data, updated, err := s.call(ctx, session, http.MethodGet, browsePath, nil, nil)
		if err != nil {
			return nil, updated, err
		}
		return pagedFilteredData(list(data), queryText, offset, count), updated, nil
	}
	if queryText == "" || queryText == "*" {
		query := url.Values{"page": {strconv.Itoa(offset/count + 1)}, "size": {strconv.Itoa(count)}}
		data, updated, err := s.call(ctx, session, http.MethodGet, browsePath, query, nil)
		if err != nil {
			return nil, updated, err
		}
		return boundedSearchData(data, "", offset, count), updated, nil
	}
	data, updated, err := s.call(ctx, session, http.MethodGet, browsePath, url.Values{"page": {"1"}, "size": {"5000"}}, nil)
	if err != nil {
		return nil, updated, err
	}
	return pagedFilteredData(list(data), queryText, offset, count), updated, nil
}

func boundedSearchData(data any, queryText string, offset, count int) map[string]any {
	values := list(data)
	queryText = strings.ToLower(strings.TrimSpace(queryText))
	filtered := make([]any, 0, len(values))
	for _, value := range values {
		if queryText != "" && queryText != "*" && !strings.Contains(strings.ToLower(searchableText(object(value))), queryText) {
			continue
		}
		filtered = append(filtered, value)
	}
	total := int(intValue(object(data), "total", "TotalRecordCount", "count"))
	if total == 0 && len(filtered) > 0 {
		total = len(filtered)
	}
	// fnmusic search currently returns the complete match set even when page
	// and size are supplied. Browse endpoints return an actual page. Apply the
	// offset only when the response is demonstrably the complete collection.
	if len(values) > count || (total > 0 && total == len(values)) {
		total = len(filtered)
		start := min(offset, len(filtered))
		filtered = filtered[start:min(start+count, len(filtered))]
	} else if len(filtered) > count {
		filtered = filtered[:count]
	}
	return map[string]any{"list": filtered, "total": total}
}

func pagedFilteredData(values []any, queryText string, offset, count int) map[string]any {
	queryText = strings.ToLower(strings.TrimSpace(queryText))
	filtered := make([]any, 0, len(values))
	for _, value := range values {
		entry := object(value)
		if queryText == "" || queryText == "*" || strings.Contains(strings.ToLower(searchableText(entry)), queryText) {
			filtered = append(filtered, value)
		}
	}
	start := min(offset, len(filtered))
	end := min(start+count, len(filtered))
	return map[string]any{"list": filtered[start:end], "total": len(filtered)}
}

func searchableText(entry map[string]any) string {
	parts := []string{stringValue(entry, "title", "name")}
	parts = append(parts, joinedNames(nestedList(entry, "artists")))
	if album := firstObject(entry, "album"); album != nil {
		parts = append(parts, stringValue(album, "name", "title"))
	}
	return strings.Join(parts, " ")
}

func isInvalidArguments(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && apiErr.Code == "UPSTREAM_100002"
}

func (s *Server) handlePlaylists(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(ctx, session, http.MethodGet, "/me/playlists", nil, nil)
	if err != nil {
		return err
	}
	root := node{Name: "playlists", ArrayChildren: map[string]bool{"playlist": true}}
	for _, item := range list(data) {
		root.Children = append(root.Children, playlistNode(object(item), session.Username))
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handlePlaylist(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	data, session, err := s.call(ctx, session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {id}}, nil)
	if err != nil {
		return err
	}
	tracks, _, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {id}, "page": {"1"}, "size": {"10000"}}, nil)
	if err != nil {
		return err
	}
	root := playlistNode(object(data), session.Username)
	for _, item := range list(tracks) {
		track := object(item)
		s.rememberTrack(track)
		entry := trackNode(track)
		entry.Name = "entry"
		root.Children = append(root.Children, entry)
	}
	root.ArrayChildren = map[string]bool{"entry": true}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleCreatePlaylist(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	name := request.Form.Get("name")
	playlistID := request.Form.Get("playlistId")
	var data any
	var err error
	if playlistID == "" {
		if name == "" {
			return &authFailure{Code: 10, Message: "Required parameter 'name' is missing"}
		}
		coverID, updated := s.playlistCoverID(ctx, session, request.Form["songId"])
		session = updated
		data, session, err = s.call(ctx, session, http.MethodPost, "/me/playlists", nil, map[string]any{"name": name, "coverId": nullableString(coverID)})
	} else {
		data, session, err = s.call(ctx, session, http.MethodGet, "/me/playlists/detail", url.Values{"guid": {playlistID}}, nil)
	}
	if err != nil {
		return err
	}
	id := stringValue(object(data), "guid", "id")
	ids := request.Form["songId"]
	if len(ids) > 0 {
		_, session, err = s.call(ctx, session, http.MethodPost, "/me/playlists/tracks/add", nil, map[string]any{"guid": id, "trackGUIDs": ids})
		if err != nil {
			return err
		}
	}
	request.Form.Set("id", id)
	return s.handlePlaylist(ctx, writer, request, session)
}

func (s *Server) playlistCoverID(ctx context.Context, session authSession, songIDs []string) (string, authSession) {
	for _, id := range songIDs {
		track, updated, err := s.findTrack(ctx, session, id)
		session = updated
		if err == nil {
			if coverID := extractCoverID(track); coverID != "" {
				return coverID, session
			}
			if coverID := extractCoverID(firstObject(track, "album")); coverID != "" {
				return coverID, session
			}
		}
	}
	data, updated, err := s.call(ctx, session, http.MethodGet, "/catalog/albums", url.Values{"page": {"1"}, "size": {"20"}}, nil)
	if err != nil {
		return "", updated
	}
	for _, item := range list(data) {
		if coverID := extractCoverID(object(item)); coverID != "" {
			return coverID, updated
		}
	}
	return "", updated
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *Server) handleUpdatePlaylist(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "playlistId")
	if name := request.Form.Get("name"); name != "" {
		_, updated, err := s.call(ctx, session, http.MethodPost, "/me/playlists/update", nil, map[string]any{"guid": id, "name": name})
		if err != nil {
			return err
		}
		session = updated
	}
	if ids := request.Form["songIdToAdd"]; len(ids) > 0 {
		_, updated, err := s.call(ctx, session, http.MethodPost, "/me/playlists/tracks/add", nil, map[string]any{"guid": id, "trackGUIDs": ids})
		if err != nil {
			return err
		}
		session = updated
	}
	if indexes := request.Form["songIndexToRemove"]; len(indexes) > 0 {
		tracks, updated, err := s.call(ctx, session, http.MethodGet, "/catalog/tracks/by-playlist", url.Values{"playlistGUID": {id}, "page": {"1"}, "size": {"10000"}}, nil)
		if err != nil {
			return err
		}
		session = updated
		items := list(tracks)
		var ids []string
		for _, rawIndex := range indexes {
			index, parseErr := strconv.Atoi(rawIndex)
			if parseErr == nil && index >= 0 && index < len(items) {
				ids = append(ids, stringValue(object(items[index]), "guid", "id"))
			}
		}
		if len(ids) > 0 {
			_, session, err = s.call(ctx, session, http.MethodPost, "/me/playlists/tracks/remove", nil, map[string]any{"guid": id, "trackGUIDs": ids})
			if err != nil {
				return err
			}
		}
	}
	s.writeOK(writer, request)
	return nil
}

func (s *Server) handleStar(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession, add bool) error {
	endpoint := "/me/favorites"
	if !add {
		endpoint += "/remove"
	}
	for _, id := range request.Form["id"] {
		var err error
		_, session, err = s.call(ctx, session, http.MethodPost, endpoint, nil, map[string]any{"trackGUID": id})
		if err != nil {
			return err
		}
	}
	s.writeOK(writer, request)
	return nil
}

func (s *Server) handleScrobble(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	ids := request.Form["id"]
	times := request.Form["time"]
	now := time.Now().UnixMilli()
	events := make([]map[string]any, 0, len(ids))
	for index, id := range ids {
		occurredAt := now
		if index < len(times) {
			if parsed, err := strconv.ParseInt(times[index], 10, 64); err == nil {
				occurredAt = parsed
			}
		}
		events = append(events, map[string]any{"eventType": "track_play", "occurredAt": occurredAt, "payload": map[string]any{"trackGUID": id}})
	}
	if len(events) > 0 {
		if _, _, err := s.call(ctx, session, http.MethodPost, "/activity/events", nil, map[string]any{"events": events}); err != nil {
			return err
		}
	}
	s.writeOK(writer, request)
	return nil
}

func (s *Server) handleLegacyLyrics(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	title := strings.TrimSpace(request.Form.Get("title"))
	artist := strings.TrimSpace(request.Form.Get("artist"))
	query := strings.TrimSpace(title + " " + artist)
	data, session, err := s.searchItems(ctx, session, "/search/tracks", query, 0, 20)
	if err != nil {
		return err
	}
	items := list(data)
	track := bestLyricTrack(items, title, artist)
	if track == nil {
		s.writeOK(writer, request, n("lyrics", map[string]any{"artist": request.Form.Get("artist"), "title": request.Form.Get("title")}))
		return nil
	}
	lyrics, _, err := s.lyricText(ctx, session, stringValue(track, "guid", "id"))
	if err != nil {
		return err
	}
	root := n("lyrics", map[string]any{"artist": request.Form.Get("artist"), "title": request.Form.Get("title")})
	root.Value = lyrics
	s.writeOK(writer, request, root)
	return nil
}

func bestLyricTrack(items []any, title, artist string) map[string]any {
	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	normalizedArtist := strings.ToLower(strings.TrimSpace(artist))
	var titleMatch map[string]any
	for _, item := range items {
		track := object(item)
		candidateTitle := strings.ToLower(strings.TrimSpace(stringValue(track, "title", "name")))
		if normalizedTitle != "" && candidateTitle != normalizedTitle {
			continue
		}
		if titleMatch == nil {
			titleMatch = track
		}
		if normalizedArtist == "" || strings.Contains(strings.ToLower(joinedNames(nestedList(track, "artists"))), normalizedArtist) {
			return track
		}
	}
	return titleMatch
}

func (s *Server) handleLyricsBySongID(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	lyrics, _, err := s.lyricText(ctx, session, id)
	if err != nil {
		return err
	}
	lines, synced := parseLyricLines(lyrics)
	structured := node{Name: "structuredLyrics", Attr: map[string]any{"displayArtist": "", "displayTitle": "", "lang": "und", "synced": synced}, Children: lines, ArrayChildren: map[string]bool{"line": true}}
	root := node{Name: "lyricsList", Children: []node{structured}, ArrayChildren: map[string]bool{"structuredLyrics": true}}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) handleScanStatus(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession) error {
	data, _, err := s.call(ctx, session, http.MethodGet, "/admin/tasks", nil, nil)
	if err != nil {
		return err
	}
	scanning := false
	count := int64(0)
	for _, item := range list(data) {
		task := object(item)
		if !boolValue(task, "done") && !boolValue(task, "canceled") {
			scanning = true
			count += intValue(task, "successCount")
		}
	}
	s.writeOK(writer, request, n("scanStatus", map[string]any{"scanning": scanning, "count": count}))
	return nil
}

func (s *Server) lyricText(ctx context.Context, session authSession, id string) (string, authSession, error) {
	data, updated, err := s.call(ctx, session, http.MethodGet, "/catalog/lyrics", url.Values{"trackGUID": {id}}, nil)
	if err != nil {
		return "", updated, err
	}
	items := list(data)
	preferred := stringValue(object(data), "preferred")
	for _, item := range items {
		entry := object(item)
		if preferred == "" || stringValue(entry, "guid", "id") == preferred {
			return stringValue(entry, "content", "text"), updated, nil
		}
	}
	return "", updated, nil
}

func (s *Server) mutate(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession, endpoint string, body map[string]any) error {
	for key, value := range body {
		if value == "" {
			return &authFailure{Code: 10, Message: "Required parameter '" + key + "' is missing"}
		}
	}
	_, _, err := s.call(ctx, session, http.MethodPost, endpoint, nil, body)
	if err != nil {
		return err
	}
	s.writeOK(writer, request)
	return nil
}

func (s *Server) mutateWithPayload(ctx context.Context, writer http.ResponseWriter, request *http.Request, session authSession, endpoint string, body map[string]any, responseName string) error {
	data, _, err := s.call(ctx, session, http.MethodPost, endpoint, nil, body)
	if err != nil {
		return err
	}
	root := n(responseName, map[string]any{"scanning": true})
	if value := object(data); value != nil {
		for key, item := range value {
			root.Attr[key] = item
		}
	}
	s.writeOK(writer, request, root)
	return nil
}

func (s *Server) call(ctx context.Context, session authSession, method, endpoint string, query url.Values, body any) (any, authSession, error) {
	data, err := s.api.call(ctx, method, endpoint, session.Token, query, body)
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		refreshed, refreshErr := s.auth.refresh(ctx, session)
		if refreshErr != nil {
			return nil, session, refreshErr
		}
		session = refreshed
		data, err = s.api.call(ctx, method, endpoint, session.Token, query, body)
	}
	return data, session, err
}

func (s *Server) proxyBinary(writer http.ResponseWriter, request *http.Request, session authSession, endpoint string, query url.Values) error {
	response, err := s.api.raw(request.Context(), endpoint, session.Token, query, request.Header)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusUnauthorized {
		_ = response.Body.Close()
		refreshed, refreshErr := s.auth.refresh(request.Context(), session)
		if refreshErr != nil {
			return refreshErr
		}
		response, err = s.api.raw(request.Context(), endpoint, refreshed.Token, query, request.Header)
		if err != nil {
			return err
		}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		return &apiError{Status: response.StatusCode, Code: "MEDIA_ERROR", Message: strings.TrimSpace(string(payload))}
	}
	for _, header := range []string{"Accept-Ranges", "Cache-Control", "Content-Disposition", "Content-Length", "Content-Range", "Content-Type", "ETag", "Last-Modified"} {
		if value := response.Header.Get(header); value != "" {
			writer.Header().Set(header, value)
		}
	}
	writer.WriteHeader(response.StatusCode)
	_, err = io.Copy(writer, response.Body)
	if err == nil || isClientDisconnect(request.Context(), err) {
		return nil
	}
	return &responseCommittedError{err: err}
}

func isClientDisconnect(ctx context.Context, err error) bool {
	return ctx.Err() != nil ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

func (s *Server) handleCoverArt(writer http.ResponseWriter, request *http.Request, session authSession) error {
	id := required(request.Form, "id")
	if id == "" {
		return &authFailure{Code: 10, Message: "Required parameter 'id' is missing"}
	}
	query := url.Values{}
	if size := request.Form.Get("size"); size != "" {
		query.Set("size", size)
	}

	s.coverMu.RLock()
	coverID := s.coverCache[id]
	s.coverMu.RUnlock()
	if coverID != "" {
		return s.proxyBinary(writer, request, session, "/media/covers/"+url.PathEscape(coverID), query)
	}
	if strings.HasPrefix(strings.ToLower(id), "al-") {
		resolved, updated := s.resolveCoverID(request.Context(), session, id)
		if resolved != "" && resolved != id {
			s.rememberCover(id, resolved)
			return s.proxyBinary(writer, request, updated, "/media/covers/"+url.PathEscape(resolved), query)
		}
	}

	directErr := s.proxyBinary(writer, request, session, "/media/covers/"+url.PathEscape(id), query)
	if directErr == nil {
		s.rememberCover(id, id)
		return nil
	}
	var apiErr *apiError
	if !errors.As(directErr, &apiErr) || (apiErr.Status != http.StatusBadRequest && apiErr.Status != http.StatusNotFound) {
		return directErr
	}

	resolved, updated := s.resolveCoverID(request.Context(), session, id)
	if resolved == "" || resolved == id {
		return directErr
	}
	s.rememberCover(id, resolved)
	return s.proxyBinary(writer, request, updated, "/media/covers/"+url.PathEscape(resolved), query)
}

func (s *Server) resolveCoverID(ctx context.Context, session authSession, id string) (string, authSession) {
	detailRequests := []struct {
		endpoint      string
		query         url.Values
		trackEndpoint string
		trackKey      string
	}{
		{"/catalog/albums/detail", url.Values{"guid": {id}}, "/catalog/tracks/by-album", "albumGUID"},
		{"/catalog/artists/detail", url.Values{"guid": {id}}, "/catalog/tracks/by-artist", "artistGUID"},
		{"/me/playlists/detail", url.Values{"guid": {id}}, "/catalog/tracks/by-playlist", "playlistGUID"},
	}
	for _, detail := range detailRequests {
		data, updated, err := s.call(ctx, session, http.MethodGet, detail.endpoint, detail.query, nil)
		if err == nil {
			session = updated
			if coverID := extractCoverID(object(data)); coverID != "" {
				return coverID, session
			}
			coverID, next, relationErr := s.firstRelatedTrackCoverID(ctx, session, detail.trackEndpoint, detail.trackKey, id)
			session = next
			if relationErr == nil && coverID != "" {
				return coverID, session
			}
		}
	}
	track, updated, err := s.findTrack(ctx, session, id)
	if err == nil {
		if coverID := extractCoverID(track); coverID != "" {
			return coverID, updated
		}
		if album := firstObject(track, "album"); album != nil {
			return extractCoverID(album), updated
		}
	}
	return "", session
}

func (s *Server) firstRelatedTrackCoverID(ctx context.Context, session authSession, endpoint, relationKey, id string) (string, authSession, error) {
	data, updated, err := s.call(ctx, session, http.MethodGet, endpoint, url.Values{
		relationKey: {id}, "page": {"1"}, "size": {"1"},
	}, nil)
	if err != nil {
		return "", updated, err
	}
	items := list(data)
	if len(items) == 0 {
		return "", updated, nil
	}
	track := object(items[0])
	s.rememberTrack(track)
	if coverID := extractCoverID(track); coverID != "" {
		return coverID, updated, nil
	}
	return extractCoverID(firstObject(track, "album")), updated, nil
}

func (s *Server) findTrack(ctx context.Context, session authSession, id string) (map[string]any, authSession, error) {
	s.trackMu.RLock()
	cached := s.trackCache[id]
	s.trackMu.RUnlock()
	if cached != nil {
		return cached, session, nil
	}
	updated := session
	role := strings.ToLower(stringValue(session.User, "role"))
	if role == "admin" || role == "administrator" {
		data, next, err := s.call(ctx, session, http.MethodGet, "/admin/tracks/metadata", url.Values{"guid": {id}}, nil)
		if err == nil && entityID(object(data)) != "" {
			track := object(data)
			s.rememberTrack(track)
			return track, next, nil
		}
		updated = next
	}
	const pageSize = 500
	for page := 1; page <= 200; page++ {
		data, next, err := s.call(ctx, updated, http.MethodGet, "/catalog/tracks", url.Values{
			"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(pageSize)},
		}, nil)
		if err != nil {
			return nil, next, err
		}
		updated = next
		items := list(data)
		for _, item := range items {
			track := object(item)
			s.rememberTrack(track)
			if entityID(track) == id {
				return track, updated, nil
			}
		}
		total := intValue(object(data), "total")
		if len(items) == 0 || len(items) < pageSize || (total > 0 && int64(page*pageSize) >= total) {
			break
		}
	}
	return nil, updated, &authFailure{Code: 70, Message: "Song not found"}
}

func (s *Server) rememberTrack(track map[string]any) {
	id := entityID(track)
	if id == "" {
		return
	}
	s.trackMu.Lock()
	s.trackCache[id] = track
	s.trackMu.Unlock()
	trackCoverID := extractCoverID(track)
	if trackCoverID != "" {
		s.rememberCover(id, trackCoverID)
	}
	if album := firstObject(track, "album"); album != nil {
		albumID, coverID := entityID(album), extractCoverID(album)
		if coverID == "" {
			coverID = trackCoverID
		}
		if albumID != "" && coverID != "" {
			s.rememberCover(albumID, coverID)
		}
	}
}

func (s *Server) rememberCover(objectID, coverID string) {
	if objectID == "" || coverID == "" {
		return
	}
	s.coverMu.Lock()
	s.coverCache[objectID] = coverID
	s.coverCache[coverID] = coverID
	s.coverMu.Unlock()
}

func entityID(data map[string]any) string {
	return stringValue(data, "guid", "id", "trackGUID", "trackGuid", "songId")
}

func extractCoverID(data map[string]any) string {
	if data == nil {
		return ""
	}
	if id := stringValue(data, "coverId", "coverID", "coverGuid", "coverGUID"); id != "" {
		return id
	}
	if cover := firstObject(data, "cover"); cover != nil {
		return stringValue(cover, "coverId", "guid", "id")
	}
	return ""
}

func (s *Server) writeProtocolError(writer http.ResponseWriter, request *http.Request, err error) {
	var authErr *authFailure
	if errors.As(err, &authErr) {
		s.writeError(writer, request, http.StatusOK, authErr.Code, authErr.Message)
		return
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		code := 0
		switch apiErr.Status {
		case http.StatusUnauthorized:
			code = 40
		case http.StatusForbidden:
			code = 50
		case http.StatusNotFound:
			code = 70
		}
		s.writeError(writer, request, http.StatusOK, code, apiErr.Message)
		return
	}
	s.writeError(writer, request, http.StatusOK, 0, "fnmusic service error")
}

func (s *Server) handleEnrollment(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, `<!doctype html><html lang="zh-CN"><meta name="viewport" content="width=device-width"><title>音桥账号绑定</title><style>body{font:16px system-ui;max-width:420px;margin:64px auto;padding:20px}label{display:block;margin:16px 0 6px}input,button{box-sizing:border-box;width:100%;padding:12px;font:inherit}button{margin-top:20px}</style><h1>音桥账号绑定</h1><p>为 OpenSubsonic 或 Ampache 客户端绑定飞牛音乐账号。</p><form method="post"><label>飞牛音乐用户名</label><input name="username" autocomplete="username" required><label>飞牛音乐密码</label><input name="password" type="password" autocomplete="current-password" required><button>验证并绑定</button></form></html>`)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username, password := "", ""
	if strings.Contains(request.Header.Get("Content-Type"), "application/json") {
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(io.LimitReader(request.Body, 1<<20)).Decode(&input); err == nil {
			username, password = input.Username, input.Password
		}
	} else {
		_ = request.ParseForm()
		username, password = request.Form.Get("username"), request.Form.Get("password")
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "INVALID_INPUT", "error": "username and password are required"})
		return
	}
	if _, err := s.auth.loginAndRemember(request.Context(), username, password); err != nil {
		s.logger.Printf("credential enrollment user=%q error=%v", username, err)
		status, code, message := classifyEnrollmentError(err)
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": code, "error": message})
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{"ok": true, "username": username})
}

func (s *Server) handleCompatibilityStatus(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "METHOD_NOT_ALLOWED", "error": "method not allowed"})
		return
	}
	data, err := s.api.call(request.Context(), http.MethodGet, "/ready", "", nil, nil)
	if err != nil {
		status, code, message := classifyEnrollmentError(err)
		if status == http.StatusUnauthorized {
			status = http.StatusBadGateway
		}
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": code, "error": message})
		return
	}
	settings := s.settings.snapshot()
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": true, "version": serverVersion, "api": data,
		"boundUsers": s.auth.vault.usernames(),
		"settings":   settings,
		"protocols": map[string]any{
			"openSubsonic": map[string]any{"enabled": settings.Protocols.OpenSubsonic, "version": protocolVersion},
			"ampache":      map[string]any{"enabled": settings.Protocols.Ampache, "version": ampacheAPIVersion},
			"synology":     map[string]any{"enabled": settings.Protocols.Synology, "version": "DSM WebAPI / Audio Station"},
			"jellyfin":     map[string]any{"enabled": settings.Protocols.Jellyfin, "version": "10.10 audio-compatible"},
		},
	})
}

func classifyEnrollmentError(err error) (int, string, string) {
	var authErr *authFailure
	if errors.As(err, &authErr) {
		if authErr.UpstreamCode != "" {
			status := authErr.UpstreamStatus
			if status < 400 || status > 599 {
				status = http.StatusUnauthorized
			}
			message := authErr.UpstreamMessage
			if message == "" {
				message = "fnmusic rejected the username or password"
			}
			return status, authErr.UpstreamCode, message
		}
		return http.StatusUnauthorized, "INVALID_CREDENTIALS", "fnmusic username or password is incorrect, or the account is temporarily banned"
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		status := apiErr.Status
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		code := apiErr.Code
		if code == "" {
			code = "API_ERROR"
		}
		message := apiErr.Message
		if message == "" {
			message = "fnmusic API rejected the login request"
		}
		return status, code, message
	}
	if strings.Contains(err.Error(), "save protocol credential") || strings.Contains(err.Error(), "credential vault") {
		return http.StatusInternalServerError, "CREDENTIAL_STORE_ERROR", "fnmusic login succeeded but the encrypted credential could not be saved"
	}
	return http.StatusBadGateway, "API_UNAVAILABLE", "cannot reach the local fnmusic Yinqiao API on port 18090"
}

func (s *Server) writeLanding(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	settings := s.settings.snapshot()
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"name": "Yinqiao Service", "version": serverVersion,
		"protocols": map[string]any{
			"openSubsonic": map[string]any{"enabled": settings.Protocols.OpenSubsonic, "version": protocolVersion, "base": "/rest"},
			"ampache":      map[string]any{"enabled": settings.Protocols.Ampache, "version": ampacheAPIVersion, "xml": "/server/xml.server.php", "json": "/server/json.server.php"},
			"synology":     map[string]any{"enabled": settings.Protocols.Synology, "version": "DSM WebAPI / Audio Station", "base": "/webapi"},
			"jellyfin":     map[string]any{"enabled": settings.Protocols.Jellyfin, "version": "10.10 audio-compatible", "base": "/"},
		},
		"enrollment": "/yinqiao/enroll", "accounts": "/yinqiao/accounts", "settings": "/yinqiao/settings", "status": "/yinqiao/status", "network": "/yinqiao/network", "capabilities": "/yinqiao/capabilities",
		"apiCapabilities": "/yinqiao/api-capabilities",
	})
}

func required(values url.Values, key string) string {
	return strings.TrimSpace(values.Get(key))
}

func pageQuery(values url.Values, extra map[string]string) url.Values {
	size := boundedInt(values.Get("size"), 50, 1, 10000)
	offset := boundedInt(values.Get("offset"), 0, 0, 10_000_000)
	query := url.Values{"page": {strconv.Itoa(offset/size + 1)}, "size": {strconv.Itoa(size)}}
	for key, value := range extra {
		if value != "" {
			query.Set(key, value)
		}
	}
	return query
}

func boundedInt(raw string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func emptyCollection(method string) node {
	names := map[string]string{
		"getNowPlaying": "nowPlaying", "getBookmarks": "bookmarks", "getInternetRadioStations": "internetRadioStations",
		"getShares": "shares", "getVideos": "videos", "getPodcasts": "podcasts", "getPlayQueue": "playQueue",
		"getNewestPodcasts": "newestPodcasts", "getChatMessages": "chatMessages", "getSimilarSongs": "similarSongs",
		"getSimilarSongs2": "similarSongs2", "getTopSongs": "topSongs", "getArtistInfo": "artistInfo",
		"getArtistInfo2": "artistInfo2", "getAlbumInfo": "albumInfo", "getAlbumInfo2": "albumInfo2",
	}
	return node{Name: names[method]}
}

var lrcTimestamp = regexp.MustCompile(`\[(\d{1,3}):(\d{2})(?:[\.:](\d{1,3}))?\]`)

func parseLyricLines(content string) ([]node, bool) {
	var result []node
	synced := false
	for _, rawLine := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		matches := lrcTimestamp.FindAllStringSubmatch(rawLine, -1)
		textValue := strings.TrimSpace(lrcTimestamp.ReplaceAllString(rawLine, ""))
		if len(matches) == 0 {
			if textValue != "" && !strings.HasPrefix(textValue, "[") {
				line := n("line", nil)
				line.Value = textValue
				result = append(result, line)
			}
			continue
		}
		synced = true
		for _, match := range matches {
			minutes, _ := strconv.ParseInt(match[1], 10, 64)
			seconds, _ := strconv.ParseInt(match[2], 10, 64)
			fraction := int64(0)
			if match[3] != "" {
				fraction, _ = strconv.ParseInt(match[3], 10, 64)
				switch len(match[3]) {
				case 1:
					fraction *= 100
				case 2:
					fraction *= 10
				}
			}
			line := n("line", map[string]any{"start": (minutes*60+seconds)*1000 + fraction})
			line.Value = textValue
			result = append(result, line)
		}
	}
	if len(result) == 0 {
		line := n("line", nil)
		line.Value = strings.TrimSpace(content)
		result = append(result, line)
	}
	return result, synced
}

func streamHTTPClient(responseHeaderTimeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = responseHeaderTimeout
	return &http.Client{Transport: transport}
}
