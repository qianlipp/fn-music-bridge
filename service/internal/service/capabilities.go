package service

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleAPICapabilities(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": "METHOD_NOT_ALLOWED", "error": "method not allowed"})
		return
	}

	capabilities, err := s.api.call(request.Context(), http.MethodGet, "/capabilities", "", nil, nil)
	if err != nil {
		status, code, message := classifyEnrollmentError(err)
		if status == http.StatusUnauthorized {
			status = http.StatusBadGateway
		}
		writer.WriteHeader(status)
		_ = json.NewEncoder(writer).Encode(map[string]any{"ok": false, "code": code, "error": message})
		return
	}

	info, err := s.api.call(request.Context(), http.MethodGet, "", "", nil, nil)
	if err != nil {
		info = map[string]any{}
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": true,
		"data": map[string]any{
			"info":         info,
			"capabilities": capabilities,
		},
	})
}

type protocolCapability struct {
	Name        string   `json:"name"`
	Enabled     bool     `json:"enabled"`
	BasePaths   []string `json:"basePaths"`
	Supported   []string `json:"supported"`
	Unsupported []string `json:"unsupported"`
}

func (s *Server) handleProtocolCapabilities(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok":      true,
		"version": serverVersion,
		"protocols": []protocolCapability{
			{
				Name: "OpenSubsonic/Subsonic 1.16.1", Enabled: s.protocolEnabled("openSubsonic"),
				BasePaths:   []string{"/rest"},
				Supported:   []string{"authentication", "artists", "albums", "songs", "genres", "search", "favorites", "playlists", "cover art", "Range streaming", "download", "lyrics", "play reporting", "scan status"},
				Unsupported: []string{"podcasts", "internet radio", "video", "chat", "shares", "jukebox", "bookmarks", "server-side play queue persistence"},
			},
			{
				Name: "Ampache API 6", Enabled: s.protocolEnabled("ampache"),
				BasePaths:   []string{"/server/xml.server.php", "/server/json.server.php"},
				Supported:   []string{"digest handshake", "artists", "albums", "songs", "genres", "search", "playlists", "cover art", "Range streaming"},
				Unsupported: []string{"podcasts", "video", "radio", "catalog administration", "user administration", "local playback control"},
			},
			{
				Name: "Synology DSM WebAPI / Audio Station", Enabled: s.protocolEnabled("synology"),
				BasePaths:   []string{"/webapi/query.cgi", "/webapi/entry.cgi", "/webapi/AudioStation"},
				Supported:   []string{"SYNO.API.Info", "SYNO.API.Auth", "SYNO.AudioStation.Info", "Album", "Artist", "Song", "Genre", "Folder", "Search", "Cover", "Stream", "Download", "Lyrics", "Playlist", "song rating as favorite"},
				Unsupported: []string{"QuickConnect discovery", "OTP", "DSM administration", "RemotePlayer", "MediaServer", "Radio", "LyricsSearch plugins", "smart playlists", "playlist sharing", "WebPlayer queue", "composer index"},
			},
			{
				Name: "Jellyfin Audio compatibility", Enabled: s.protocolEnabled("jellyfin"),
				BasePaths:   []string{"/System", "/Users", "/Items", "/Artists", "/Genres", "/Audio", "/Playlists", "/Sessions"},
				Supported:   []string{"public server discovery", "password authentication", "encrypted persistent sessions", "users", "music views", "artists", "albums", "songs", "genres", "real pagination and counts", "search", "song favorites and favorite-derived album/artist browsing", "play history and library-wide random browsing", "playlists", "images", "direct Range streaming", "universal direct audio", "download and file", "lyrics reading", "play reporting", "scan status", "session logout"},
				Unsupported: []string{"video", "Live TV", "channels", "plugins", "remote session control", "SyncPlay", "WebSocket events", "subtitle APIs", "transcoding", "Jellyfin administration", "album and artist favorite mutations", "similar items and Instant Mix", "lyrics upload or deletion", "playlist reordering and collaborative users", "ratings"},
			},
		},
		"unsupportedStatus": "不支持",
	})
}
