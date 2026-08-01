#!/usr/bin/env python3
"""Evidence-based black-box audit client for Yinqiao Jellyfin Audio."""

from __future__ import annotations

import argparse
import json
import re
import secrets
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable


MAX_BODY = 256 * 1024
MEDIA_BODY = 4096
SENSITIVE_KEYS = {"accesstoken", "token", "password", "pw", "api_key", "apikey"}


@dataclass
class Response:
    method: str
    path: str
    status: int
    elapsed_ms: int
    content_type: str
    headers: dict[str, str]
    body: bytes
    json_data: Any
    artifact: str


@dataclass
class Result:
    name: str
    category: str
    method: str
    path: str
    outcome: str
    status: int | None
    elapsed_ms: int
    detail: str
    artifact: str


def now_iso() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def slug(value: str) -> str:
    return re.sub(r"[^A-Za-z0-9_.-]+", "-", value).strip("-") or "request"


def redact(value: Any) -> Any:
    if isinstance(value, dict):
        return {
            key: "[redacted]" if key.casefold() in SENSITIVE_KEYS else redact(item)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [redact(item) for item in value]
    return value


def items(payload: Any) -> list[dict[str, Any]]:
    if not isinstance(payload, dict):
        return []
    values = payload.get("Items", [])
    return [value for value in values if isinstance(value, dict)] if isinstance(values, list) else []


class Client:
    def __init__(self, base_url: str, username: str, password: str, output: Path, timeout: float) -> None:
        self.base_url = base_url.rstrip("/")
        self.username = username
        self.password = password
        self.output = output
        self.timeout = timeout
        self.token = ""
        self.user_id = ""
        self.counter = 0

    def request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, Any] | None = None,
        payload: dict[str, Any] | None = None,
        auth: bool = True,
        headers: dict[str, str] | None = None,
        media: bool = False,
    ) -> Response:
        self.counter += 1
        query = urllib.parse.urlencode(params or {}, doseq=True)
        request_path = path + (("?" + query) if query else "")
        data = None
        request_headers = {
            "Accept": "application/json",
            "X-Emby-Authorization": 'MediaBrowser Client="Yinqiao Audit", Device="CLI", DeviceId="yinqiao-audit", Version="1.0"',
        }
        if auth and self.token:
            request_headers["X-Emby-Token"] = self.token
        if payload is not None:
            data = json.dumps(payload, ensure_ascii=False).encode()
            request_headers["Content-Type"] = "application/json"
        if headers:
            request_headers.update(headers)
        request = urllib.request.Request(self.base_url + request_path, data=data, headers=request_headers, method=method)
        started = time.monotonic()
        try:
            response = urllib.request.urlopen(request, timeout=self.timeout)
        except urllib.error.HTTPError as error:
            response = error
        except Exception as error:
            elapsed = round((time.monotonic() - started) * 1000)
            raise RuntimeError(f"request failed after {elapsed} ms: {error}") from error
        limit = MEDIA_BODY if media else MAX_BODY
        body = response.read(limit + 1)[:limit]
        elapsed = round((time.monotonic() - started) * 1000)
        content_type = response.headers.get("Content-Type", "")
        parsed: Any = None
        if body and ("json" in content_type.casefold() or body[:1] in (b"{", b"[")):
            try:
                parsed = json.loads(body)
            except json.JSONDecodeError:
                parsed = None
        safe_path = path
        safe_payload = redact(parsed) if parsed is not None else None
        extension = ".json" if safe_payload is not None else ".bin"
        artifact = f"responses/{self.counter:03d}-{slug(method + '-' + safe_path)}{extension}"
        target = self.output / artifact
        if safe_payload is not None:
            target.write_text(json.dumps(safe_payload, ensure_ascii=False, indent=2), encoding="utf-8")
        else:
            target.write_bytes(body)
        return Response(
            method=method,
            path=request_path,
            status=response.status,
            elapsed_ms=elapsed,
            content_type=content_type,
            headers={key: value for key, value in response.headers.items()},
            body=body,
            json_data=parsed,
            artifact=artifact,
        )

    def login(self, path: str = "/Users/AuthenticateByName") -> Response:
        response = self.request(
            "POST",
            path,
            payload={"Username": self.username, "Pw": self.password},
            auth=False,
        )
        if response.status == 200 and isinstance(response.json_data, dict):
            self.token = str(response.json_data.get("AccessToken", ""))
            user = response.json_data.get("User", {})
            if isinstance(user, dict):
                self.user_id = str(user.get("Id", ""))
        return response


class Audit:
    def __init__(self, client: Client, mutations: bool) -> None:
        self.client = client
        self.mutations = mutations
        self.results: list[Result] = []
        self.tracks: list[dict[str, Any]] = []
        self.track: dict[str, Any] | None = None
        self.album: dict[str, Any] | None = None
        self.artist: dict[str, Any] | None = None
        self.genre: dict[str, Any] | None = None
        self.playlist: dict[str, Any] | None = None

    def test(
        self,
        name: str,
        category: str,
        method: str,
        path: str,
        *,
        params: dict[str, Any] | None = None,
        payload: dict[str, Any] | None = None,
        auth: bool = True,
        headers: dict[str, str] | None = None,
        media: bool = False,
        statuses: tuple[int, ...] = (200,),
        validate: Callable[[Response], str] | None = None,
        unsupported: bool = False,
    ) -> Response | None:
        try:
            response = self.client.request(method, path, params=params, payload=payload, auth=auth, headers=headers, media=media)
            detail = validate(response) if validate else ""
            if response.status not in statuses:
                raise AssertionError(f"HTTP {response.status}, expected {statuses}; {detail}".rstrip("; "))
            outcome = "unsupported" if unsupported else "pass"
            self.results.append(Result(name, category, method, response.path, outcome, response.status, response.elapsed_ms, detail, response.artifact))
            return response
        except Exception as error:
            self.results.append(Result(name, category, method, path, "fail", None, 0, str(error), ""))
            return None

    def skip(self, name: str, category: str, method: str, path: str, reason: str) -> None:
        self.results.append(Result(name, category, method, path, "skip", None, 0, reason, ""))

    @staticmethod
    def collection_detail(response: Response) -> str:
        payload = response.json_data if isinstance(response.json_data, dict) else {}
        return f"items={len(items(payload))}, total={payload.get('TotalRecordCount')}, start={payload.get('StartIndex')}"

    @staticmethod
    def cross_page_detail(response: Response) -> str:
        payload = response.json_data if isinstance(response.json_data, dict) else {}
        values = items(payload)
        total = payload.get("TotalRecordCount")
        start = payload.get("StartIndex")
        if len(values) != 20 or not isinstance(total, int) or total <= 50 or start != 45:
            raise AssertionError(f"incomplete cross-page window: items={len(values)}, total={total}, start={start}")
        return f"items=20, total={total}, start=45"

    @staticmethod
    def media_range_detail(response: Response) -> str:
        content_range = response.headers.get("Content-Range", "")
        if response.status != 206 or len(response.body) != MEDIA_BODY or not content_range.startswith("bytes 0-4095/"):
            raise AssertionError(f"invalid Range response: status={response.status}, bytes={len(response.body)}, content-range={content_range}")
        return f"bytes={len(response.body)}, content-range={content_range}"

    def run(self) -> None:
        self.public_and_login()
        if not self.client.token:
            return
        self.discovery_and_users()
        self.library()
        self.dynamic_media()
        self.sessions_and_tasks()
        self.explicit_unsupported()
        if self.mutations:
            self.playlist_mutations()
        self.session_logout()

    def public_and_login(self) -> None:
        self.test("server root", "discovery", "GET", "/", auth=False)
        self.test("system ping GET", "discovery", "GET", "/System/Ping", auth=False, validate=lambda r: r.body.decode(errors="replace")[:80])
        self.test("system ping POST", "discovery", "POST", "/System/Ping", auth=False, validate=lambda r: r.body.decode(errors="replace")[:80])
        self.test("public system info", "discovery", "GET", "/System/Info/Public", auth=False, validate=lambda r: f"version={(r.json_data or {}).get('Version')}")
        self.test("mixed-case public info", "compatibility", "GET", "/jElLyFiN/sYsTeM/iNfO/pUbLiC", auth=False)
        self.test("branding", "discovery", "GET", "/Branding/Configuration", auth=False)
        self.test("quick connect disabled", "discovery", "GET", "/QuickConnect/Enabled", auth=False, validate=lambda r: f"value={r.json_data}")
        login = self.client.login()
        if login.status == 200 and self.client.token and self.client.user_id:
            self.results.append(Result("authenticate", "authentication", "POST", "/Users/AuthenticateByName", "pass", login.status, login.elapsed_ms, f"user_id={self.client.user_id}", login.artifact))
        else:
            self.results.append(Result("authenticate", "authentication", "POST", "/Users/AuthenticateByName", "fail", login.status, login.elapsed_ms, "missing AccessToken or User.Id", login.artifact))

    def discovery_and_users(self) -> None:
        self.test("system info", "discovery", "GET", "/System/Info")
        self.test("system endpoint", "discovery", "GET", "/System/Endpoint")
        self.test("users", "users", "GET", "/Users", validate=lambda r: f"users={len(r.json_data or [])}")
        self.test("public users hidden", "users", "GET", "/Users/Public", auth=False, validate=lambda r: f"users={len(r.json_data or [])}")
        self.test("current user", "users", "GET", "/Users/Me")
        self.test("user by id", "users", "GET", f"/Users/{self.client.user_id}")
        self.test("user views", "users", "GET", "/UserViews", validate=self.collection_detail)
        self.test("legacy user views", "users", "GET", f"/Users/{self.client.user_id}/Views", validate=self.collection_detail)
        self.test("view grouping options", "users", "GET", "/UserViews/GroupingOptions", validate=lambda r: f"options={len(r.json_data or [])}")

    def _capture_first(self, response: Response | None, attribute: str) -> None:
        values = items(response.json_data) if response else []
        if values:
            setattr(self, attribute, values[0])

    def library(self) -> None:
        self.test("item counts", "library", "GET", "/Items/Counts", validate=lambda r: json.dumps(r.json_data, ensure_ascii=False)[:300])
        self.test("item root", "library", "GET", "/Items/Root", validate=lambda r: f"type={(r.json_data or {}).get('Type')}")
        track = self.test("audio items", "library", "GET", "/Items", params={"IncludeItemTypes": "Audio", "Recursive": "true", "StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self._capture_first(track, "track")
        self.tracks = items(track.json_data) if track else []
        album = self.test("album items", "library", "GET", "/Items", params={"IncludeItemTypes": "MusicAlbum", "Recursive": "true", "StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self._capture_first(album, "album")
        artist = self.test("artist items", "library", "GET", "/Items", params={"IncludeItemTypes": "MusicArtist", "Recursive": "true", "StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self._capture_first(artist, "artist")
        genre = self.test("genre items", "library", "GET", "/Items", params={"IncludeItemTypes": "MusicGenre", "Recursive": "true", "StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self._capture_first(genre, "genre")
        playlist = self.test("playlist items", "library", "GET", "/Items", params={"IncludeItemTypes": "Playlist", "Recursive": "true", "StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self._capture_first(playlist, "playlist")
        self.test("standard cross-page items", "pagination", "GET", "/Items", params={"IncludeItemTypes": "Audio", "StartIndex": 45, "Limit": 20}, validate=self.cross_page_detail)
        self.test("lowercase cross-page items", "compatibility", "GET", "/AuDiO/EmBy/iTeMs", params={"includeitemtypes": "audio", "startindex": 45, "limit": 20}, validate=self.cross_page_detail)
        self.test("latest items", "library", "GET", "/Items/Latest", params={"IncludeItemTypes": "Audio", "Limit": 5}, validate=lambda r: f"items={len(r.json_data or [])}")
        self.test("suggestions", "library", "GET", "/Items/Suggestions", params={"UserId": self.client.user_id, "Limit": 5}, validate=self.collection_detail)
        self.test("artists", "library", "GET", "/Artists", params={"StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self.test("album artists", "library", "GET", "/Artists/AlbumArtists", params={"StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self.test("genres", "library", "GET", "/Genres", params={"StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self.test("music genres", "library", "GET", "/MusicGenres", params={"StartIndex": 0, "Limit": 5}, validate=self.collection_detail)
        self.test("search hints", "search", "GET", "/Search/Hints", params={"SearchTerm": "a", "Limit": 10}, validate=lambda r: f"hints={len((r.json_data or {}).get('SearchHints', []))}")
        self.test("playlists list", "playlists", "GET", "/Playlists", params={"UserId": self.client.user_id, "Limit": 5}, validate=self.collection_detail)

    def dynamic_media(self) -> None:
        if not self.track:
            self.skip("track details", "media", "GET", "/Items/{id}", "no track returned")
            return
        track_id = str(self.track.get("Id", ""))
        album_id = str((self.album or {}).get("Id", ""))
        artist_id = str((self.artist or {}).get("Id", ""))
        genre_id = str((self.genre or {}).get("Id", ""))
        self.test("track detail", "media", "GET", f"/Items/{track_id}", validate=lambda r: f"type={(r.json_data or {}).get('Type')}")
        self.test("legacy track detail", "media", "GET", f"/Users/{self.client.user_id}/Items/{track_id}")
        self.test("track user data", "users", "GET", f"/UserItems/{track_id}/UserData", params={"UserId": self.client.user_id}, validate=lambda r: f"favorite={(r.json_data or {}).get('IsFavorite')}")
        self.test("playback info GET", "playback", "GET", f"/Items/{track_id}/PlaybackInfo", params={"UserId": self.client.user_id})
        self.test("playback info POST", "playback", "POST", f"/Items/{track_id}/PlaybackInfo", params={"UserId": self.client.user_id})
        self.test("lyrics", "lyrics", "GET", f"/Audio/{track_id}/Lyrics", statuses=(200, 404))
        self.test("stream range", "playback", "GET", f"/Audio/{track_id}/stream", headers={"Range": "bytes=0-4095"}, media=True, statuses=(206,), validate=self.media_range_detail)
        self.test("stream container", "playback", "GET", f"/Audio/{track_id}/stream.mp3", headers={"Range": "bytes=0-4095"}, media=True, statuses=(200, 206))
        self.test("stream HEAD", "playback", "HEAD", f"/Audio/{track_id}/stream", media=True, statuses=(200, 206))
        self.test("stream container HEAD", "playback", "HEAD", f"/Audio/{track_id}/stream.mp3", media=True, statuses=(200, 206))
        self.test("universal audio", "playback", "GET", f"/Audio/{track_id}/universal", headers={"Range": "bytes=0-4095"}, media=True, statuses=(206,), validate=self.media_range_detail)
        self.test("universal audio HEAD", "playback", "HEAD", f"/Audio/{track_id}/universal", media=True, statuses=(200, 206))
        self.test("download", "playback", "GET", f"/Items/{track_id}/Download", headers={"Range": "bytes=0-4095"}, media=True, statuses=(206,), validate=self.media_range_detail)
        self.test("download HEAD", "playback", "HEAD", f"/Items/{track_id}/Download", media=True, statuses=(200, 206))
        self.test("file", "playback", "GET", f"/Items/{track_id}/File", headers={"Range": "bytes=0-4095"}, media=True, statuses=(206,), validate=self.media_range_detail)
        self.test("file HEAD", "playback", "HEAD", f"/Items/{track_id}/File", media=True, statuses=(200, 206))
        self.test("similar tracks", "unsupported", "GET", f"/Items/{track_id}/Similar", params={"Limit": 5}, statuses=(501,), unsupported=True)
        image_tags = self.track.get("ImageTags", {})
        if isinstance(image_tags, dict) and image_tags.get("Primary"):
            self.test("track image", "images", "GET", f"/Items/{track_id}/Images/Primary", media=True)
            self.test("track image HEAD", "images", "HEAD", f"/Items/{track_id}/Images/Primary", media=True)
        else:
            self.skip("track image", "images", "GET", f"/Items/{track_id}/Images/Primary", "track has no Primary image tag")
        for label, value in (("album", album_id), ("artist", artist_id), ("genre", genre_id)):
            if value:
                self.test(f"{label} detail", "library", "GET", f"/Items/{value}")
                expected_children = int(({"album": self.album, "artist": self.artist, "genre": self.genre}.get(label) or {}).get("ChildCount") or 0)
                def validate_children(response: Response, expected: int = expected_children) -> str:
                    detail = self.collection_detail(response)
                    if expected > 0 and not items(response.json_data):
                        raise AssertionError(f"parent declares ChildCount={expected}, but returned no tracks; {detail}")
                    return detail
                self.test(f"{label} children", "library", "GET", "/Items", params={"ParentId": value, "IncludeItemTypes": "Audio", "Limit": 5}, validate=validate_children)
        if self.artist and self.artist.get("Name"):
            self.test("artist by name", "library", "GET", "/Artists/" + urllib.parse.quote(str(self.artist["Name"]), safe=""))
        if self.genre and self.genre.get("Name"):
            encoded_genre = urllib.parse.quote(str(self.genre["Name"]), safe="")
            self.test("genre by name", "library", "GET", "/Genres/" + encoded_genre)
            self.test("music genre by name", "library", "GET", "/MusicGenres/" + encoded_genre)

    def sessions_and_tasks(self) -> None:
        self.test("session capabilities full", "sessions", "POST", "/Sessions/Capabilities/Full", statuses=(204,))
        self.test("session capabilities", "sessions", "POST", "/Sessions/Capabilities", statuses=(204,))
        self.test("session viewing", "sessions", "POST", "/Sessions/Viewing", payload={"ItemId": (self.track or {}).get("Id")}, statuses=(204,))
        self.test("playing", "sessions", "POST", "/Sessions/Playing", payload={"ItemId": (self.track or {}).get("Id")}, statuses=(204,))
        self.test("playing progress", "sessions", "POST", "/Sessions/Playing/Progress", payload={"ItemId": (self.track or {}).get("Id"), "PositionTicks": 10000000}, statuses=(204,))
        self.test("playing ping", "sessions", "POST", "/Sessions/Playing/Ping", statuses=(204,))
        tasks = self.test("scheduled tasks", "tasks", "GET", "/ScheduledTasks", validate=lambda r: f"tasks={len(r.json_data or [])}")
        task_values = tasks.json_data if tasks and isinstance(tasks.json_data, list) else []
        task_id = str(task_values[0].get("Id", "")) if task_values and isinstance(task_values[0], dict) else ""
        if task_id:
            self.test("scheduled task detail", "tasks", "GET", "/ScheduledTasks/" + urllib.parse.quote(task_id, safe=""))
        else:
            self.skip("scheduled task detail", "tasks", "GET", "/ScheduledTasks/{id}", "no task ID returned")
        self.test("display preferences", "users", "GET", "/DisplayPreferences/music", params={"UserId": self.client.user_id, "Client": "YinqiaoAudit"})

    def explicit_unsupported(self) -> None:
        for name, method, path in (
            ("live tv", "GET", "/LiveTv/Channels"),
            ("videos", "GET", "/Videos/ActiveEncodings"),
            ("plugins", "GET", "/Plugins"),
            ("remote sessions", "GET", "/Sessions"),
            ("auth providers", "GET", "/Auth/Providers"),
            ("library options", "GET", "/Libraries/AvailableOptions"),
            ("system logs", "GET", "/System/Logs"),
        ):
            self.test(name, "unsupported", method, path, statuses=(404, 501), unsupported=True)
        if self.track:
            track_id = str(self.track.get("Id", ""))
            for name, method, path, payload in (
                ("item ancestors", "GET", f"/Items/{track_id}/Ancestors", None),
                ("instant mix", "GET", f"/Items/{track_id}/InstantMix", None),
                ("lyric upload", "POST", f"/Audio/{track_id}/Lyrics", {}),
                ("remote lyric search", "GET", f"/Audio/{track_id}/RemoteSearch/Lyrics", None),
                ("resume position update", "POST", f"/UserItems/{track_id}/UserData", {"PlaybackPositionTicks": 10000000}),
            ):
                self.test(name, "unsupported", method, path, payload=payload, statuses=(501,), unsupported=True)

    def playlist_mutations(self) -> None:
        if not self.track:
            self.skip("playlist mutation", "mutations", "POST", "/Playlists", "no track available")
            return
        name = "Yinqiao Audit " + secrets.token_hex(4)
        created = self.test("create temporary playlist", "mutations", "POST", "/Playlists", params={"Name": name, "Ids": self.track.get("Id"), "UserId": self.client.user_id})
        if not created or not isinstance(created.json_data, dict):
            return
        playlist_id = str(created.json_data.get("Id", ""))
        if not playlist_id:
            return
        try:
            self.test("read temporary playlist", "mutations", "GET", f"/Items/{playlist_id}")
            self.test("read playlist protocol detail", "mutations", "GET", f"/Playlists/{playlist_id}")
            self.test("rename temporary playlist", "mutations", "POST", f"/Playlists/{playlist_id}", payload={"Name": name + " renamed"}, statuses=(204,))
            self.test("read playlist items", "mutations", "GET", f"/Playlists/{playlist_id}/Items", params={"UserId": self.client.user_id}, validate=lambda r: self._exact_collection(r, 1))
            if len(self.tracks) > 1:
                second_id = str(self.tracks[1].get("Id", ""))
                self.test("add playlist item", "mutations", "POST", f"/Playlists/{playlist_id}/Items", params={"Ids": second_id, "UserId": self.client.user_id}, statuses=(204,))
                self.test("verify playlist add", "mutations", "GET", f"/Playlists/{playlist_id}/Items", params={"UserId": self.client.user_id}, validate=lambda r: self._exact_collection(r, 2))
                self.test("remove playlist item", "mutations", "DELETE", f"/Playlists/{playlist_id}/Items", params={"EntryIds": second_id}, statuses=(204,))
                self.test("verify playlist remove", "mutations", "GET", f"/Playlists/{playlist_id}/Items", params={"UserId": self.client.user_id}, validate=lambda r: self._exact_collection(r, 1))
            self.favorite_mutation()
            self.test("report played item", "mutations", "POST", f"/UserPlayedItems/{self.track.get('Id')}", params={"UserId": self.client.user_id})
        finally:
            self.test("delete temporary playlist", "mutations", "DELETE", f"/Items/{playlist_id}", statuses=(204,))

    def _exact_collection(self, response: Response, expected: int) -> str:
        detail = self.collection_detail(response)
        payload = response.json_data if isinstance(response.json_data, dict) else {}
        if len(items(payload)) != expected or payload.get("TotalRecordCount") != expected:
            raise AssertionError(f"expected exactly {expected} items; {detail}")
        return detail

    def favorite_mutation(self) -> None:
        if not self.track:
            return
        track_id = str(self.track.get("Id", ""))
        before = self.test("read favorite before mutation", "mutations", "GET", f"/UserItems/{track_id}/UserData")
        original = bool((before.json_data or {}).get("IsFavorite")) if before else False
        first_method = "DELETE" if original else "POST"
        restore_method = "POST" if original else "DELETE"
        self.test("toggle favorite", "mutations", first_method, f"/UserFavoriteItems/{track_id}", statuses=(200, 204))
        changed = self.test("verify favorite toggle", "mutations", "GET", f"/UserItems/{track_id}/UserData")
        if changed and bool((changed.json_data or {}).get("IsFavorite")) == original:
            self.results.append(Result("verify favorite value", "mutations", "GET", f"/UserItems/{track_id}/UserData", "fail", changed.status, changed.elapsed_ms, "favorite state did not change", changed.artifact))
        self.test("restore favorite", "mutations", restore_method, f"/UserFavoriteItems/{track_id}", statuses=(200, 204))
        restored = self.test("verify favorite restore", "mutations", "GET", f"/UserItems/{track_id}/UserData")
        if restored and bool((restored.json_data or {}).get("IsFavorite")) != original:
            self.results.append(Result("verify favorite restored value", "mutations", "GET", f"/UserItems/{track_id}/UserData", "fail", restored.status, restored.elapsed_ms, "favorite state was not restored", restored.artifact))

    def session_logout(self) -> None:
        logout = self.test("session logout", "sessions", "POST", "/Sessions/Logout", statuses=(204,))
        if not logout:
            return
        response = self.client.request("GET", "/Users/Me")
        if response.status == 401:
            self.results.append(Result("logged-out token rejected", "authentication", "GET", "/Users/Me", "pass", response.status, response.elapsed_ms, "token invalidated", response.artifact))
        else:
            self.results.append(Result("logged-out token rejected", "authentication", "GET", "/Users/Me", "fail", response.status, response.elapsed_ms, "token remained valid after logout", response.artifact))


def write_report(output: Path, base_url: str, started: str, results: list[Result]) -> None:
    counts = {status: sum(1 for result in results if result.outcome == status) for status in ("pass", "fail", "skip", "unsupported")}
    report = {
        "startedAt": started,
        "finishedAt": now_iso(),
        "baseUrl": base_url,
        "summary": counts,
        "results": [asdict(result) for result in results],
    }
    (output / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    lines = [
        "# Jellyfin Audio audit",
        "",
        f"- Base URL: `{base_url}`",
        f"- Started: `{started}`",
        f"- Result: pass {counts['pass']}, fail {counts['fail']}, skip {counts['skip']}, explicitly unsupported {counts['unsupported']}",
        "",
        "| Result | Category | Interface | HTTP | Time | Evidence / detail |",
        "| --- | --- | --- | ---: | ---: | --- |",
    ]
    for result in results:
        evidence = f"[{result.artifact}]({result.artifact})" if result.artifact else ""
        detail = result.detail.replace("|", "\\|").replace("\n", " ")
        lines.append(f"| {result.outcome} | {result.category} | `{result.method} {result.path}` | {result.status or ''} | {result.elapsed_ms} ms | {evidence} {detail} |")
    (output / "report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--username", required=True)
    parser.add_argument("--password", required=True)
    parser.add_argument("--timeout", type=float, default=15)
    parser.add_argument("--output", type=Path, default=Path(__file__).resolve().parent / "reports")
    parser.add_argument("--mutations", action="store_true")
    args = parser.parse_args()
    run_id = datetime.now().strftime("%Y%m%d-%H%M%S") + "-" + secrets.token_hex(3)
    output = args.output / (run_id + "-jellyfin")
    (output / "responses").mkdir(parents=True)
    started = now_iso()
    client = Client(args.url, args.username, args.password, output, args.timeout)
    audit = Audit(client, args.mutations)
    audit.run()
    write_report(output, args.url, started, audit.results)
    print(output / "report.md")
    return 1 if any(result.outcome == "fail" for result in audit.results) else 0


if __name__ == "__main__":
    sys.exit(main())
