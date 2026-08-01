#!/usr/bin/env python3
"""Black-box acceptance client for the fnmusic Subsonic gateway."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import secrets
import sys
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable


PROTOCOL_VERSION = "1.16.1"
MAX_TEXT_BODY = 2 * 1024 * 1024
MAX_BINARY_BODY = 64 * 1024

SUPPORTED_METHODS = [
    "getOpenSubsonicExtensions",
    "ping",
    "getLicense",
    "getMusicFolders",
    "getUser",
    "getArtists",
    "getIndexes",
    "getArtist",
    "getAlbum",
    "getSong",
    "getMusicDirectory",
    "getGenres",
    "getAlbumList",
    "getAlbumList2",
    "getRandomSongs",
    "getSongsByGenre",
    "getStarred",
    "getStarred2",
    "search2",
    "search3",
    "getPlaylists",
    "getPlaylist",
    "createPlaylist",
    "updatePlaylist",
    "deletePlaylist",
    "star",
    "unstar",
    "scrobble",
    "stream",
    "download",
    "getCoverArt",
    "getLyrics",
    "getLyricsBySongId",
    "getScanStatus",
    "startScan",
    "getNowPlaying",
    "getBookmarks",
    "getInternetRadioStations",
    "getShares",
    "getVideos",
    "getPodcasts",
    "getNewestPodcasts",
    "getPlayQueue",
    "getChatMessages",
    "getSimilarSongs",
    "getSimilarSongs2",
    "getTopSongs",
    "getArtistInfo",
    "getArtistInfo2",
    "getAlbumInfo",
    "getAlbumInfo2",
    "setRating",
    "savePlayQueue",
    "createBookmark",
    "deleteBookmark",
]

PLACEHOLDER_METHODS = {
    "getNowPlaying",
    "getBookmarks",
    "getInternetRadioStations",
    "getShares",
    "getVideos",
    "getPodcasts",
    "getNewestPodcasts",
    "getPlayQueue",
    "getChatMessages",
    "getSimilarSongs",
    "getSimilarSongs2",
    "getTopSongs",
    "getArtistInfo",
    "getArtistInfo2",
    "getAlbumInfo",
    "getAlbumInfo2",
    "setRating",
    "savePlayQueue",
    "createBookmark",
    "deleteBookmark",
}


class SkipTest(Exception):
    pass


@dataclass
class HTTPResult:
    request_id: str
    protocol_method: str
    http_method: str
    url: str
    params: dict[str, Any]
    status: int
    elapsed_ms: int
    content_type: str
    headers: dict[str, str]
    body_size: int
    body_truncated: bool
    artifact: str
    body: bytes = field(repr=False)


@dataclass
class TestResult:
    name: str
    category: str
    methods: list[str]
    implementation: str
    status: str
    elapsed_ms: int
    detail: str = ""
    protocol_error_code: int | None = None
    protocol_error_message: str = ""
    http_statuses: list[int] = field(default_factory=list)
    artifacts: list[str] = field(default_factory=list)


def utc_now() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def slug(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.-]+", "-", value).strip("-") or "request"


def listify(value: Any) -> list[Any]:
    if value is None:
        return []
    return value if isinstance(value, list) else [value]


def first_dict(values: Any) -> dict[str, Any] | None:
    for value in listify(values):
        if isinstance(value, dict):
            return value
    return None


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def normalized_search_text(value: Any) -> str:
    return "".join(character.casefold() for character in str(value or "") if character.isalnum())


class Client:
    def __init__(
        self,
        base_url: str,
        username: str,
        password: str,
        run_id: str,
        response_dir: Path,
        timeout: float,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.username = username
        self.password = password
        self.run_id = run_id
        self.client_name = f"yinqiao-autotest-{run_id}"
        self.response_dir = response_dir
        self.timeout = timeout
        self.requests: list[HTTPResult] = []
        self.counter = 0

    def _auth_params(self, mode: str) -> dict[str, str]:
        if mode == "none":
            return {}
        if mode == "password":
            return {"u": self.username, "p": self.password}
        if mode == "enc":
            return {"u": self.username, "p": "enc:" + self.password.encode().hex()}
        salt = secrets.token_hex(8)
        token = hashlib.md5((self.password + salt).encode()).hexdigest()
        return {"u": self.username, "t": token, "s": salt}

    def call(
        self,
        method: str,
        params: dict[str, Any] | None = None,
        *,
        response_format: str = "json",
        http_method: str = "POST",
        auth_mode: str = "token",
        binary: bool = False,
        headers: dict[str, str] | None = None,
    ) -> HTTPResult:
        fields: dict[str, Any] = {
            **self._auth_params(auth_mode),
            "v": PROTOCOL_VERSION,
            "c": self.client_name,
        }
        if response_format:
            fields["f"] = response_format
        if params:
            fields.update(params)
        return self._request(
            path=f"/rest/{method}.view",
            protocol_method=method,
            params=fields,
            http_method=http_method,
            binary=binary,
            headers=headers,
        )

    def raw(self, path: str, *, http_method: str = "GET") -> HTTPResult:
        return self._request(
            path=path,
            protocol_method=path,
            params={},
            http_method=http_method,
            binary=False,
            headers=None,
        )

    def _request(
        self,
        *,
        path: str,
        protocol_method: str,
        params: dict[str, Any],
        http_method: str,
        binary: bool,
        headers: dict[str, str] | None,
    ) -> HTTPResult:
        self.counter += 1
        request_id = f"{self.counter:03d}-{slug(protocol_method)}"
        encoded = urllib.parse.urlencode(params, doseq=True).encode()
        target = self.base_url + path
        body = None
        request_headers = {
            "Accept": "*/*" if binary else "application/json, text/xml;q=0.9, */*;q=0.1",
            "User-Agent": self.client_name,
        }
        if headers:
            request_headers.update(headers)
        if http_method.upper() == "GET":
            if encoded:
                target += ("&" if "?" in target else "?") + encoded.decode()
        else:
            body = encoded
            request_headers["Content-Type"] = "application/x-www-form-urlencoded"
        request = urllib.request.Request(target, data=body, method=http_method.upper(), headers=request_headers)
        started = time.monotonic()
        try:
            response = urllib.request.urlopen(request, timeout=self.timeout)
        except urllib.error.HTTPError as error:
            response = error
        elapsed_ms = round((time.monotonic() - started) * 1000)
        limit = MAX_BINARY_BODY if binary else MAX_TEXT_BODY
        payload = response.read(limit + 1)
        truncated = len(payload) > limit
        if truncated:
            payload = payload[:limit]
        response_headers = {key: value for key, value in response.headers.items()}
        content_type = response.headers.get("Content-Type", "")
        suffix = ".bin" if binary else (".json" if "json" in content_type else ".xml" if "xml" in content_type else ".txt")
        artifact_path = self.response_dir / f"{request_id}{suffix}"
        artifact_path.write_bytes(payload)
        safe_params = {key: value for key, value in params.items() if key not in {"p", "t", "s"}}
        result = HTTPResult(
            request_id=request_id,
            protocol_method=protocol_method,
            http_method=http_method.upper(),
            url=self.base_url + path,
            params=safe_params,
            status=response.status,
            elapsed_ms=elapsed_ms,
            content_type=content_type,
            headers=response_headers,
            body_size=len(payload),
            body_truncated=truncated,
            artifact=str(artifact_path),
            body=payload,
        )
        self.requests.append(result)
        return result


def parse_json_response(response: HTTPResult) -> dict[str, Any]:
    require(response.status == 200, f"HTTP {response.status}")
    try:
        payload = json.loads(response.body.decode("utf-8-sig"))
    except Exception as error:
        raise AssertionError(f"invalid JSON response: {error}") from error
    root = payload.get("subsonic-response") if isinstance(payload, dict) else None
    require(isinstance(root, dict), "missing JSON subsonic-response root")
    return root


def parse_xml_response(response: HTTPResult) -> ET.Element:
    require(response.status == 200, f"HTTP {response.status}")
    try:
        root = ET.fromstring(response.body)
    except Exception as error:
        raise AssertionError(f"invalid XML response: {error}") from error
    require(root.tag.split("}")[-1] == "subsonic-response", "missing XML subsonic-response root")
    return root


def protocol_error(response: HTTPResult) -> tuple[int | None, str]:
    try:
        if "json" in response.content_type:
            root = parse_json_response(response)
            error = root.get("error") or {}
            code = error.get("code")
            return (int(code) if code is not None else None, str(error.get("message") or ""))
        root = parse_xml_response(response)
        for child in root:
            if child.tag.split("}")[-1] == "error":
                raw = child.attrib.get("code")
                return (int(raw) if raw else None, child.attrib.get("message", ""))
    except Exception:
        pass
    return None, ""


def expect_ok(response: HTTPResult, child: str | None = None) -> dict[str, Any]:
    root = parse_json_response(response)
    if root.get("status") != "ok":
        error = root.get("error") or {}
        raise AssertionError(f"Subsonic {error.get('code')}: {error.get('message') or 'request failed'}")
    if child is not None:
        require(child in root, f"successful response is missing '{child}'")
    return root


def expect_xml_ok(response: HTTPResult) -> ET.Element:
    root = parse_xml_response(response)
    require(root.attrib.get("status") == "ok", f"Subsonic XML status={root.attrib.get('status')}")
    return root


class Runner:
    def __init__(self, args: argparse.Namespace) -> None:
        run_stamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        self.run_id = f"{run_stamp}-{secrets.token_hex(4)}"
        self.started_at = utc_now()
        self.output_dir = Path(args.output).expanduser().resolve() / self.run_id
        self.response_dir = self.output_dir / "responses"
        self.response_dir.mkdir(parents=True, exist_ok=False)
        self.client = Client(args.url, args.username, args.password, self.run_id, self.response_dir, args.timeout)
        self.args = args
        self.results: list[TestResult] = []
        self.context: dict[str, Any] = {
            "artist": None,
            "album": None,
            "song": None,
            "artists": [],
            "albums": [],
            "songs": [],
            "genre": None,
            "playlist": None,
            "starred_ids": set(),
        }
        self.created_playlist_id = ""
        self.created_playlist_deleted = False
        self.temporary_star_id = ""

    def run_test(
        self,
        name: str,
        category: str,
        methods: str | list[str],
        function: Callable[[], Any],
        *,
        implementation: str = "native",
    ) -> TestResult:
        method_list = [methods] if isinstance(methods, str) else methods
        request_start = len(self.client.requests)
        started = time.monotonic()
        status, detail = "PASS", ""
        error_code: int | None = None
        error_message = ""
        try:
            function()
        except SkipTest as error:
            status, detail = "SKIP", str(error)
        except Exception as error:
            status = "FAIL"
            detail = str(error) or error.__class__.__name__
            if self.args.verbose and not isinstance(error, AssertionError):
                detail += "\n" + traceback.format_exc(limit=3)
        requests = self.client.requests[request_start:]
        if status == "FAIL" and requests:
            error_code, error_message = protocol_error(requests[-1])
        result = TestResult(
            name=name,
            category=category,
            methods=method_list,
            implementation=implementation,
            status=status,
            elapsed_ms=round((time.monotonic() - started) * 1000),
            detail=detail,
            protocol_error_code=error_code,
            protocol_error_message=error_message,
            http_statuses=[request.status for request in requests],
            artifacts=[str(Path(request.artifact).relative_to(self.output_dir)) for request in requests],
        )
        self.results.append(result)
        if self.args.verbose or status != "PASS":
            suffix = f": {detail.splitlines()[0]}" if detail else ""
            print(f"[{status}] {name}{suffix}", flush=True)
        return result

    def call_ok(
        self,
        method: str,
        params: dict[str, Any] | None = None,
        *,
        child: str | None = None,
        **kwargs: Any,
    ) -> dict[str, Any]:
        return expect_ok(self.client.call(method, params, **kwargs), child)

    def require_id(self, key: str) -> dict[str, Any]:
        value = self.context.get(key)
        if not isinstance(value, dict) or not value.get("id"):
            raise SkipTest(f"曲库中未发现可用于测试的 {key} ID")
        return value

    def remember_entities(self, key: str, values: Any) -> None:
        target = self.context[key]
        known = {str(item.get("id")) for item in target if isinstance(item, dict)}
        for value in listify(values):
            if not isinstance(value, dict) or not value.get("id") or str(value["id"]) in known:
                continue
            target.append(value)
            known.add(str(value["id"]))

    def capture_artists(self, root: dict[str, Any]) -> None:
        artists = root.get("artists") or root.get("indexes") or {}
        for index in listify(artists.get("index") if isinstance(artists, dict) else None):
            if isinstance(index, dict):
                values = listify(index.get("artist"))
                self.remember_entities("artists", values)
                if not self.context.get("artist"):
                    candidate = first_dict(values)
                    if candidate and candidate.get("id"):
                        self.context["artist"] = candidate

    def capture_albums(self, root: dict[str, Any], key: str = "albumList2") -> None:
        container = root.get(key) or {}
        values = container.get("album") if isinstance(container, dict) else None
        self.remember_entities("albums", values)
        candidate = first_dict(values)
        if candidate and candidate.get("id"):
            self.context["album"] = candidate

    def capture_songs(self, value: Any) -> None:
        self.remember_entities("songs", value)
        candidate = first_dict(value)
        if candidate and candidate.get("id"):
            self.context["song"] = candidate
            if not self.context.get("album") and candidate.get("albumId"):
                self.context["album"] = {"id": candidate["albumId"], "name": candidate.get("album", "")}
            if not self.context.get("artist") and candidate.get("artistId"):
                self.context["artist"] = {"id": candidate["artistId"], "name": candidate.get("artist", "")}

    def execute(self) -> None:
        self.transport_tests()
        self.identity_tests()
        self.catalog_tests()
        self.dependent_catalog_tests()
        self.media_tests()
        self.placeholder_tests()
        self.mutation_tests()
        self.error_contract_tests()

    def transport_tests(self) -> None:
        def landing() -> None:
            response = self.client.raw("/")
            require(response.status == 200, f"HTTP {response.status}")
            payload = json.loads(response.body)
            require(payload.get("name") == "Yinqiao Service", "unexpected Yinqiao landing payload")

        def status() -> None:
            response = self.client.raw("/yinqiao/status")
            require(response.status == 200, f"HTTP {response.status}")
            payload = json.loads(response.body)
            require(payload.get("ok") is True, f"bridge status is not ready: {payload}")

        self.run_test("Gateway landing", "transport", [], landing)
        self.run_test("Bridge compatibility status", "transport", [], status)
        self.run_test(
            "Public OpenSubsonic extensions (JSON POST)",
            "transport",
            "getOpenSubsonicExtensions",
            lambda: require(
                len(expect_ok(self.client.call("getOpenSubsonicExtensions", auth_mode="none"), "openSubsonicExtensions")["openSubsonicExtensions"]) >= 2,
                "expected advertised extensions",
            ),
        )
        self.run_test(
            "Public OpenSubsonic extensions (XML GET)",
            "transport",
            "getOpenSubsonicExtensions",
            lambda: expect_xml_ok(self.client.call("getOpenSubsonicExtensions", auth_mode="none", response_format="xml", http_method="GET")),
        )
        self.run_test("Token authentication + JSON form POST", "authentication", "ping", lambda: self.call_ok("ping"))
        self.run_test(
            "Plain password authentication",
            "authentication",
            "ping",
            lambda: self.call_ok("ping", auth_mode="password"),
        )
        self.run_test(
            "enc: password authentication + XML GET",
            "authentication",
            "ping",
            lambda: expect_xml_ok(self.client.call("ping", auth_mode="enc", response_format="xml", http_method="GET")),
        )

        def jsonp() -> None:
            response = self.client.call("ping", {"callback": "qa.callback"}, response_format="jsonp")
            require(response.status == 200, f"HTTP {response.status}")
            require(response.body.startswith(b"qa.callback("), "invalid JSONP callback response")

        self.run_test("JSONP response encoding", "transport", "ping", jsonp)

    def identity_tests(self) -> None:
        self.run_test("License", "identity", "getLicense", lambda: self.call_ok("getLicense", child="license"))
        self.run_test("Music folders", "identity", "getMusicFolders", lambda: self.call_ok("getMusicFolders", child="musicFolders"))
        self.run_test(
            "Current user and roles",
            "identity",
            "getUser",
            lambda: self.call_ok("getUser", {"username": self.args.username}, child="user"),
        )

    def catalog_tests(self) -> None:
        def artists() -> None:
            root = self.call_ok("getArtists", child="artists")
            self.capture_artists(root)

        def indexes() -> None:
            root = self.call_ok("getIndexes", child="indexes")
            self.capture_artists(root)

        self.run_test("Artists", "catalog", "getArtists", artists)
        self.run_test("Indexes", "catalog", "getIndexes", indexes)

        album_types = ["newest", "recent", "frequent", "highest", "starred", "random", "alphabeticalByName", "alphabeticalByArtist"]
        for album_type in album_types:
            def album_list(current_type: str = album_type) -> None:
                root = self.call_ok("getAlbumList2", {"type": current_type, "size": 20}, child="albumList2")
                self.capture_albums(root)

            self.run_test(f"Album list type={album_type}", "catalog", "getAlbumList2", album_list)

        def legacy_album_list() -> None:
            root = self.call_ok("getAlbumList", {"type": "newest", "size": 20}, child="albumList")
            self.capture_albums(root, "albumList")

        self.run_test("Legacy album list", "catalog", "getAlbumList", legacy_album_list)

        def genres() -> None:
            root = self.call_ok("getGenres", child="genres")
            candidate = first_dict((root.get("genres") or {}).get("genre"))
            if candidate:
                self.context["genre"] = candidate

        self.run_test("Genres", "catalog", "getGenres", genres)

        def random_songs() -> None:
            root = self.call_ok("getRandomSongs", {"size": 20}, child="randomSongs")
            self.capture_songs((root.get("randomSongs") or {}).get("song"))

        self.run_test("Random songs", "catalog", "getRandomSongs", random_songs)

        def starred(method: str, key: str) -> None:
            root = self.call_ok(method, {"size": 100}, child=key)
            songs = listify((root.get(key) or {}).get("song"))
            self.context["starred_ids"] = {str(song.get("id")) for song in songs if isinstance(song, dict) and song.get("id")}
            self.capture_songs(songs)

        self.run_test("Legacy starred", "catalog", "getStarred", lambda: starred("getStarred", "starred"))
        self.run_test("Starred2", "catalog", "getStarred2", lambda: starred("getStarred2", "starred2"))

        def playlists() -> None:
            root = self.call_ok("getPlaylists", child="playlists")
            candidate = first_dict((root.get("playlists") or {}).get("playlist"))
            if candidate and candidate.get("id"):
                self.context["playlist"] = candidate

        self.run_test("Playlists", "playlist", "getPlaylists", playlists)

        def search(method: str, query: str) -> None:
            key = "searchResult2" if method == "search2" else "searchResult3"
            limits = {"artist": 10, "album": 10, "song": 20}
            root = self.call_ok(method, {"query": query, "artistCount": limits["artist"], "albumCount": limits["album"], "songCount": limits["song"]}, child=key)
            result = root.get(key) or {}
            for entity_type, limit in limits.items():
                values = listify(result.get(entity_type))
                require(len(values) <= limit, f"{method} returned {len(values)} {entity_type} items, requested at most {limit}")
                if query and query != "*":
                    needle = normalized_search_text(query)
                    unrelated = []
                    for value in values:
                        if not isinstance(value, dict):
                            unrelated.append(repr(value))
                            continue
                        haystack = normalized_search_text(" ".join(str(value.get(field) or "") for field in ["name", "title", "artist", "album"]))
                        if needle not in haystack:
                            unrelated.append(str(value.get("name") or value.get("title") or value.get("id")))
                    require(not unrelated, f"{method} returned unrelated {entity_type} items for {query!r}: {unrelated[:5]}")
            self.capture_songs(result.get("song"))
            self.remember_entities("artists", result.get("artist"))
            self.remember_entities("albums", result.get("album"))
            artist = first_dict(result.get("artist"))
            album = first_dict(result.get("album"))
            if artist and artist.get("id"):
                self.context["artist"] = artist
            if album and album.get("id"):
                self.context["album"] = album

        self.run_test("Search3 empty browse", "search", "search3", lambda: search("search3", ""))
        term = str((self.context.get("song") or {}).get("title") or "music")[:80]
        self.run_test("Search3 text query", "search", "search3", lambda: search("search3", term))
        self.run_test("Search2 text query", "search", "search2", lambda: search("search2", term))

    def dependent_catalog_tests(self) -> None:
        def artist_detail() -> None:
            artist = self.require_id("artist")
            root = self.call_ok("getArtist", {"id": artist["id"]}, child="artist")
            albums = (root.get("artist") or {}).get("album")
            candidate = first_dict(albums)
            if candidate and candidate.get("id"):
                self.context["album"] = candidate

        self.run_test("Artist detail", "catalog", "getArtist", artist_detail)

        def album_detail() -> None:
            album = self.require_id("album")
            root = self.call_ok("getAlbum", {"id": album["id"]}, child="album")
            self.capture_songs((root.get("album") or {}).get("song"))

        self.run_test("Album detail and tracks", "catalog", "getAlbum", album_detail)

        def song_detail() -> None:
            song = self.require_id("song")
            root = self.call_ok("getSong", {"id": song["id"]}, child="song")
            value = root.get("song")
            require(isinstance(value, dict) and value.get("id") == song["id"], "getSong returned a different or empty song")
            self.context["song"] = value

        self.run_test("Song lookup by ID", "catalog", "getSong", song_detail)
        self.run_test(
            "Music directory root",
            "catalog",
            "getMusicDirectory",
            lambda: self.call_ok("getMusicDirectory", {"id": "fnmusic"}, child="directory"),
        )
        self.run_test(
            "Music directory album children",
            "catalog",
            "getMusicDirectory",
            lambda: self.call_ok("getMusicDirectory", {"id": self.require_id("album")["id"]}, child="directory"),
        )

        def songs_by_genre() -> None:
            genre = self.context.get("genre")
            if not isinstance(genre, dict):
                raise SkipTest("曲库没有可用于测试的 genre")
            genre_name = genre.get("value") or genre.get("name")
            if not genre_name:
                raise SkipTest("genre 响应没有名称")
            self.call_ok("getSongsByGenre", {"genre": genre_name, "count": 20}, child="songsByGenre")

        self.run_test("Songs by genre", "catalog", "getSongsByGenre", songs_by_genre)
        self.run_test(
            "Existing playlist detail",
            "playlist",
            "getPlaylist",
            lambda: self.call_ok("getPlaylist", {"id": self.require_id("playlist")["id"]}, child="playlist"),
        )

    def media_tests(self) -> None:
        def binary(method: str, expected: str) -> None:
            song = self.require_id("song")
            response = self.client.call(
                method,
                {"id": song["id"]},
                http_method="GET",
                response_format="",
                binary=True,
                headers={"Range": "bytes=0-4095"},
            )
            require(response.status in {200, 206}, f"HTTP {response.status}")
            require(response.body_size > 0, "empty media response")
            require(expected in response.content_type.lower() or "octet-stream" in response.content_type.lower(), f"unexpected content type {response.content_type!r}")
            if response.status == 206:
                require(bool(response.headers.get("Content-Range")), "206 response is missing Content-Range")

        self.run_test("Audio stream with Range", "media", "stream", lambda: binary("stream", "audio"))
        self.run_test("Audio download with Range", "media", "download", lambda: binary("download", "audio"))

        def cover() -> None:
            song = self.require_id("song")
            cover_id = song.get("coverArt") or song.get("albumId") or song.get("id")
            response = self.client.call(
                "getCoverArt",
                {"id": cover_id, "size": 300},
                http_method="GET",
                response_format="",
                binary=True,
                headers={"Range": "bytes=0-65535"},
            )
            require(response.status in {200, 206}, f"HTTP {response.status}")
            require(response.body_size > 0, "empty cover response")
            require("image/" in response.content_type.lower(), f"unexpected content type {response.content_type!r}")

        self.run_test("Cover art", "media", "getCoverArt", cover)

        cover_samples: list[tuple[str, dict[str, Any]]] = []
        for entity_type, key in [("artist", "artists"), ("album", "albums"), ("song", "songs")]:
            seen: set[str] = set()
            for entity in self.context[key]:
                cover_id = str(entity.get("coverArt") or "")
                if not cover_id or cover_id in seen:
                    continue
                seen.add(cover_id)
                cover_samples.append((entity_type, entity))
                if len(seen) >= 6:
                    break

        for entity_type, entity in cover_samples:
            def sampled_cover(current_type: str = entity_type, current: dict[str, Any] = entity) -> None:
                cover_id = str(current["coverArt"])
                response = self.client.call(
                    "getCoverArt",
                    {"id": cover_id, "size": 96},
                    http_method="GET",
                    response_format="",
                    binary=True,
                    headers={"Range": "bytes=0-65535"},
                )
                require(response.status in {200, 206}, f"{current_type} {current.get('id')} cover {cover_id}: HTTP {response.status}")
                require(response.body_size > 0 and "image/" in response.content_type.lower(), f"{current_type} {current.get('id')} cover {cover_id}: invalid {response.content_type!r} response")

            self.run_test(
                f"Advertised {entity_type} cover {entity.get('coverArt')}",
                "media-sample",
                "getCoverArt",
                sampled_cover,
            )

        for entity_type, key in [("artist", "artists"), ("album", "albums"), ("song", "songs")]:
            for entity in self.context[key][:3]:
                if not entity.get("coverArt"):
                    continue

                def entity_cover(current_type: str = entity_type, current: dict[str, Any] = entity) -> None:
                    response = self.client.call(
                        "getCoverArt",
                        {"id": current["id"], "size": 96},
                        http_method="GET",
                        response_format="",
                        binary=True,
                        headers={"Range": "bytes=0-65535"},
                    )
                    require(response.status in {200, 206}, f"{current_type} ID {current['id']}: HTTP {response.status}")
                    require(response.body_size > 0 and "image/" in response.content_type.lower(), f"{current_type} ID {current['id']}: invalid {response.content_type!r} response")

                self.run_test(
                    f"Resolve {entity_type} ID to cover {entity.get('id')}",
                    "media-resolution",
                    "getCoverArt",
                    entity_cover,
                )

        def legacy_lyrics() -> None:
            song = self.require_id("song")
            root = self.call_ok("getLyrics", {"title": song.get("title", ""), "artist": song.get("artist", "")}, child="lyrics")
            value = str((root.get("lyrics") or {}).get("value") or "").strip()
            if not value:
                raise SkipTest(f"曲目 {song.get('title')!r} 没有可验证的歌词内容")

        self.run_test("Legacy lyrics", "lyrics", "getLyrics", legacy_lyrics)
        def structured_lyrics() -> None:
            root = self.call_ok("getLyricsBySongId", {"id": self.require_id("song")["id"]}, child="lyricsList")
            sets = listify((root.get("lyricsList") or {}).get("structuredLyrics"))
            lines = [line for item in sets if isinstance(item, dict) for line in listify(item.get("line")) if isinstance(line, dict) and str(line.get("value") or "").strip()]
            if not lines:
                raise SkipTest("接口成功但没有非空歌词行")

        self.run_test("Structured lyrics by song ID", "lyrics", "getLyricsBySongId", structured_lyrics)
        self.run_test("Library scan status", "admin", "getScanStatus", lambda: self.call_ok("getScanStatus", child="scanStatus"))

    def placeholder_tests(self) -> None:
        parameter_map: dict[str, dict[str, Any]] = {
            "getChatMessages": {"since": 0},
            "getSimilarSongs": {"id": lambda: self.require_id("song")["id"], "count": 10},
            "getSimilarSongs2": {"id": lambda: self.require_id("song")["id"], "count": 10},
            "getTopSongs": {"artist": lambda: self.require_id("artist").get("name", "")},
            "getArtistInfo": {"id": lambda: self.require_id("artist")["id"]},
            "getArtistInfo2": {"id": lambda: self.require_id("artist")["id"]},
            "getAlbumInfo": {"id": lambda: self.require_id("album")["id"]},
            "getAlbumInfo2": {"id": lambda: self.require_id("album")["id"]},
            "setRating": {"id": lambda: self.require_id("song")["id"], "rating": 3},
            "savePlayQueue": {"id": lambda: [self.require_id("song")["id"]], "current": lambda: self.require_id("song")["id"], "position": 0},
            "createBookmark": {"id": lambda: self.require_id("song")["id"], "position": 1000, "comment": self.run_id},
            "deleteBookmark": {"id": lambda: self.require_id("song")["id"]},
        }
        ordered = [method for method in SUPPORTED_METHODS if method in PLACEHOLDER_METHODS]
        for method in ordered:
            def invoke(current_method: str = method) -> None:
                params: dict[str, Any] = {}
                for key, value in parameter_map.get(current_method, {}).items():
                    params[key] = value() if callable(value) else value
                self.call_ok(current_method, params)

            self.run_test(
                f"Compatibility placeholder: {method}",
                "compatibility-placeholder",
                method,
                invoke,
                implementation="placeholder",
            )

    def mutation_tests(self) -> None:
        song = self.context.get("song") if isinstance(self.context.get("song"), dict) else None
        if not self.args.mutations:
            for method in ["createPlaylist", "updatePlaylist", "deletePlaylist", "star", "unstar", "scrobble"]:
                self.run_test(
                    f"Mutation disabled: {method}",
                    "mutation",
                    method,
                    lambda: (_ for _ in ()).throw(SkipTest("需要 --mutations 才会写入真实账户状态")),
                )
        else:
            playlist_name = f"yinqiao-autotest-{self.run_id}"

            def create_playlist() -> None:
                root = self.call_ok("createPlaylist", {"name": playlist_name}, child="playlist")
                playlist = root.get("playlist") or {}
                require(bool(playlist.get("id")), "created playlist has no ID")
                self.created_playlist_id = str(playlist["id"])

            create_result = self.run_test("Create disposable playlist", "mutation", "createPlaylist", create_playlist)

            def update_playlist() -> None:
                if not self.created_playlist_id:
                    raise SkipTest("临时歌单创建失败")
                renamed = playlist_name + "-updated"
                self.call_ok("updatePlaylist", {"playlistId": self.created_playlist_id, "name": renamed})
                if song and song.get("id"):
                    self.call_ok("updatePlaylist", {"playlistId": self.created_playlist_id, "songIdToAdd": [song["id"]]})
                    self.call_ok("updatePlaylist", {"playlistId": self.created_playlist_id, "songIndexToRemove": [0]})
                root = self.call_ok("getPlaylist", {"id": self.created_playlist_id}, child="playlist")
                require((root.get("playlist") or {}).get("name") == renamed, "playlist rename was not persisted")

            if create_result.status == "PASS":
                self.run_test("Update disposable playlist", "mutation", ["updatePlaylist", "getPlaylist"], update_playlist)
            else:
                self.run_test("Update disposable playlist", "mutation", "updatePlaylist", lambda: (_ for _ in ()).throw(SkipTest("临时歌单创建失败")))

            def delete_playlist() -> None:
                if not self.created_playlist_id:
                    raise SkipTest("临时歌单未创建")
                self.call_ok("deletePlaylist", {"id": self.created_playlist_id})
                self.created_playlist_deleted = True

            if self.args.keep_playlist:
                self.run_test("Delete disposable playlist", "mutation", "deletePlaylist", lambda: (_ for _ in ()).throw(SkipTest("--keep-playlist 已要求保留")))
            else:
                self.run_test("Delete disposable playlist", "mutation", "deletePlaylist", delete_playlist)

            starred_ids = self.context.get("starred_ids") or set()
            if song and song.get("id") and song["id"] not in starred_ids:
                song_id = str(song["id"])

                def star() -> None:
                    self.call_ok("star", {"id": [song_id]})
                    self.temporary_star_id = song_id

                self.run_test("Star disposable song state", "mutation", "star", star)

                def unstar() -> None:
                    if not self.temporary_star_id:
                        raise SkipTest("临时收藏没有成功创建")
                    self.call_ok("unstar", {"id": [song_id]})
                    self.temporary_star_id = ""

                self.run_test("Restore disposable song state", "mutation", "unstar", unstar)
            else:
                reason = "没有曲目" if not song else "选中的曲目原本已收藏，避免改变原状态"
                for method in ["star", "unstar"]:
                    self.run_test(f"Disposable favorite: {method}", "mutation", method, lambda r=reason: (_ for _ in ()).throw(SkipTest(r)))

            self.run_test(
                "Scrobble playback event",
                "mutation",
                "scrobble",
                lambda: self.call_ok("scrobble", {"id": [self.require_id("song")["id"]], "submission": "true", "time": [int(time.time() * 1000)]}),
            )

        if self.args.admin_actions:
            self.run_test("Start full library scan", "admin-mutation", "startScan", lambda: self.call_ok("startScan", child="scanStatus"))
        else:
            self.run_test(
                "Start full library scan",
                "admin-mutation",
                "startScan",
                lambda: (_ for _ in ()).throw(SkipTest("危险管理操作，需要 --admin-actions")),
            )

    def error_contract_tests(self) -> None:
        def unauthenticated() -> None:
            response = self.client.call("ping", auth_mode="none")
            root = parse_json_response(response)
            require(root.get("status") == "failed", "unauthenticated ping unexpectedly succeeded")
            require(int((root.get("error") or {}).get("code", -1)) in {10, 40, 41}, f"unexpected auth error: {root.get('error')}")

        def unknown_method() -> None:
            response = self.client.call("fnmusicAutotestUnknown")
            root = parse_json_response(response)
            require(root.get("status") == "failed", "unknown method unexpectedly succeeded")
            require(int((root.get("error") or {}).get("code", -1)) == 70, f"unexpected error: {root.get('error')}")

        self.run_test("Unauthenticated request rejection", "error-contract", "ping", unauthenticated)
        self.run_test("Unknown method rejection", "error-contract", [], unknown_method)

    def emergency_cleanup(self) -> None:
        if self.created_playlist_id and not self.created_playlist_deleted and not self.args.keep_playlist:
            self.run_test(
                "Emergency playlist cleanup",
                "cleanup",
                "deletePlaylist",
                lambda: self.call_ok("deletePlaylist", {"id": self.created_playlist_id}),
            )
        if self.temporary_star_id:
            temporary_id = self.temporary_star_id
            self.run_test(
                "Emergency favorite cleanup",
                "cleanup",
                "unstar",
                lambda: self.call_ok("unstar", {"id": [temporary_id]}),
            )

    def write_reports(self) -> None:
        ended_at = utc_now()
        method_status: dict[str, str] = {}
        severity = {"PASS": 0, "SKIP": 1, "FAIL": 2}
        for method in SUPPORTED_METHODS:
            matches = [result.status for result in self.results if method in result.methods]
            method_status[method] = max(matches, key=lambda item: severity[item]) if matches else "UNTESTED"
        counts = {status: sum(1 for result in self.results if result.status == status) for status in ["PASS", "FAIL", "SKIP"]}
        method_counts = {status: sum(1 for value in method_status.values() if value == status) for status in ["PASS", "FAIL", "SKIP", "UNTESTED"]}
        request_payload = []
        for request in self.client.requests:
            value = asdict(request)
            value.pop("body", None)
            value["artifact"] = str(Path(request.artifact).relative_to(self.output_dir))
            request_payload.append(value)
        payload = {
            "run_id": self.run_id,
            "started_at": self.started_at,
            "ended_at": ended_at,
            "base_url": self.client.base_url,
            "username": self.args.username,
            "client_name": self.client.client_name,
            "protocol_version": PROTOCOL_VERSION,
            "options": {"mutations": self.args.mutations, "admin_actions": self.args.admin_actions, "timeout": self.args.timeout},
            "summary": {"tests": counts, "methods": method_counts, "http_requests": len(self.client.requests)},
            "method_status": method_status,
            "discovery": {key: sorted(value) if isinstance(value, set) else value for key, value in self.context.items()},
            "tests": [asdict(result) for result in self.results],
            "requests": request_payload,
        }
        (self.output_dir / "report.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")

        lines = [
            "# fnmusic Subsonic 自动化测试报告",
            "",
            f"- Run ID: `{self.run_id}`",
            f"- 地址: `{self.client.base_url}`",
            f"- 账号: `{self.args.username}`",
            f"- 时间: `{self.started_at}` 至 `{ended_at}`",
            f"- 客户端标识: `{self.client.client_name}`",
            f"- 参数: mutations=`{self.args.mutations}`, admin-actions=`{self.args.admin_actions}`",
            "",
            "## 摘要",
            "",
            f"测试项：PASS {counts['PASS']} / FAIL {counts['FAIL']} / SKIP {counts['SKIP']}；HTTP 请求 {len(self.client.requests)} 次。",
            "",
            f"公开方法：PASS {method_counts['PASS']} / FAIL {method_counts['FAIL']} / SKIP {method_counts['SKIP']} / UNTESTED {method_counts['UNTESTED']}，共 {len(SUPPORTED_METHODS)} 个。",
            "",
            "## 失败项",
            "",
        ]
        failures = [result for result in self.results if result.status == "FAIL"]
        if not failures:
            lines.append("没有失败项。")
        for index, result in enumerate(failures, 1):
            lines.extend([
                f"### {index}. {result.name}",
                "",
                f"- 方法: `{', '.join(result.methods) or '非协议端点'}`",
                f"- HTTP: `{result.http_statuses}`",
                f"- Subsonic 错误: `{result.protocol_error_code}` {result.protocol_error_message}",
                f"- 现象: {result.detail}",
                f"- 证据: {', '.join(f'`{item}`' for item in result.artifacts) or '无响应文件'}",
                "",
            ])
        lines.extend(["## 方法覆盖", "", "| 方法 | 实现类型 | 结果 |", "| --- | --- | --- |"])
        for method in SUPPORTED_METHODS:
            implementation = "兼容占位" if method in PLACEHOLDER_METHODS else "真实桥接"
            lines.append(f"| `{method}` | {implementation} | {method_status[method]} |")
        lines.extend(["", "## 全部测试项", "", "| 结果 | 分类 | 测试 | 方法 | 耗时 |", "| --- | --- | --- | --- | ---: |"])
        for result in self.results:
            detail = result.detail.replace("|", "\\|").replace("\n", " ")
            label = result.name + (f" - {detail}" if detail else "")
            lines.append(f"| {result.status} | {result.category} | {label} | `{', '.join(result.methods)}` | {result.elapsed_ms} ms |")
        lines.extend([
            "",
            "## 复测命令",
            "",
            "```bash",
            f"SUBSONIC_URL='{self.client.base_url}' SUBSONIC_USER='{self.args.username}' SUBSONIC_PASSWORD='<password>' ./run.sh" + (" --mutations" if self.args.mutations else "") + (" --admin-actions" if self.args.admin_actions else ""),
            "```",
            "",
        ])
        (self.output_dir / "report.md").write_text("\n".join(lines), encoding="utf-8")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default=os.environ.get("SUBSONIC_URL"), required=not os.environ.get("SUBSONIC_URL"))
    parser.add_argument("--username", default=os.environ.get("SUBSONIC_USER"), required=not os.environ.get("SUBSONIC_USER"))
    parser.add_argument("--password", default=os.environ.get("SUBSONIC_PASSWORD"), required=not os.environ.get("SUBSONIC_PASSWORD"))
    parser.add_argument("--timeout", type=float, default=30.0)
    parser.add_argument("--output", default=str(Path(__file__).resolve().parent / "reports"))
    parser.add_argument("--mutations", action="store_true", help="test reversible account mutations and scrobble")
    parser.add_argument("--admin-actions", action="store_true", help="allow startScan (triggers a real library scan)")
    parser.add_argument("--keep-playlist", action="store_true", help="do not delete the disposable test playlist")
    parser.add_argument("--verbose", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    runner = Runner(args)
    print(f"Run ID: {runner.run_id}")
    print(f"Target: {runner.client.base_url}")
    try:
        runner.execute()
    finally:
        runner.emergency_cleanup()
        runner.write_reports()
    failures = sum(result.status == "FAIL" for result in runner.results)
    print(f"Report: {runner.output_dir / 'report.md'}")
    print(f"Result: {failures} failed, {sum(result.status == 'PASS' for result in runner.results)} passed, {sum(result.status == 'SKIP' for result in runner.results)} skipped")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
