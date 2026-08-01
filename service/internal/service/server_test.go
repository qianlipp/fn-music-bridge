package service

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPasswordAndTokenAuthentication(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, dataPath := newTestServer(t, api.URL)

	passwordURL := server.URL + "/rest/ping.view?u=alice&p=secret&v=1.16.1&c=test&f=json"
	assertSubsonicStatus(t, passwordURL, "ok", 0)

	salt := "1234abcd"
	digest := md5.Sum([]byte("secret" + salt))
	tokenURL := server.URL + "/rest/ping.view?u=alice&t=" + hex.EncodeToString(digest[:]) + "&s=" + salt + "&v=1.16.1&c=test&f=json"
	assertSubsonicStatus(t, tokenURL, "ok", 0)

	contents, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "secret") {
		t.Fatal("credential vault contains a plaintext password")
	}
}

func TestTokenAuthenticationRequiresEnrollment(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	assertSubsonicStatus(t, server.URL+"/rest/ping.view?u=alice&t=bad&s=salt&v=1.16.1&c=test&f=json", "failed", 41)
}

func TestOpenSubsonicExtensionsArePublic(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/rest/getOpenSubsonicExtensions.view?f=json")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	extensions, ok := object(payload["subsonic-response"])["openSubsonicExtensions"].([]any)
	if !ok || len(extensions) != 2 || stringValue(object(extensions[0]), "name") != "songLyrics" || stringValue(object(extensions[1]), "name") != "formPost" {
		t.Fatalf("unexpected extension response: %v", payload)
	}
}

func TestParseSyncedLyrics(t *testing.T) {
	lines, synced := parseLyricLines("[ar:Artist]\n[00:01.25]First\n[01:02.003]Second")
	if !synced || len(lines) != 2 {
		t.Fatalf("synced=%v lines=%v", synced, lines)
	}
	if lines[0].Attr["start"] != int64(1250) || lines[0].Value != "First" || lines[1].Attr["start"] != int64(62003) {
		t.Fatalf("incorrect lyric parsing: %#v", lines)
	}
}

func TestEncryptedPasswordAndXML(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/rest/ping.view?u=alice&p=enc:736563726574&v=1.16.1&c=test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), `status="ok"`) || !strings.Contains(string(body), `version="1.16.1"`) {
		t.Fatalf("unexpected XML response: %s", body)
	}
}

func TestCatalogAndRangeStreaming(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/rest/getAlbum.view?u=alice&p=secret&id=album-1&v=1.16.1&c=test&f=json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if !strings.Contains(string(body), `"id":"track-1"`) || !strings.Contains(string(body), `"title":"Song One"`) {
		t.Fatalf("album did not contain mapped song: %s", body)
	}

	request, _ := http.NewRequest(http.MethodGet, server.URL+"/rest/stream.view?u=alice&p=secret&id=track-1&v=1.16.1&c=test", nil)
	request.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" || stream.Header.Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range response status=%d headers=%v body=%q", stream.StatusCode, stream.Header, streamBody)
	}
}

func TestClientDisconnectDuringStreamingIsIgnored(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	directory := t.TempDir()
	handler, err := NewServer(Config{
		APIBaseURL:     api.URL,
		CredentialFile: filepath.Join(directory, "credentials.enc"),
		CredentialKey:  filepath.Join(directory, "credentials.key"),
		RequestTimeout: 5 * time.Second,
		EnableSubsonic: true,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/rest/stream.view", nil)
	request.Header.Set("Range", "bytes=2-5")
	writer := &disconnectingResponseWriter{header: make(http.Header)}
	err = handler.proxyBinary(writer, request, authSession{Token: "token"}, "/media/tracks/track-1/stream", nil)
	if err != nil {
		t.Fatalf("client disconnect should not become a protocol error: %v", err)
	}
	if writer.status != http.StatusPartialContent || writer.writeHeaderCalls != 1 {
		t.Fatalf("status=%d writeHeaderCalls=%d", writer.status, writer.writeHeaderCalls)
	}
}

type disconnectingResponseWriter struct {
	header           http.Header
	status           int
	writeHeaderCalls int
}

func (w *disconnectingResponseWriter) Header() http.Header { return w.header }

func (w *disconnectingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.writeHeaderCalls++
}

func (w *disconnectingResponseWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestSearchSongLookupAndCoverFallbacks(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	auth := "u=alice&p=secret&v=1.16.1&c=test&f=json"

	song, err := http.Get(server.URL + "/rest/getSong.view?" + auth + "&id=track-1")
	if err != nil {
		t.Fatal(err)
	}
	songBody, _ := io.ReadAll(song.Body)
	_ = song.Body.Close()
	if !strings.Contains(string(songBody), `"id":"track-1"`) {
		t.Fatalf("song lookup did not scan the user catalog: %s", songBody)
	}

	search, err := http.Get(server.URL + "/rest/search3.view?" + auth + "&query=&artistCount=5&albumCount=5&songCount=5")
	if err != nil {
		t.Fatal(err)
	}
	searchBody, _ := io.ReadAll(search.Body)
	_ = search.Body.Close()
	if !strings.Contains(string(searchBody), `"title":"Song One"`) || !strings.Contains(string(searchBody), `"name":"Artist One"`) {
		t.Fatalf("empty search did not fall back to catalog browsing: %s", searchBody)
	}

	cover, err := http.Get(server.URL + "/rest/getCoverArt.view?" + auth + "&id=album-1")
	if err != nil {
		t.Fatal(err)
	}
	coverBody, _ := io.ReadAll(cover.Body)
	_ = cover.Body.Close()
	if cover.StatusCode != http.StatusOK || cover.Header.Get("Content-Type") != "image/png" || string(coverBody) != "PNG" {
		t.Fatalf("cover fallback status/type/body = %d/%q/%q", cover.StatusCode, cover.Header.Get("Content-Type"), coverBody)
	}
}

func TestSearchCapsAndFiltersSuccessfulUpstreamResults(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	auth := "u=alice&p=secret&v=1.16.1&c=test&f=json"

	response, err := http.Get(server.URL + "/rest/search3.view?" + auth + "&query=Song+One&artistCount=0&albumCount=0&songCount=1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	root := object(payload["subsonic-response"])
	results := object(root["searchResult3"])
	songs, ok := results["song"].([]any)
	if root["status"] != "ok" || !ok || len(songs) != 1 || stringValue(object(songs[0]), "title") != "Song One" {
		t.Fatalf("search did not filter and cap upstream data: %#v", payload)
	}
}

func TestCreatePlaylistSelectsARealCover(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	auth := "u=alice&p=secret&v=1.16.1&c=test&f=json"

	response, err := http.Get(server.URL + "/rest/createPlaylist.view?" + auth + "&name=Subsonic+List")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	root := object(payload["subsonic-response"])
	playlist := object(root["playlist"])
	if root["status"] != "ok" || stringValue(playlist, "id") != "playlist-1" || stringValue(playlist, "name") != "Subsonic List" {
		t.Fatalf("unexpected createPlaylist response: %#v", payload)
	}
}

func TestEnrollment(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Post(server.URL+"/yinqiao/enroll", "application/json", strings.NewReader(`{"username":"alice","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("enrollment status: %d", response.StatusCode)
	}

	salt := "after-enroll"
	digest := md5.Sum([]byte("secret" + salt))
	assertSubsonicStatus(t, server.URL+"/rest/ping.view?u=alice&t="+hex.EncodeToString(digest[:])+"&s="+salt+"&v=1.16.1&c=test&f=json", "ok", 0)
}

func TestEnrollmentReturnsUpstreamDiagnostic(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Post(server.URL+"/yinqiao/enroll", "application/json", strings.NewReader(`{"username":"blocked","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusConflict || payload["code"] != "UPSTREAM_120003" {
		t.Fatalf("unexpected diagnostic: status=%d payload=%v", response.StatusCode, payload)
	}
}

func TestEnrollmentPreservesCredentialError(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Post(server.URL+"/yinqiao/enroll", "application/json", strings.NewReader(`{"username":"unknown","password":"wrong"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized || payload["code"] != "UPSTREAM_120002" || payload["error"] != "wrong username or password" {
		t.Fatalf("unexpected credential diagnostic: status=%d payload=%v", response.StatusCode, payload)
	}
}

func TestCompatibilityStatus(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/yinqiao/status")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status endpoint returned %d", response.StatusCode)
	}
	var initial map[string]any
	if err := json.NewDecoder(response.Body).Decode(&initial); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if initial["version"] != serverVersion || len(initial["boundUsers"].([]any)) != 0 {
		t.Fatalf("unexpected initial status: %#v", initial)
	}

	enrollment, err := http.Post(server.URL+"/yinqiao/enroll", "application/json", strings.NewReader(`{"username":"alice","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = enrollment.Body.Close()

	response, err = http.Get(server.URL + "/yinqiao/status")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var enrolled map[string]any
	if err := json.NewDecoder(response.Body).Decode(&enrolled); err != nil {
		t.Fatal(err)
	}
	users := enrolled["boundUsers"].([]any)
	if len(users) != 1 || users[0] != "alice" {
		t.Fatalf("bound account missing from status: %#v", enrolled)
	}
}

func TestAccountCollectionSupportsMultipleAccountsAndDeletion(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	for _, username := range []string{"alice", "bob"} {
		response, err := http.Post(server.URL+"/yinqiao/accounts", "application/json", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":"secret"}`, username)))
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			t.Fatalf("bind %s returned %d: %s", username, response.StatusCode, body)
		}
		_ = response.Body.Close()
	}

	response, err := http.Get(server.URL + "/yinqiao/accounts")
	if err != nil {
		t.Fatal(err)
	}
	var listed map[string]any
	if err := json.NewDecoder(response.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	accounts := listed["accounts"].([]any)
	if len(accounts) != 2 || accounts[0] != "alice" || accounts[1] != "bob" {
		t.Fatalf("unexpected account list: %#v", listed)
	}

	request, _ := http.NewRequest(http.MethodDelete, server.URL+"/yinqiao/accounts?username=alice", nil)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var removed map[string]any
	if err := json.NewDecoder(response.Body).Decode(&removed); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	remaining := removed["accounts"].([]any)
	if response.StatusCode != http.StatusOK || len(remaining) != 1 || remaining[0] != "bob" {
		t.Fatalf("unexpected deletion result: status=%d payload=%#v", response.StatusCode, removed)
	}
}

func TestSensitiveManagementMutationsRequireSameOriginInBrowsers(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	crossOrigin, _ := http.NewRequest(http.MethodPost, server.URL+"/yinqiao/accounts", strings.NewReader(`{"username":"alice","password":"secret"}`))
	crossOrigin.Header.Set("Content-Type", "application/json")
	crossOrigin.Header.Set("Origin", "https://untrusted.example")
	response, err := http.DefaultClient.Do(crossOrigin)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var blocked map[string]any
	if err := json.NewDecoder(response.Body).Decode(&blocked); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden || blocked["code"] != "CROSS_ORIGIN_BLOCKED" || response.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("cross-origin request was not blocked: status=%d headers=%v payload=%#v", response.StatusCode, response.Header, blocked)
	}

	sameOrigin, _ := http.NewRequest(http.MethodPost, server.URL+"/yinqiao/accounts", strings.NewReader(`{"username":"alice","password":"secret"}`))
	sameOrigin.Header.Set("Content-Type", "application/json")
	sameOrigin.Header.Set("Origin", server.URL)
	response, err = http.DefaultClient.Do(sameOrigin)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("same-origin request returned %d", response.StatusCode)
	}
}

func TestSettingsPersistAndControlProtocols(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	directory := t.TempDir()
	settingsPath := filepath.Join(directory, "service-settings.json")
	appliedPorts := []int{}
	handler, err := NewServer(Config{
		ListenAddr:     "127.0.0.1:14040",
		APIBaseURL:     api.URL,
		CredentialFile: filepath.Join(directory, "credentials.enc"),
		CredentialKey:  filepath.Join(directory, "credentials.key"),
		SettingsFile:   settingsPath,
		RequestTimeout: 5 * time.Second,
		EnableSubsonic: true,
		EnableAmpache:  true,
		EnableSynology: true,
		EnableJellyfin: true,
		ApplyProtocolPort: func(port int) error {
			appliedPorts = append(appliedPorts, port)
			return nil
		},
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler.Handler())
	defer server.Close()

	request, _ := http.NewRequest(http.MethodPut, server.URL+"/yinqiao/settings", strings.NewReader(`{"protocolPort":15050,"advertisedHost":"music-nas.local","protocols":{"openSubsonic":false,"synology":true,"jellyfin":false,"ampache":true}}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	settings := object(payload["settings"])
	if response.StatusCode != http.StatusOK || settings["protocolPort"] != float64(15050) || len(appliedPorts) != 1 || appliedPorts[0] != 15050 {
		t.Fatalf("unexpected settings response: status=%d payload=%#v applied=%v", response.StatusCode, payload, appliedPorts)
	}

	response, err = http.Get(server.URL + "/rest/ping.view?u=alice&p=secret&v=1.16.1&c=test")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled OpenSubsonic returned %d", response.StatusCode)
	}

	reloaded, err := openSettingsStore(settingsPath, ServiceSettings{})
	if err != nil {
		t.Fatal(err)
	}
	stored := reloaded.snapshot()
	if stored.ProtocolPort != 15050 || stored.AdvertisedHost != "music-nas.local" || stored.Protocols.OpenSubsonic || stored.Protocols.Jellyfin {
		t.Fatalf("settings were not persisted: %#v", stored)
	}
}

func TestSettingsRejectUnavailablePortWithoutChangingState(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	directory := t.TempDir()
	handler, err := NewServer(Config{
		ListenAddr: "127.0.0.1:14040", APIBaseURL: api.URL,
		CredentialFile: filepath.Join(directory, "credentials.enc"), CredentialKey: filepath.Join(directory, "credentials.key"), SettingsFile: filepath.Join(directory, "settings.json"),
		RequestTimeout: 5 * time.Second, EnableSubsonic: true, EnableAmpache: true, EnableSynology: true, EnableJellyfin: true,
		ApplyProtocolPort: func(int) error { return errors.New("address already in use") },
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler.Handler())
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPut, server.URL+"/yinqiao/settings", strings.NewReader(`{"protocolPort":15051}`))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusConflict || handler.ProtocolPort() != 14040 {
		t.Fatalf("port conflict status=%d current=%d", response.StatusCode, handler.ProtocolPort())
	}
}

func TestNetworkInfo(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/yinqiao/network")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || payload["ok"] != true || payload["port"] != "0" {
		t.Fatalf("unexpected network info: status=%d payload=%#v", response.StatusCode, payload)
	}
	if listenPort("0.0.0.0:14040") != "14040" {
		t.Fatal("production protocol port was not parsed correctly")
	}
}

func TestDashboardAssets(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	response, err := http.Get(server.URL + "/yinqiao/ui/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "音桥") {
		t.Fatalf("dashboard status=%d body=%q", response.StatusCode, body)
	}
	if response.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("dashboard is missing its content security policy")
	}

	manifest, err := http.Get(server.URL + "/yinqiao/ui/api-manifest.js")
	if err != nil {
		t.Fatal(err)
	}
	defer manifest.Body.Close()
	manifestBody, _ := io.ReadAll(manifest.Body)
	if manifest.StatusCode != http.StatusOK || strings.Count(string(manifestBody), `"id":`) != 85 {
		t.Fatalf("dashboard API manifest is incomplete: status=%d routes=%d", manifest.StatusCode, strings.Count(string(manifestBody), `"id":`))
	}
}

func TestAmpacheHandshakeCatalogAndStreaming(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	enroll, err := http.Post(server.URL+"/yinqiao/enroll", "application/json", strings.NewReader(`{"username":"alice","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = enroll.Body.Close()
	if enroll.StatusCode != http.StatusOK {
		t.Fatalf("enrollment status = %d", enroll.StatusCode)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	passwordDigest := sha256.Sum256([]byte("secret"))
	passphrase := sha256.Sum256([]byte(timestamp + hex.EncodeToString(passwordDigest[:])))
	handshakeURL := server.URL + "/server/json.server.php?action=handshake&user=alice&timestamp=" + timestamp + "&auth=" + hex.EncodeToString(passphrase[:]) + "&version=6.0.0"
	response, err := http.Get(handshakeURL)
	if err != nil {
		t.Fatal(err)
	}
	var handshake map[string]any
	if err := json.NewDecoder(response.Body).Decode(&handshake); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	token := stringValue(handshake, "auth")
	if token == "" || handshake["api"] != ampacheAPIVersion {
		t.Fatalf("unexpected handshake: %#v", handshake)
	}

	artists, err := http.Get(server.URL + "/server/xml.server.php?action=artists&auth=" + token)
	if err != nil {
		t.Fatal(err)
	}
	artistsBody, _ := io.ReadAll(artists.Body)
	_ = artists.Body.Close()
	if !strings.Contains(string(artistsBody), `<artist id="artist-1">`) || !strings.Contains(string(artistsBody), "Artist One") {
		t.Fatalf("unexpected artists response: %s", artistsBody)
	}

	songs, err := http.Get(server.URL + "/server/json.server.php?action=songs&auth=" + token)
	if err != nil {
		t.Fatal(err)
	}
	var songPayload map[string]any
	if err := json.NewDecoder(songs.Body).Decode(&songPayload); err != nil {
		t.Fatal(err)
	}
	_ = songs.Body.Close()
	if entries, ok := songPayload["song"].([]any); !ok || len(entries) != 1 || stringValue(object(entries[0]), "title") != "Song One" {
		t.Fatalf("unexpected songs response: %#v", songPayload)
	}

	streamRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/play/index.php?ssid="+token+"&type=song&oid=track-1", nil)
	streamRequest.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" {
		t.Fatalf("stream status/body = %d/%q", stream.StatusCode, streamBody)
	}
}

func TestSynologyAudioStationLoginCatalogAndStreaming(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	info, err := http.Get(server.URL + "/webapi/query.cgi?api=SYNO.API.Info&version=1&method=query&query=all")
	if err != nil {
		t.Fatal(err)
	}
	var infoPayload map[string]any
	if err := json.NewDecoder(info.Body).Decode(&infoPayload); err != nil {
		t.Fatal(err)
	}
	_ = info.Body.Close()
	apis := object(infoPayload["data"])
	if infoPayload["success"] != true || object(apis["SYNO.AudioStation.Song"])["path"] != "AudioStation/song.cgi" {
		t.Fatalf("unexpected Synology API declaration: %#v", infoPayload)
	}
	if _, incorrectlyDeclared := apis["SYNO.AudioStation.RemotePlayer"]; incorrectlyDeclared {
		t.Fatal("unsupported RemotePlayer API must not be advertised")
	}

	login, err := http.Post(server.URL+"/webapi/entry.cgi", "application/json", strings.NewReader(`{"api":"SYNO.API.Auth","version":6,"method":"login","session":"audiostation","account":"alice","passwd":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var loginPayload map[string]any
	if err := json.NewDecoder(login.Body).Decode(&loginPayload); err != nil {
		t.Fatal(err)
	}
	_ = login.Body.Close()
	sid := stringValue(object(loginPayload["data"]), "sid")
	if loginPayload["success"] != true || sid == "" {
		t.Fatalf("unexpected Synology login response: %#v", loginPayload)
	}

	songs, err := http.Get(server.URL + "/webapi/AudioStation/song.cgi?api=SYNO.AudioStation.Song&version=3&method=list&limit=20&_sid=" + sid)
	if err != nil {
		t.Fatal(err)
	}
	var songPayload map[string]any
	if err := json.NewDecoder(songs.Body).Decode(&songPayload); err != nil {
		t.Fatal(err)
	}
	_ = songs.Body.Close()
	entries := object(songPayload["data"])["songs"].([]any)
	if len(entries) != 1 || stringValue(object(entries[0]), "id") != "music_track-1" || stringValue(object(entries[0]), "title") != "Song One" {
		t.Fatalf("unexpected Synology song list: %#v", songPayload)
	}

	streamRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/webapi/AudioStation/stream.cgi?api=SYNO.AudioStation.Stream&version=2&method=stream&id=music_track-1&_sid="+sid, nil)
	streamRequest.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" {
		t.Fatalf("Synology stream status/body = %d/%q", stream.StatusCode, streamBody)
	}
}

func TestJellyfinAudioLoginBrowseLyricsAndStreaming(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	arrowPublicRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/System/Info/Public", nil)
	arrowPublicRequest.Header.Set("Content-Type", "application/json")
	arrowPublicRequest.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Amcfy Music for iOS", Device="iPhone", DeviceId="arrow-test", Version="1.0"`)
	arrowPublic, err := http.DefaultClient.Do(arrowPublicRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = arrowPublic.Body.Close()
	if arrowPublic.StatusCode != http.StatusOK {
		t.Fatalf("Arrow-style empty JSON GET public info status = %d", arrowPublic.StatusCode)
	}

	public, err := http.Get(server.URL + "/System/Info/Public")
	if err != nil {
		t.Fatal(err)
	}
	var publicPayload map[string]any
	if err := json.NewDecoder(public.Body).Decode(&publicPayload); err != nil {
		t.Fatal(err)
	}
	_ = public.Body.Close()
	if publicPayload["ProductName"] != "Jellyfin Server" || publicPayload["Id"] != jellyfinServerID {
		t.Fatalf("unexpected Jellyfin public info: %#v", publicPayload)
	}

	loginRequest, err := http.NewRequest(http.MethodPost, server.URL+"/Users/authenticatebyname", strings.NewReader(`{"Username":"alice","Pw":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Arrow Music", Device="iPhone", DeviceId="arrow-test", Version="1.2.4"`)
	login, err := http.DefaultClient.Do(loginRequest)
	if err != nil {
		t.Fatal(err)
	}
	var loginPayload map[string]any
	if err := json.NewDecoder(login.Body).Decode(&loginPayload); err != nil {
		t.Fatal(err)
	}
	_ = login.Body.Close()
	token := stringValue(loginPayload, "AccessToken")
	userID := stringValue(object(loginPayload["User"]), "Id")
	if token == "" || userID == "" {
		t.Fatalf("unexpected Jellyfin login response: %#v", loginPayload)
	}
	authorization := `MediaBrowser Client="test", Device="test", DeviceId="device", Version="1", Token="` + token + `"`

	arrowViewsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Users/"+userID+"/Views", nil)
	arrowViewsRequest.Header.Set("Content-Type", "application/json")
	arrowViewsRequest.Header.Set("X-Emby-Authorization", authorization)
	arrowViews, err := http.DefaultClient.Do(arrowViewsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var arrowViewsPayload map[string]any
	_ = json.NewDecoder(arrowViews.Body).Decode(&arrowViewsPayload)
	_ = arrowViews.Body.Close()
	if arrowViews.StatusCode != http.StatusOK || len(list(arrowViewsPayload["Items"])) != 1 {
		t.Fatalf("Arrow-style empty JSON GET views status/payload = %d/%#v", arrowViews.StatusCode, arrowViewsPayload)
	}

	countsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Items/Counts?api_key=secret", nil)
	countsRequest.Header.Set("X-Emby-Authorization", authorization)
	counts, err := http.DefaultClient.Do(countsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var countsPayload map[string]any
	if err := json.NewDecoder(counts.Body).Decode(&countsPayload); err != nil {
		t.Fatal(err)
	}
	_ = counts.Body.Close()
	if counts.StatusCode != http.StatusOK || intValue(countsPayload, "SongCount") != 1 || intValue(countsPayload, "AlbumCount") != 1 || intValue(countsPayload, "ArtistCount") != 1 {
		t.Fatalf("unexpected Jellyfin counts: status=%d payload=%#v", counts.StatusCode, countsPayload)
	}

	playlistsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Playlists?UserId="+url.QueryEscape(userID), nil)
	playlistsRequest.Header.Set("X-Emby-Authorization", authorization)
	playlists, err := http.DefaultClient.Do(playlistsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var playlistsPayload map[string]any
	if err := json.NewDecoder(playlists.Body).Decode(&playlistsPayload); err != nil {
		t.Fatal(err)
	}
	_ = playlists.Body.Close()
	if playlists.StatusCode != http.StatusOK || intValue(playlistsPayload, "TotalRecordCount") != 0 {
		t.Fatalf("unexpected Jellyfin playlists: status=%d payload=%#v", playlists.StatusCode, playlistsPayload)
	}

	itemsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Users/"+userID+"/Items?IncludeItemTypes=Audio&Recursive=true", nil)
	itemsRequest.Header.Set("Content-Type", "application/json")
	itemsRequest.Header.Set("X-Emby-Authorization", authorization)
	items, err := http.DefaultClient.Do(itemsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var itemPayload map[string]any
	if err := json.NewDecoder(items.Body).Decode(&itemPayload); err != nil {
		t.Fatal(err)
	}
	_ = items.Body.Close()
	entries := itemPayload["Items"].([]any)
	if len(entries) != 1 || stringValue(object(entries[0]), "Id") != "fnm-track-track-1" || object(entries[0])["Type"] != "Audio" {
		t.Fatalf("unexpected Jellyfin items: %#v", itemPayload)
	}

	invalidLoginRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/Users/AuthenticateByName", strings.NewReader(`{`))
	invalidLoginRequest.Header.Set("Content-Type", "application/json")
	invalidLogin, err := http.DefaultClient.Do(invalidLoginRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = invalidLogin.Body.Close()
	if invalidLogin.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON must remain rejected, status = %d", invalidLogin.StatusCode)
	}

	lyricsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/fnm-track-track-1/Lyrics", nil)
	lyricsRequest.Header.Set("X-Emby-Token", token)
	lyrics, err := http.DefaultClient.Do(lyricsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var lyricPayload map[string]any
	if err := json.NewDecoder(lyrics.Body).Decode(&lyricPayload); err != nil {
		t.Fatal(err)
	}
	_ = lyrics.Body.Close()
	if lines, ok := lyricPayload["Lyrics"].([]any); !ok || len(lines) != 2 || stringValue(object(lines[0]), "Text") != "First" {
		t.Fatalf("unexpected Jellyfin lyrics: %#v", lyricPayload)
	}

	searchRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Search/Hints?SearchTerm=One&Limit=10", nil)
	searchRequest.Header.Set("X-Emby-Token", token)
	search, err := http.DefaultClient.Do(searchRequest)
	if err != nil {
		t.Fatal(err)
	}
	var searchPayload map[string]any
	if err := json.NewDecoder(search.Body).Decode(&searchPayload); err != nil {
		t.Fatal(err)
	}
	_ = search.Body.Close()
	if hints, ok := searchPayload["SearchHints"].([]any); !ok || len(hints) < 3 {
		t.Fatalf("unexpected Jellyfin search hints: %#v", searchPayload)
	}

	streamRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/fnm-track-track-1/stream?static=true&api_key="+token, nil)
	streamRequest.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" {
		t.Fatalf("Jellyfin stream status/body = %d/%q", stream.StatusCode, streamBody)
	}

	for _, path := range []string{"/Audio/fnm-track-track-1/universal", "/Items/fnm-track-track-1/File"} {
		mediaRequest, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
		mediaRequest.Header.Set("X-Emby-Token", token)
		mediaRequest.Header.Set("Range", "bytes=2-5")
		media, err := http.DefaultClient.Do(mediaRequest)
		if err != nil {
			t.Fatal(err)
		}
		mediaBody, _ := io.ReadAll(media.Body)
		_ = media.Body.Close()
		if media.StatusCode != http.StatusPartialContent || string(mediaBody) != "2345" {
			t.Fatalf("Jellyfin media path %s status/body = %d/%q", path, media.StatusCode, mediaBody)
		}
	}

	userDataRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/UserItems/fnm-track-track-1/UserData", nil)
	userDataRequest.Header.Set("X-Emby-Token", token)
	userData, err := http.DefaultClient.Do(userDataRequest)
	if err != nil {
		t.Fatal(err)
	}
	var userDataPayload map[string]any
	_ = json.NewDecoder(userData.Body).Decode(&userDataPayload)
	_ = userData.Body.Close()
	if userData.StatusCode != http.StatusOK || userDataPayload["IsFavorite"] != false {
		t.Fatalf("unexpected Jellyfin user data: status=%d payload=%#v", userData.StatusCode, userDataPayload)
	}

	updateUserDataRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/UserItems/fnm-track-track-1/UserData", strings.NewReader(`{"IsFavorite":true}`))
	updateUserDataRequest.Header.Set("Content-Type", "application/json")
	updateUserDataRequest.Header.Set("X-Emby-Token", token)
	updateUserData, err := http.DefaultClient.Do(updateUserDataRequest)
	if err != nil {
		t.Fatal(err)
	}
	var updatedUserDataPayload map[string]any
	_ = json.NewDecoder(updateUserData.Body).Decode(&updatedUserDataPayload)
	_ = updateUserData.Body.Close()
	if updateUserData.StatusCode != http.StatusOK || updatedUserDataPayload["IsFavorite"] != true {
		t.Fatalf("unexpected updated Jellyfin user data: status=%d payload=%#v", updateUserData.StatusCode, updatedUserDataPayload)
	}

	groupingRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/UserViews/GroupingOptions", nil)
	groupingRequest.Header.Set("X-Emby-Token", token)
	grouping, err := http.DefaultClient.Do(groupingRequest)
	if err != nil {
		t.Fatal(err)
	}
	var groupingPayload []any
	_ = json.NewDecoder(grouping.Body).Decode(&groupingPayload)
	_ = grouping.Body.Close()
	if grouping.StatusCode != http.StatusOK || len(groupingPayload) != 0 {
		t.Fatalf("unexpected Jellyfin grouping options: status=%d payload=%#v", grouping.StatusCode, groupingPayload)
	}

	capabilitiesRequest, _ := http.NewRequest(http.MethodPost, server.URL+"/Sessions/Capabilities", nil)
	capabilitiesRequest.Header.Set("X-Emby-Token", token)
	capabilities, err := http.DefaultClient.Do(capabilitiesRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = capabilities.Body.Close()
	if capabilities.StatusCode != http.StatusNoContent {
		t.Fatalf("Jellyfin session capabilities status = %d", capabilities.StatusCode)
	}

	similarRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Items/fnm-track-track-1/Similar", nil)
	similarRequest.Header.Set("X-Emby-Token", token)
	similar, err := http.DefaultClient.Do(similarRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = similar.Body.Close()
	if similar.StatusCode != http.StatusNotImplemented {
		t.Fatalf("unsupported similarity must be explicit, status = %d", similar.StatusCode)
	}

	for _, unsupported := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/Users"},
		{http.MethodDelete, "/Items"},
		{http.MethodPost, "/Audio/fnm-track-track-1/Lyrics"},
		{http.MethodGet, "/Items/fnm-track-track-1/InstantMix"},
		{http.MethodGet, "/Items/fnm-track-track-1/Ancestors"},
		{http.MethodGet, "/Audio/fnm-track-track-1/RemoteSearch/Lyrics"},
		{http.MethodGet, "/Providers/Lyrics/provider-id"},
		{http.MethodPost, "/UserItems/fnm-track-track-1/UserData"},
	} {
		body := io.Reader(nil)
		if unsupported.path == "/UserItems/fnm-track-track-1/UserData" {
			body = strings.NewReader(`{"PlaybackPositionTicks":10000000}`)
		}
		request, _ := http.NewRequest(unsupported.method, server.URL+unsupported.path, body)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("X-Emby-Token", token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotImplemented {
			t.Fatalf("unsupported %s %s must be explicit, status = %d", unsupported.method, unsupported.path, response.StatusCode)
		}
	}
}

func TestJellyfinSessionSurvivesRestartAndLogout(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	directory := t.TempDir()
	config := Config{
		ListenAddr:     "127.0.0.1:0",
		APIBaseURL:     api.URL,
		CredentialFile: filepath.Join(directory, "credentials.enc"),
		CredentialKey:  filepath.Join(directory, "credentials.key"),
		SettingsFile:   filepath.Join(directory, "settings.json"),
		RequestTimeout: 5 * time.Second,
		EnableSubsonic: true,
		EnableAmpache:  true,
		EnableSynology: true,
		EnableJellyfin: true,
	}
	start := func() *httptest.Server {
		handler, err := NewServer(config, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		return httptest.NewServer(handler.Handler())
	}

	first := start()
	token, _ := jellyfinTestLogin(t, first.URL)
	first.Close()

	ciphertext, err := os.ReadFile(filepath.Join(directory, "yinqiao-sessions.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(token)) || bytes.Contains(ciphertext, []byte("alice")) {
		t.Fatal("persisted Jellyfin session must be encrypted")
	}

	second := start()
	itemsRequest, _ := http.NewRequest(http.MethodGet, second.URL+"/Items?IncludeItemTypes=Audio&Limit=1", nil)
	itemsRequest.Header.Set("X-Emby-Token", token)
	items, err := http.DefaultClient.Do(itemsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	_ = json.NewDecoder(items.Body).Decode(&payload)
	_ = items.Body.Close()
	if items.StatusCode != http.StatusOK || len(list(payload["Items"])) != 1 {
		t.Fatalf("persisted Jellyfin session was not restored: status=%d payload=%#v", items.StatusCode, payload)
	}

	logoutRequest, _ := http.NewRequest(http.MethodPost, second.URL+"/Sessions/Logout", nil)
	logoutRequest.Header.Set("X-Emby-Token", token)
	logout, err := http.DefaultClient.Do(logoutRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = logout.Body.Close()
	if logout.StatusCode != http.StatusNoContent {
		t.Fatalf("Jellyfin logout status = %d", logout.StatusCode)
	}
	second.Close()

	third := start()
	defer third.Close()
	revokedRequest, _ := http.NewRequest(http.MethodGet, third.URL+"/Items?IncludeItemTypes=Audio&Limit=1", nil)
	revokedRequest.Header.Set("X-Emby-Token", token)
	revoked, err := http.DefaultClient.Do(revokedRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = revoked.Body.Close()
	if revoked.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logged-out Jellyfin token survived restart: status=%d", revoked.StatusCode)
	}
}

func TestArrowMusicLibraryQueriesUseFavoritesHistoryRandomAndSearch(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	token, userID := jellyfinTestLogin(t, server.URL)
	defer server.Close()

	tests := []struct {
		name      string
		query     string
		wantID    string
		wantTotal int64
	}{
		{name: "favorites", query: "Fields=BasicSyncInfo%2CProductionYear%2CPath&Filters=IsFavorite&ImageTypeLimit=1&IncludeItemTypes=Audio&Limit=21&MediaType=Audio&Recursive=true&SortBy=SortName&SortOrder=Ascending&StartIndex=0", wantID: "fnm-track-favorite-1", wantTotal: 1},
		{name: "favorite-albums", query: "Filters=IsFavorite&IncludeItemTypes=MusicAlbum&Limit=21&Recursive=true&StartIndex=0", wantID: "fnm-album-favorite-album", wantTotal: 1},
		{name: "recent", query: "IncludeItemTypes=Audio&Limit=21&MediaType=Audio&Recursive=true&SortBy=DatePlayed&SortOrder=Descending&StartIndex=0", wantID: "fnm-track-history-1", wantTotal: 1},
		{name: "random", query: "IncludeItemTypes=Audio&Limit=21&MediaType=Audio&Recursive=true&SortBy=Random&SortOrder=Descending&StartIndex=0", wantID: "fnm-track-track-1", wantTotal: 1},
		{name: "search", query: "IncludeItemTypes=Audio&Limit=100&Recursive=true&SearchTerm=Song%20One&StartIndex=0&enableTotalRecordCount=false&tabKey=songs", wantID: "fnm-track-track-1", wantTotal: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, _ := http.NewRequest(http.MethodGet, server.URL+"/Users/"+userID+"/Items?"+test.query, nil)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Emby-Token", token)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			_ = json.NewDecoder(response.Body).Decode(&payload)
			_ = response.Body.Close()
			items := list(payload["Items"])
			if response.StatusCode != http.StatusOK || len(items) != 1 || stringValue(object(items[0]), "Id") != test.wantID || intValue(payload, "TotalRecordCount") != test.wantTotal {
				t.Fatalf("unexpected %s response: status=%d payload=%#v", test.name, response.StatusCode, payload)
			}
			if test.name == "favorites" && object(object(items[0])["UserData"])["IsFavorite"] != true {
				t.Fatalf("favorite track is missing Jellyfin favorite user data: %#v", items[0])
			}
			if test.name == "recent" && stringValue(object(object(items[0])["UserData"]), "LastPlayedDate") == "" {
				t.Fatalf("history track is missing LastPlayedDate: %#v", items[0])
			}
		})
	}
}

func jellyfinTestLogin(t *testing.T, baseURL string) (string, string) {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/Users/AuthenticateByName", strings.NewReader(`{"Username":"alice","Pw":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Emby-Authorization", `MediaBrowser Client="Amcfy Music for iOS", Device="iPhone", DeviceId="arrow-test", Version="1.0"`)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	token := stringValue(payload, "AccessToken")
	userID := stringValue(object(payload["User"]), "Id")
	if response.StatusCode != http.StatusOK || token == "" || userID == "" {
		t.Fatalf("Jellyfin test login failed: status=%d payload=%#v", response.StatusCode, payload)
	}
	return token, userID
}

func TestJellyfinPaginationReservedPathsAndCaseCompatibility(t *testing.T) {
	var trackListCalls int
	api := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/auth/login/password":
			writeAPIEnvelope(writer, true, map[string]any{"userToken": "token", "user": map[string]any{"guid": "user-1", "name": "alice", "role": "member"}}, "", "")
		case "/catalog/tracks":
			trackListCalls++
			page := boundedInt(request.URL.Query().Get("page"), 1, 1, 10_000)
			size := boundedInt(request.URL.Query().Get("size"), 50, 1, 50)
			start := (page - 1) * size
			end := min(start+size, 123)
			items := make([]any, 0, max(0, end-start))
			for index := start; index < end; index++ {
				items = append(items, map[string]any{
					"guid": fmt.Sprintf("track-%03d", index), "title": fmt.Sprintf("Song %03d", index), "duration": 180000,
					"artists":   []any{map[string]any{"guid": "artist-1", "name": "Artist"}},
					"album":     map[string]any{"guid": "album-1", "name": "Album"},
					"audioSpec": map[string]any{"extension": "mp3", "duration": 180000},
				})
			}
			writeAPIEnvelope(writer, true, map[string]any{"list": items, "total": 123}, "", "")
		case "/catalog/albums":
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{}, "total": 17}, "", "")
		case "/catalog/artists":
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{}, "total": 9}, "", "")
		case "/me/playlists":
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{}, "total": 3}, "", "")
		default:
			writer.WriteHeader(http.StatusNotFound)
			writeAPIEnvelope(writer, false, nil, "NOT_FOUND", request.URL.Path)
		}
	}))
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	login, err := http.Post(server.URL+"/AuDiO/EmBy/uSeRs/aUtHeNtIcAtEbYnAmE", "application/json", strings.NewReader(`{"Username":"alice","Password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var loginPayload map[string]any
	if err := json.NewDecoder(login.Body).Decode(&loginPayload); err != nil {
		t.Fatal(err)
	}
	_ = login.Body.Close()
	token := stringValue(loginPayload, "AccessToken")
	if login.StatusCode != http.StatusOK || token == "" {
		t.Fatalf("mixed-case login failed: status=%d payload=%#v", login.StatusCode, loginPayload)
	}

	itemsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/AuDiO/EmBy/iTeMs?includeitemtypes=audio&startindex=45&limit=20", nil)
	itemsRequest.Header.Set("X-Emby-Token", token)
	items, err := http.DefaultClient.Do(itemsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.NewDecoder(items.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	_ = items.Body.Close()
	values := list(payload["Items"])
	if items.StatusCode != http.StatusOK || len(values) != 20 || intValue(payload, "TotalRecordCount") != 123 || intValue(payload, "StartIndex") != 45 {
		t.Fatalf("cross-page result is incomplete: status=%d payload=%#v", items.StatusCode, payload)
	}
	if stringValue(object(values[0]), "Id") != "fnm-track-track-045" || stringValue(object(values[19]), "Id") != "fnm-track-track-064" {
		t.Fatalf("cross-page IDs are wrong: first=%q last=%q", stringValue(object(values[0]), "Id"), stringValue(object(values[19]), "Id"))
	}
	if trackListCalls != 2 {
		t.Fatalf("cross-page request used %d upstream calls, want 2", trackListCalls)
	}

	rootRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/JeLlYfIn/ITEMS/ROOT", nil)
	rootRequest.Header.Set("X-Emby-Token", token)
	root, err := http.DefaultClient.Do(rootRequest)
	if err != nil {
		t.Fatal(err)
	}
	var rootPayload map[string]any
	_ = json.NewDecoder(root.Body).Decode(&rootPayload)
	_ = root.Body.Close()
	if root.StatusCode != http.StatusOK || rootPayload["Type"] != "CollectionFolder" || trackListCalls != 2 {
		t.Fatalf("Items/Root fell through to track lookup: status=%d calls=%d payload=%#v", root.StatusCode, trackListCalls, rootPayload)
	}

	countsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/items/COUNTS", nil)
	countsRequest.Header.Set("X-Emby-Token", token)
	counts, err := http.DefaultClient.Do(countsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var countPayload map[string]any
	_ = json.NewDecoder(counts.Body).Decode(&countPayload)
	_ = counts.Body.Close()
	if counts.StatusCode != http.StatusOK || intValue(countPayload, "SongCount") != 123 || intValue(countPayload, "AlbumCount") != 17 || intValue(countPayload, "ArtistCount") != 9 || intValue(countPayload, "PlaylistCount") != 3 {
		t.Fatalf("Items/Counts does not preserve upstream totals: status=%d payload=%#v", counts.StatusCode, countPayload)
	}
}

func TestRedactedJellyfinQuery(t *testing.T) {
	query := url.Values{
		"api_key":          {"secret-key"},
		"AccessToken":      {"secret-token"},
		"password":         {"secret-password"},
		"IncludeItemTypes": {"Audio"},
	}
	encoded := redactedJellyfinQuery(query)
	if strings.Contains(encoded, "secret") {
		t.Fatalf("sensitive Jellyfin query value was logged: %s", encoded)
	}
	if !strings.Contains(encoded, "IncludeItemTypes=Audio") {
		t.Fatalf("non-sensitive Jellyfin query value was removed: %s", encoded)
	}
}

func TestJellyfinAudioBasePathLoginAndBrowse(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	public, err := http.Get(server.URL + "/Audio/System/Info/Public")
	if err != nil {
		t.Fatal(err)
	}
	var publicPayload map[string]any
	if err := json.NewDecoder(public.Body).Decode(&publicPayload); err != nil {
		t.Fatal(err)
	}
	_ = public.Body.Close()
	if public.StatusCode != http.StatusOK || publicPayload["ProductName"] != "Jellyfin Server" {
		t.Fatalf("unexpected prefixed Jellyfin public info: status=%d payload=%#v", public.StatusCode, publicPayload)
	}

	login, err := http.Post(server.URL+"/Audio/Users/AuthenticateByName", "application/json", strings.NewReader(`{"Username":"alice","Pw":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var loginPayload map[string]any
	if err := json.NewDecoder(login.Body).Decode(&loginPayload); err != nil {
		t.Fatal(err)
	}
	_ = login.Body.Close()
	token := stringValue(loginPayload, "AccessToken")
	userID := stringValue(object(loginPayload["User"]), "Id")
	if login.StatusCode != http.StatusOK || token == "" || userID == "" {
		t.Fatalf("unexpected prefixed Jellyfin login: status=%d payload=%#v", login.StatusCode, loginPayload)
	}

	itemsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/Users/"+userID+"/Items?IncludeItemTypes=Audio", nil)
	itemsRequest.Header.Set("X-Emby-Token", token)
	items, err := http.DefaultClient.Do(itemsRequest)
	if err != nil {
		t.Fatal(err)
	}
	var itemPayload map[string]any
	if err := json.NewDecoder(items.Body).Decode(&itemPayload); err != nil {
		t.Fatal(err)
	}
	_ = items.Body.Close()
	if items.StatusCode != http.StatusOK || intValue(itemPayload, "TotalRecordCount") != 1 {
		t.Fatalf("unexpected prefixed Jellyfin items: status=%d payload=%#v", items.StatusCode, itemPayload)
	}

	streamRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/Audio/fnm-track-track-1/stream", nil)
	streamRequest.Header.Set("X-Emby-Token", token)
	streamRequest.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" {
		t.Fatalf("prefixed Jellyfin stream status/body = %d/%q", stream.StatusCode, streamBody)
	}
}

func TestJellyfinEmbyNamespaceAndTokenVariants(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)

	public, err := http.Get(server.URL + "/Audio/emby/System/Info/Public")
	if err != nil {
		t.Fatal(err)
	}
	_ = public.Body.Close()
	if public.StatusCode != http.StatusOK {
		t.Fatalf("prefixed Emby discovery status = %d", public.StatusCode)
	}

	login, err := http.Post(server.URL+"/Audio/emby/Users/AuthenticateByName", "application/json", strings.NewReader(`{"Username":"alice","Password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var loginPayload map[string]any
	if err := json.NewDecoder(login.Body).Decode(&loginPayload); err != nil {
		t.Fatal(err)
	}
	_ = login.Body.Close()
	token := stringValue(loginPayload, "AccessToken")
	userID := stringValue(object(loginPayload["User"]), "Id")
	if login.StatusCode != http.StatusOK || token == "" || userID == "" {
		t.Fatalf("prefixed Emby login: status=%d payload=%#v", login.StatusCode, loginPayload)
	}

	itemsRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/emby/Users/"+userID+"/Items?IncludeItemTypes=Audio", nil)
	itemsRequest.Header.Set("Authorization", `MediaBrowser Client="test", token="`+token+`"`)
	items, err := http.DefaultClient.Do(itemsRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = items.Body.Close()
	if items.StatusCode != http.StatusOK {
		t.Fatalf("lowercase MediaBrowser token status = %d", items.StatusCode)
	}

	streamRequest, _ := http.NewRequest(http.MethodGet, server.URL+"/Audio/emby/Audio/fnm-track-track-1/stream", nil)
	streamRequest.Header.Set("Authorization", "Bearer "+token)
	streamRequest.Header.Set("Range", "bytes=2-5")
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamBody, _ := io.ReadAll(stream.Body)
	_ = stream.Body.Close()
	if stream.StatusCode != http.StatusPartialContent || string(streamBody) != "2345" {
		t.Fatalf("Bearer stream status/body = %d/%q", stream.StatusCode, streamBody)
	}

	jellyfinPublic, err := http.Get(server.URL + "/jellyfin/System/Info/Public")
	if err != nil {
		t.Fatal(err)
	}
	_ = jellyfinPublic.Body.Close()
	if jellyfinPublic.StatusCode != http.StatusOK {
		t.Fatalf("Jellyfin namespace discovery status = %d", jellyfinPublic.StatusCode)
	}
}

func TestProtocolCapabilitiesDeclareUnsupportedAreas(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	response, err := http.Get(server.URL + "/yinqiao/capabilities")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	protocols := payload["protocols"].([]any)
	if len(protocols) != 4 || len(object(protocols[2])["unsupported"].([]any)) == 0 || len(object(protocols[3])["unsupported"].([]any)) == 0 || payload["unsupportedStatus"] != "不支持" {
		t.Fatalf("protocol support declaration is incomplete: %#v", payload)
	}
}

func TestAPICapabilitiesProxy(t *testing.T) {
	api := newFakeAPI(t)
	defer api.Close()
	server, _ := newTestServer(t, api.URL)
	response, err := http.Get(server.URL + "/yinqiao/api-capabilities")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	data := object(payload["data"])
	capabilities := object(data["capabilities"])
	if response.StatusCode != http.StatusOK || payload["ok"] != true || capabilities["count"] != float64(2) || len(capabilities["routes"].([]any)) != 2 {
		t.Fatalf("unexpected API capability payload: %#v", payload)
	}
}

func newTestServer(t *testing.T, apiURL string) (*httptest.Server, string) {
	t.Helper()
	directory := t.TempDir()
	dataPath := filepath.Join(directory, "credentials.enc")
	handler, err := NewServer(Config{
		ListenAddr:     "127.0.0.1:0",
		APIBaseURL:     apiURL,
		CredentialFile: dataPath,
		CredentialKey:  filepath.Join(directory, "credentials.key"),
		RequestTimeout: 5 * time.Second,
		EnableSubsonic: true,
		EnableAmpache:  true,
		EnableSynology: true,
		EnableJellyfin: true,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(handler.Handler()), dataPath
}

func newFakeAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/":
			writeAPIEnvelope(writer, true, map[string]any{"name": "yinqiao-api", "version": "1.0.4", "apiVersion": "v1", "prefix": "/yinqiao/v1", "capabilityCount": 2}, "", "")
		case "/capabilities":
			writeAPIEnvelope(writer, true, map[string]any{
				"count": 2,
				"routes": []any{
					map[string]any{"id": "auth.me", "category": "auth", "summary": "读取当前用户", "method": "GET", "path": "/yinqiao/v1/auth/me", "access": "user", "responseMode": "json"},
					map[string]any{"id": "playback.stream", "category": "playback", "summary": "流式读取原音频", "method": "GET", "path": "/yinqiao/v1/media/tracks/{guid}/stream", "access": "media", "responseMode": "raw"},
				},
				"upstreamPassthrough": map[string]any{"path": "/yinqiao/v1/upstream/{officialApiPath}", "methods": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}},
			}, "", "")
		case "/auth/login/password":
			var input map[string]any
			_ = json.NewDecoder(request.Body).Decode(&input)
			if input["username"] == "blocked" {
				writer.WriteHeader(http.StatusConflict)
				writeAPIEnvelope(writer, false, nil, "UPSTREAM_120003", "account is temporarily banned")
				return
			}
			expectedPassword := sha256.Sum256([]byte("secret"))
			username, _ := input["username"].(string)
			if (username != "alice" && username != "bob") || input["password"] != hex.EncodeToString(expectedPassword[:]) {
				writer.WriteHeader(http.StatusUnauthorized)
				writeAPIEnvelope(writer, false, nil, "UPSTREAM_120002", "wrong username or password")
				return
			}
			token := "token"
			if username != "alice" {
				token = "token-" + username
			}
			writeAPIEnvelope(writer, true, map[string]any{"userToken": token, "user": map[string]any{"name": username, "role": "member"}}, "", "")
		case "/ready":
			writeAPIEnvelope(writer, true, map[string]any{"status": "ready", "upstream": "ready"}, "", "")
		case "/catalog/albums/detail":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"guid": "album-1", "name": "Album One", "coverId": "cover-1", "artists": []any{map[string]any{"guid": "artist-1", "name": "Artist One"}}}, "", "")
		case "/catalog/albums":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{
				"list": []any{map[string]any{
					"guid": "album-1", "name": "Album One", "coverId": "cover-1",
					"artists": []any{map[string]any{"guid": "artist-1", "name": "Artist One"}},
				}},
				"total": 1,
			}, "", "")
		case "/catalog/tracks/by-album", "/catalog/tracks":
			requireBearer(t, request)
			track := map[string]any{
				"guid": "track-1", "title": "Song One", "duration": 180000, "coverId": "cover-1",
				"album":     map[string]any{"guid": "album-1", "name": "Album One"},
				"artists":   []any{map[string]any{"guid": "artist-1", "name": "Artist One"}},
				"audioSpec": map[string]any{"extension": "mp3", "codec": "mp3", "container": "mp3", "duration": 180000, "bitrate": 320000, "sampleRate": 44100, "channel": 2, "size": 10},
			}
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{track}, "total": 1}, "", "")
		case "/catalog/genres":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{"guid": "genre-1", "name": "Pop", "trackCount": 1}}, "total": 1}, "", "")
		case "/catalog/lyrics":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{"guid": "lyric-1", "content": "[00:01.00]First\n[00:02.50]Second"}}}, "", "")
		case "/search/tracks":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{
				map[string]any{
					"guid": "unrelated", "title": "Another Track",
					"album":   map[string]any{"guid": "album-2", "name": "Other Album"},
					"artists": []any{map[string]any{"guid": "artist-2", "name": "Other Artist"}},
				},
				map[string]any{
					"guid": "track-1", "title": "Song One",
					"album":   map[string]any{"guid": "album-1", "name": "Album One"},
					"artists": []any{map[string]any{"guid": "artist-1", "name": "Artist One"}},
				},
			}, "total": 2}, "", "")
		case "/search/artists":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{"guid": "artist-1", "name": "Artist One", "albumCount": 1, "trackCount": 1}}, "total": 1}, "", "")
		case "/search/albums":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{
				"list": []any{map[string]any{
					"guid": "album-1", "name": "Album One", "coverId": "cover-1",
					"artists": []any{map[string]any{"guid": "artist-1", "name": "Artist One"}},
				}},
				"total": 1,
			}, "", "")
		case "/me/playlists":
			requireBearer(t, request)
			if request.Method == http.MethodPost {
				var input map[string]any
				_ = json.NewDecoder(request.Body).Decode(&input)
				if input["name"] != "Subsonic List" || input["coverId"] != "cover-1" {
					t.Errorf("unexpected playlist create body: %#v", input)
				}
				writeAPIEnvelope(writer, true, map[string]any{"guid": "playlist-1", "name": input["name"], "coverId": input["coverId"]}, "", "")
				return
			}
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{}, "total": 0}, "", "")
		case "/me/playlists/detail":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"guid": "playlist-1", "name": "Subsonic List", "coverId": "cover-1"}, "", "")
		case "/catalog/tracks/by-playlist":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{}, "total": 0}, "", "")
		case "/catalog/artists", "/catalog/artists/all":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{"guid": "artist-1", "name": "Artist One", "albumCount": 1, "trackCount": 1}}, "total": 1}, "", "")
		case "/media/tracks/track-1/stream":
			requireBearer(t, request)
			if request.Header.Get("Range") != "bytes=2-5" {
				t.Errorf("range header not forwarded: %q", request.Header.Get("Range"))
			}
			writer.Header().Set("Content-Type", "audio/mpeg")
			writer.Header().Set("Content-Range", "bytes 2-5/10")
			writer.Header().Set("Accept-Ranges", "bytes")
			writer.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(writer, "2345")
		case "/media/covers/cover-1":
			requireBearer(t, request)
			writer.Header().Set("Content-Type", "image/png")
			_, _ = io.WriteString(writer, "PNG")
		case "/me/favorites":
			requireBearer(t, request)
			if request.Method != http.MethodGet {
				writeAPIEnvelope(writer, true, map[string]any{}, "", "")
				return
			}
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{
				"guid": "favorite-1", "title": "Favorite Song", "duration": 180000, "coverId": "cover-1", "isFavorite": true,
				"album":   map[string]any{"guid": "favorite-album", "name": "Favorite Album", "coverId": "cover-1"},
				"artists": []any{map[string]any{"guid": "favorite-artist", "name": "Favorite Artist"}},
			}}, "total": 1}, "", "")
		case "/me/history":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{"list": []any{map[string]any{
				"guid": "history-1", "title": "Recently Played", "duration": 180000, "coverId": "cover-1", "playedAt": int64(1785560000000), "playCount": 4,
				"album":   map[string]any{"guid": "history-album", "name": "History Album"},
				"artists": []any{map[string]any{"guid": "history-artist", "name": "History Artist"}},
			}}, "total": 1}, "", "")
		case "/me/favorites/remove", "/activity/events":
			requireBearer(t, request)
			writeAPIEnvelope(writer, true, map[string]any{}, "", "")
		default:
			writer.WriteHeader(http.StatusNotFound)
			writeAPIEnvelope(writer, false, nil, "NOT_FOUND", request.URL.Path)
		}
	}))
}

func requireBearer(t *testing.T, request *http.Request) {
	t.Helper()
	if request.Header.Get("Authorization") != "Bearer token" {
		t.Errorf("missing API bearer token")
	}
}

func writeAPIEnvelope(writer http.ResponseWriter, ok bool, data any, code, message string) {
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"ok": ok, "data": data, "error": map[string]any{"code": code, "message": message},
	})
}

func assertSubsonicStatus(t *testing.T, target, expected string, errorCode int) {
	t.Helper()
	response, err := http.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	root := object(payload["subsonic-response"])
	if root["status"] != expected {
		t.Fatalf("status=%v payload=%v", root["status"], payload)
	}
	if errorCode != 0 {
		actual := intValue(object(root["error"]), "code")
		if actual != int64(errorCode) {
			t.Fatalf("error code=%d, want=%d payload=%v", actual, errorCode, payload)
		}
	}
}
