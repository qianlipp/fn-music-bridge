package api

import "net/http"

type Access string
type ResponseMode string
type Adapter string

const (
	AccessPublic Access = "public"
	AccessUser   Access = "user"
	AccessAdmin  Access = "admin"
	AccessMedia  Access = "media"

	ModeJSON   ResponseMode = "json"
	ModeRaw    ResponseMode = "raw"
	ModeUpload ResponseMode = "upload"

	AdapterNone                  Adapter = "none"
	AdapterPassword              Adapter = "password"
	AdapterInitializationConfirm Adapter = "initialization-confirm"
)

type Route struct {
	ID             string            `json:"id"`
	Category       string            `json:"category"`
	Summary        string            `json:"summary"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Access         Access            `json:"access"`
	UpstreamMethod string            `json:"-"`
	UpstreamPath   string            `json:"-"`
	QueryAdd       map[string]string `json:"-"`
	Mode           ResponseMode      `json:"responseMode"`
	Adapter        Adapter           `json:"-"`
}

func JSONRoute(id, category, summary, method, path string, access Access, upstreamPath string) Route {
	return Route{ID: id, Category: category, Summary: summary, Method: method, Path: path, Access: access, UpstreamMethod: method, UpstreamPath: upstreamPath, Mode: ModeJSON}
}

func StableRoutes() []Route {
	routes := []Route{
		// Initialization and authentication (8).
		JSONRoute("system.config", "system", "读取服务配置与 OAuth 入口", http.MethodGet, "/system/config", AccessPublic, "/sys/config"),
		JSONRoute("system.initialization", "system", "读取初始化状态", http.MethodGet, "/system/initialization", AccessPublic, "/initialization/state"),
		JSONRoute("auth.initialization.prepare", "auth", "准备初始化会话", http.MethodPost, "/auth/initialization/prepare", AccessPublic, "/initialization/prepare"),
		{ID: "auth.initialization.confirm", Category: "auth", Summary: "确认初始化", Method: http.MethodPost, Path: "/auth/initialization/confirm", Access: AccessPublic, UpstreamMethod: http.MethodPost, UpstreamPath: "/initialization/confirm", Mode: ModeJSON, Adapter: AdapterInitializationConfirm},
		JSONRoute("auth.login.oauth", "auth", "使用 fnOS OAuth 登录", http.MethodPost, "/auth/login/oauth", AccessPublic, "/user/auth-login"),
		{ID: "auth.login.password", Category: "auth", Summary: "使用用户名密码登录", Method: http.MethodPost, Path: "/auth/login/password", Access: AccessPublic, UpstreamMethod: http.MethodPost, UpstreamPath: "/user/password-login", Mode: ModeJSON, Adapter: AdapterPassword},
		JSONRoute("auth.me", "auth", "读取当前用户", http.MethodGet, "/auth/me", AccessUser, "/user/me"),
		JSONRoute("auth.logout", "auth", "退出登录", http.MethodPost, "/auth/logout", AccessUser, "/user/logout"),

		// Users and permissions (7).
		JSONRoute("users.list", "users", "列出用户", http.MethodGet, "/admin/users", AccessAdmin, "/user/list"),
		JSONRoute("users.exists", "users", "检查用户名", http.MethodGet, "/admin/users/exists", AccessAdmin, "/user/exists"),
		{ID: "users.create", Category: "users", Summary: "创建用户", Method: http.MethodPost, Path: "/admin/users", Access: AccessAdmin, UpstreamMethod: http.MethodPost, UpstreamPath: "/user/create", Mode: ModeJSON, Adapter: AdapterPassword},
		{ID: "users.update", Category: "users", Summary: "编辑用户", Method: http.MethodPost, Path: "/admin/users/update", Access: AccessAdmin, UpstreamMethod: http.MethodPost, UpstreamPath: "/user/edit", Mode: ModeJSON, Adapter: AdapterPassword},
		{ID: "account.password", Category: "users", Summary: "修改当前用户密码", Method: http.MethodPost, Path: "/account/password", Access: AccessUser, UpstreamMethod: http.MethodPost, UpstreamPath: "/user/passwd-change", Mode: ModeJSON, Adapter: AdapterPassword},
		JSONRoute("users.delete", "users", "删除用户", http.MethodPost, "/admin/users/delete", AccessAdmin, "/user/delete"),
		JSONRoute("users.unban", "users", "解除用户封禁", http.MethodPost, "/admin/users/unban", AccessAdmin, "/user/unbanned"),

		// Settings (4).
		JSONRoute("settings.server.get", "settings", "读取服务器设置", http.MethodGet, "/admin/settings/server", AccessAdmin, "/settings/server"),
		JSONRoute("settings.server.update", "settings", "更新服务器设置", http.MethodPost, "/admin/settings/server", AccessAdmin, "/settings/server"),
		JSONRoute("settings.default-access.get", "settings", "读取新用户默认曲库权限", http.MethodGet, "/admin/settings/default-user-access", AccessAdmin, "/settings/user"),
		JSONRoute("settings.default-access.update", "settings", "更新新用户默认曲库权限", http.MethodPost, "/admin/settings/default-user-access", AccessAdmin, "/settings/user"),

		// Authorized directories (2).
		JSONRoute("directories.list", "directories", "列出 fnOS 授权目录", http.MethodGet, "/admin/directories", AccessAdmin, "/app-center/authed-dir/list"),
		JSONRoute("directories.children", "directories", "列出授权目录子目录", http.MethodGet, "/admin/directories/children", AccessAdmin, "/app-center/authed-dir/sub/list"),

		// Shared libraries (7).
		JSONRoute("libraries.list", "libraries", "列出共享音乐库", http.MethodGet, "/admin/libraries", AccessAdmin, "/shared-library/list"),
		JSONRoute("libraries.detail", "libraries", "读取共享音乐库详情", http.MethodGet, "/admin/libraries/detail", AccessAdmin, "/shared-library/detail"),
		JSONRoute("libraries.create", "libraries", "创建共享音乐库", http.MethodPost, "/admin/libraries", AccessAdmin, "/shared-library/create"),
		JSONRoute("libraries.update", "libraries", "更新共享音乐库", http.MethodPost, "/admin/libraries/update", AccessAdmin, "/shared-library/edit"),
		JSONRoute("libraries.delete", "libraries", "删除共享音乐库", http.MethodPost, "/admin/libraries/delete", AccessAdmin, "/shared-library/delete"),
		JSONRoute("libraries.scan", "libraries", "扫描单个共享音乐库", http.MethodPost, "/admin/libraries/scan", AccessAdmin, "/shared-library/scan"),
		JSONRoute("libraries.scan-all", "libraries", "扫描全部共享音乐库", http.MethodPost, "/admin/libraries/scan-all", AccessAdmin, "/shared-library/scan-all"),

		// Catalog track lists (5).
		JSONRoute("tracks.list", "catalog", "分页列出曲目", http.MethodGet, "/catalog/tracks", AccessUser, "/track/list"),
		JSONRoute("tracks.by-album", "catalog", "列出专辑曲目", http.MethodGet, "/catalog/tracks/by-album", AccessUser, "/track/album-detail/list"),
		JSONRoute("tracks.by-artist", "catalog", "列出艺术家曲目", http.MethodGet, "/catalog/tracks/by-artist", AccessUser, "/track/artist-detail/list"),
		JSONRoute("tracks.by-genre", "catalog", "列出风格曲目", http.MethodGet, "/catalog/tracks/by-genre", AccessUser, "/track/genre-detail/list"),
		JSONRoute("tracks.by-playlist", "catalog", "列出歌单曲目", http.MethodGet, "/catalog/tracks/by-playlist", AccessUser, "/track/playlist-detail/list"),

		// Roam playback (3).
		JSONRoute("roam.start", "playback", "开始漫游播放", http.MethodGet, "/playback/roam/start", AccessUser, "/track/roam-start"),
		JSONRoute("roam.next", "playback", "获取漫游下一曲", http.MethodGet, "/playback/roam/next", AccessUser, "/track/roam-next"),
		JSONRoute("roam.previous", "playback", "获取漫游上一曲", http.MethodGet, "/playback/roam/previous", AccessUser, "/track/roam-previous"),

		// Track metadata (2).
		JSONRoute("tracks.metadata.get", "metadata", "读取曲目元数据", http.MethodGet, "/admin/tracks/metadata", AccessAdmin, "/track/metadata"),
		JSONRoute("tracks.metadata.update", "metadata", "更新曲目元数据", http.MethodPost, "/admin/tracks/metadata", AccessAdmin, "/track/metadata"),

		// Covers (3).
		{ID: "covers.get", Category: "media", Summary: "读取封面", Method: http.MethodGet, Path: "/media/covers/{coverId}", Access: AccessMedia, UpstreamMethod: http.MethodGet, UpstreamPath: "/static/cover", QueryAdd: map[string]string{"coverId": "{coverId}"}, Mode: ModeRaw},
		{ID: "covers.track.upload", Category: "media", Summary: "上传曲目封面", Method: http.MethodPost, Path: "/admin/tracks/cover", Access: AccessAdmin, UpstreamMethod: http.MethodPost, UpstreamPath: "/static/cover/track", Mode: ModeUpload},
		{ID: "covers.playlist.upload", Category: "media", Summary: "上传歌单封面", Method: http.MethodPost, Path: "/me/playlists/cover", Access: AccessUser, UpstreamMethod: http.MethodPost, UpstreamPath: "/static/cover/playlist", Mode: ModeUpload},

		// Streaming and transcoding (5).
		{ID: "playback.stream", Category: "playback", Summary: "流式读取原音频", Method: http.MethodGet, Path: "/media/tracks/{guid}/stream", Access: AccessMedia, UpstreamMethod: http.MethodGet, UpstreamPath: "/track/stream", QueryAdd: map[string]string{"guid": "{guid}"}, Mode: ModeRaw},
		{ID: "playback.hls", Category: "playback", Summary: "读取 HLS 播放列表或分片", Method: http.MethodGet, Path: "/media/tracks/{guid}/hls/{filename}", Access: AccessMedia, UpstreamMethod: http.MethodGet, UpstreamPath: "/track/hls/{guid}/{filename}", Mode: ModeRaw},
		JSONRoute("transcode.start", "playback", "启动转码会话", http.MethodPost, "/playback/transcode/start", AccessUser, "/track/transcode"),
		JSONRoute("transcode.heartbeat", "playback", "维持转码会话", http.MethodPost, "/playback/transcode/heartbeat", AccessUser, "/track/transcode/heartbeat"),
		JSONRoute("transcode.stop", "playback", "结束转码会话", http.MethodPost, "/playback/transcode/stop", AccessUser, "/track/transcode/quit"),

		// Albums (3).
		JSONRoute("albums.list", "catalog", "分页列出专辑", http.MethodGet, "/catalog/albums", AccessUser, "/album/list"),
		JSONRoute("albums.by-artist", "catalog", "列出艺术家的专辑", http.MethodGet, "/catalog/albums/by-artist", AccessUser, "/album/artist-detail/list"),
		JSONRoute("albums.detail", "catalog", "读取专辑详情", http.MethodGet, "/catalog/albums/detail", AccessUser, "/album/detail"),

		// Artists (4).
		JSONRoute("artists.create", "metadata", "创建艺术家", http.MethodPost, "/admin/artists", AccessAdmin, "/artist/create"),
		JSONRoute("artists.list", "catalog", "分页列出艺术家", http.MethodGet, "/catalog/artists", AccessUser, "/artist/list"),
		JSONRoute("artists.all", "catalog", "列出全部艺术家", http.MethodGet, "/catalog/artists/all", AccessUser, "/artist/list-all"),
		JSONRoute("artists.detail", "catalog", "读取艺术家详情", http.MethodGet, "/catalog/artists/detail", AccessUser, "/artist/detail"),

		// Genres (3).
		JSONRoute("genres.create", "metadata", "创建风格", http.MethodPost, "/admin/genres", AccessAdmin, "/genre/create"),
		JSONRoute("genres.list", "catalog", "分页列出风格", http.MethodGet, "/catalog/genres", AccessUser, "/genre/list"),
		JSONRoute("genres.detail", "catalog", "读取风格详情", http.MethodGet, "/catalog/genres/detail", AccessUser, "/genre/detail"),

		// Lyrics and events (2).
		JSONRoute("lyrics.list", "catalog", "读取曲目歌词", http.MethodGet, "/catalog/lyrics", AccessUser, "/lyric/list"),
		JSONRoute("events.report", "activity", "上报播放与用户偏好事件", http.MethodPost, "/activity/events", AccessUser, "/event/report"),

		// Favorites (5).
		JSONRoute("favorites.list", "personal", "分页列出收藏曲目", http.MethodGet, "/me/favorites", AccessUser, "/favorite-track/list"),
		JSONRoute("favorites.add", "personal", "收藏曲目", http.MethodPost, "/me/favorites", AccessUser, "/favorite-track/create"),
		JSONRoute("favorites.remove", "personal", "取消收藏曲目", http.MethodPost, "/me/favorites/remove", AccessUser, "/favorite-track/delete"),
		JSONRoute("favorites.purge-count", "personal", "统计失效收藏项", http.MethodGet, "/me/favorites/purge-count", AccessUser, "/favorite-track/purge-track-count"),
		JSONRoute("favorites.purge", "personal", "清理失效收藏项", http.MethodPost, "/me/favorites/purge", AccessUser, "/favorite-track/purge-track"),

		// Play history (2).
		JSONRoute("history.list", "personal", "分页列出播放历史", http.MethodGet, "/me/history", AccessUser, "/play-history/list"),
		JSONRoute("history.delete", "personal", "删除播放历史", http.MethodPost, "/me/history/delete", AccessUser, "/play-history/delete"),

		// Playlists except the cover upload already listed above (10).
		JSONRoute("playlists.list", "playlists", "列出歌单", http.MethodGet, "/me/playlists", AccessUser, "/playlist/list"),
		JSONRoute("playlists.batch-detail", "playlists", "批量读取歌单曲目数", http.MethodGet, "/me/playlists/batch-detail", AccessUser, "/playlist/batch-detail"),
		JSONRoute("playlists.create", "playlists", "创建歌单", http.MethodPost, "/me/playlists", AccessUser, "/playlist/create"),
		JSONRoute("playlists.update", "playlists", "更新歌单", http.MethodPost, "/me/playlists/update", AccessUser, "/playlist/edit"),
		JSONRoute("playlists.delete", "playlists", "删除歌单", http.MethodPost, "/me/playlists/delete", AccessUser, "/playlist/delete"),
		JSONRoute("playlists.detail", "playlists", "读取歌单详情", http.MethodGet, "/me/playlists/detail", AccessUser, "/playlist/detail"),
		JSONRoute("playlists.tracks.add", "playlists", "向歌单添加曲目", http.MethodPost, "/me/playlists/tracks/add", AccessUser, "/playlist/add-track"),
		JSONRoute("playlists.tracks.remove", "playlists", "从歌单移除曲目", http.MethodPost, "/me/playlists/tracks/remove", AccessUser, "/playlist/remove-track"),
		JSONRoute("playlists.purge-count", "playlists", "统计歌单失效曲目", http.MethodGet, "/me/playlists/purge-count", AccessUser, "/playlist/purge-track-count"),
		JSONRoute("playlists.purge", "playlists", "清理歌单失效曲目", http.MethodPost, "/me/playlists/purge", AccessUser, "/playlist/purge-track"),

		// Search (6).
		JSONRoute("search.tracks", "search", "搜索曲目", http.MethodGet, "/search/tracks", AccessUser, "/search/track"),
		JSONRoute("search.albums", "search", "搜索专辑", http.MethodGet, "/search/albums", AccessUser, "/search/album"),
		JSONRoute("search.artists", "search", "搜索艺术家", http.MethodGet, "/search/artists", AccessUser, "/search/artist"),
		JSONRoute("search.playlists", "search", "搜索歌单", http.MethodGet, "/search/playlists", AccessUser, "/search/playlist"),
		JSONRoute("search.suggest", "search", "获取搜索联想", http.MethodGet, "/search/suggest", AccessUser, "/search/suggest"),
		JSONRoute("search.rebuild", "search", "重建搜索索引", http.MethodPost, "/admin/search/rebuild", AccessAdmin, "/search/index/rebuild"),

		// Background tasks (4).
		JSONRoute("tasks.list", "tasks", "列出后台任务", http.MethodGet, "/admin/tasks", AccessAdmin, "/task/list"),
		JSONRoute("tasks.cancel", "tasks", "取消后台任务", http.MethodPost, "/admin/tasks/cancel", AccessAdmin, "/task/cancel"),
		JSONRoute("tasks.delete", "tasks", "删除后台任务记录", http.MethodPost, "/admin/tasks/delete", AccessAdmin, "/task/delete"),
		JSONRoute("tasks.retry", "tasks", "重试后台任务", http.MethodPost, "/admin/tasks/retry", AccessAdmin, "/task/retry"),
	}
	return routes
}
