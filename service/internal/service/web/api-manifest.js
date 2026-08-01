window.API_CAPABILITY_MANIFEST = Object.freeze([
  {
    "id": "system.config",
    "category": "system",
    "summary": "读取服务配置与 OAuth 入口",
    "method": "GET",
    "path": "/yinqiao/v1/system/config",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "system.initialization",
    "category": "system",
    "summary": "读取初始化状态",
    "method": "GET",
    "path": "/yinqiao/v1/system/initialization",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "auth.initialization.prepare",
    "category": "auth",
    "summary": "准备初始化会话",
    "method": "POST",
    "path": "/yinqiao/v1/auth/initialization/prepare",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "auth.initialization.confirm",
    "category": "auth",
    "summary": "确认初始化",
    "method": "POST",
    "path": "/yinqiao/v1/auth/initialization/confirm",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "auth.login.oauth",
    "category": "auth",
    "summary": "使用 fnOS OAuth 登录",
    "method": "POST",
    "path": "/yinqiao/v1/auth/login/oauth",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "auth.login.password",
    "category": "auth",
    "summary": "使用用户名密码登录",
    "method": "POST",
    "path": "/yinqiao/v1/auth/login/password",
    "access": "public",
    "responseMode": "json"
  },
  {
    "id": "auth.me",
    "category": "auth",
    "summary": "读取当前用户",
    "method": "GET",
    "path": "/yinqiao/v1/auth/me",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "auth.logout",
    "category": "auth",
    "summary": "退出登录",
    "method": "POST",
    "path": "/yinqiao/v1/auth/logout",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "users.list",
    "category": "users",
    "summary": "列出用户",
    "method": "GET",
    "path": "/yinqiao/v1/admin/users",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "users.exists",
    "category": "users",
    "summary": "检查用户名",
    "method": "GET",
    "path": "/yinqiao/v1/admin/users/exists",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "users.create",
    "category": "users",
    "summary": "创建用户",
    "method": "POST",
    "path": "/yinqiao/v1/admin/users",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "users.update",
    "category": "users",
    "summary": "编辑用户",
    "method": "POST",
    "path": "/yinqiao/v1/admin/users/update",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "account.password",
    "category": "users",
    "summary": "修改当前用户密码",
    "method": "POST",
    "path": "/yinqiao/v1/account/password",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "users.delete",
    "category": "users",
    "summary": "删除用户",
    "method": "POST",
    "path": "/yinqiao/v1/admin/users/delete",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "users.unban",
    "category": "users",
    "summary": "解除用户封禁",
    "method": "POST",
    "path": "/yinqiao/v1/admin/users/unban",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "settings.server.get",
    "category": "settings",
    "summary": "读取服务器设置",
    "method": "GET",
    "path": "/yinqiao/v1/admin/settings/server",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "settings.server.update",
    "category": "settings",
    "summary": "更新服务器设置",
    "method": "POST",
    "path": "/yinqiao/v1/admin/settings/server",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "settings.default-access.get",
    "category": "settings",
    "summary": "读取新用户默认曲库权限",
    "method": "GET",
    "path": "/yinqiao/v1/admin/settings/default-user-access",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "settings.default-access.update",
    "category": "settings",
    "summary": "更新新用户默认曲库权限",
    "method": "POST",
    "path": "/yinqiao/v1/admin/settings/default-user-access",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "directories.list",
    "category": "directories",
    "summary": "列出 fnOS 授权目录",
    "method": "GET",
    "path": "/yinqiao/v1/admin/directories",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "directories.children",
    "category": "directories",
    "summary": "列出授权目录子目录",
    "method": "GET",
    "path": "/yinqiao/v1/admin/directories/children",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.list",
    "category": "libraries",
    "summary": "列出共享音乐库",
    "method": "GET",
    "path": "/yinqiao/v1/admin/libraries",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.detail",
    "category": "libraries",
    "summary": "读取共享音乐库详情",
    "method": "GET",
    "path": "/yinqiao/v1/admin/libraries/detail",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.create",
    "category": "libraries",
    "summary": "创建共享音乐库",
    "method": "POST",
    "path": "/yinqiao/v1/admin/libraries",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.update",
    "category": "libraries",
    "summary": "更新共享音乐库",
    "method": "POST",
    "path": "/yinqiao/v1/admin/libraries/update",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.delete",
    "category": "libraries",
    "summary": "删除共享音乐库",
    "method": "POST",
    "path": "/yinqiao/v1/admin/libraries/delete",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.scan",
    "category": "libraries",
    "summary": "扫描单个共享音乐库",
    "method": "POST",
    "path": "/yinqiao/v1/admin/libraries/scan",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "libraries.scan-all",
    "category": "libraries",
    "summary": "扫描全部共享音乐库",
    "method": "POST",
    "path": "/yinqiao/v1/admin/libraries/scan-all",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tracks.list",
    "category": "catalog",
    "summary": "分页列出曲目",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/tracks",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "tracks.by-album",
    "category": "catalog",
    "summary": "列出专辑曲目",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/tracks/by-album",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "tracks.by-artist",
    "category": "catalog",
    "summary": "列出艺术家曲目",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/tracks/by-artist",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "tracks.by-genre",
    "category": "catalog",
    "summary": "列出风格曲目",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/tracks/by-genre",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "tracks.by-playlist",
    "category": "catalog",
    "summary": "列出歌单曲目",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/tracks/by-playlist",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "roam.start",
    "category": "playback",
    "summary": "开始漫游播放",
    "method": "GET",
    "path": "/yinqiao/v1/playback/roam/start",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "roam.next",
    "category": "playback",
    "summary": "获取漫游下一曲",
    "method": "GET",
    "path": "/yinqiao/v1/playback/roam/next",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "roam.previous",
    "category": "playback",
    "summary": "获取漫游上一曲",
    "method": "GET",
    "path": "/yinqiao/v1/playback/roam/previous",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "tracks.metadata.get",
    "category": "metadata",
    "summary": "读取曲目元数据",
    "method": "GET",
    "path": "/yinqiao/v1/admin/tracks/metadata",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tracks.metadata.update",
    "category": "metadata",
    "summary": "更新曲目元数据",
    "method": "POST",
    "path": "/yinqiao/v1/admin/tracks/metadata",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "covers.get",
    "category": "media",
    "summary": "读取封面",
    "method": "GET",
    "path": "/yinqiao/v1/media/covers/{coverId}",
    "access": "media",
    "responseMode": "raw"
  },
  {
    "id": "covers.track.upload",
    "category": "media",
    "summary": "上传曲目封面",
    "method": "POST",
    "path": "/yinqiao/v1/admin/tracks/cover",
    "access": "admin",
    "responseMode": "upload"
  },
  {
    "id": "covers.playlist.upload",
    "category": "media",
    "summary": "上传歌单封面",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/cover",
    "access": "user",
    "responseMode": "upload"
  },
  {
    "id": "playback.stream",
    "category": "playback",
    "summary": "流式读取原音频",
    "method": "GET",
    "path": "/yinqiao/v1/media/tracks/{guid}/stream",
    "access": "media",
    "responseMode": "raw"
  },
  {
    "id": "playback.hls",
    "category": "playback",
    "summary": "读取 HLS 播放列表或分片",
    "method": "GET",
    "path": "/yinqiao/v1/media/tracks/{guid}/hls/{filename}",
    "access": "media",
    "responseMode": "raw"
  },
  {
    "id": "transcode.start",
    "category": "playback",
    "summary": "启动转码会话",
    "method": "POST",
    "path": "/yinqiao/v1/playback/transcode/start",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "transcode.heartbeat",
    "category": "playback",
    "summary": "维持转码会话",
    "method": "POST",
    "path": "/yinqiao/v1/playback/transcode/heartbeat",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "transcode.stop",
    "category": "playback",
    "summary": "结束转码会话",
    "method": "POST",
    "path": "/yinqiao/v1/playback/transcode/stop",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "albums.list",
    "category": "catalog",
    "summary": "分页列出专辑",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/albums",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "albums.by-artist",
    "category": "catalog",
    "summary": "列出艺术家的专辑",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/albums/by-artist",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "albums.detail",
    "category": "catalog",
    "summary": "读取专辑详情",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/albums/detail",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "artists.create",
    "category": "metadata",
    "summary": "创建艺术家",
    "method": "POST",
    "path": "/yinqiao/v1/admin/artists",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "artists.list",
    "category": "catalog",
    "summary": "分页列出艺术家",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/artists",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "artists.all",
    "category": "catalog",
    "summary": "列出全部艺术家",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/artists/all",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "artists.detail",
    "category": "catalog",
    "summary": "读取艺术家详情",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/artists/detail",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "genres.create",
    "category": "metadata",
    "summary": "创建风格",
    "method": "POST",
    "path": "/yinqiao/v1/admin/genres",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "genres.list",
    "category": "catalog",
    "summary": "分页列出风格",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/genres",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "genres.detail",
    "category": "catalog",
    "summary": "读取风格详情",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/genres/detail",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "lyrics.list",
    "category": "catalog",
    "summary": "读取曲目歌词",
    "method": "GET",
    "path": "/yinqiao/v1/catalog/lyrics",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "events.report",
    "category": "activity",
    "summary": "上报播放与用户偏好事件",
    "method": "POST",
    "path": "/yinqiao/v1/activity/events",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "favorites.list",
    "category": "personal",
    "summary": "分页列出收藏曲目",
    "method": "GET",
    "path": "/yinqiao/v1/me/favorites",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "favorites.add",
    "category": "personal",
    "summary": "收藏曲目",
    "method": "POST",
    "path": "/yinqiao/v1/me/favorites",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "favorites.remove",
    "category": "personal",
    "summary": "取消收藏曲目",
    "method": "POST",
    "path": "/yinqiao/v1/me/favorites/remove",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "favorites.purge-count",
    "category": "personal",
    "summary": "统计失效收藏项",
    "method": "GET",
    "path": "/yinqiao/v1/me/favorites/purge-count",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "favorites.purge",
    "category": "personal",
    "summary": "清理失效收藏项",
    "method": "POST",
    "path": "/yinqiao/v1/me/favorites/purge",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "history.list",
    "category": "personal",
    "summary": "分页列出播放历史",
    "method": "GET",
    "path": "/yinqiao/v1/me/history",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "history.delete",
    "category": "personal",
    "summary": "删除播放历史",
    "method": "POST",
    "path": "/yinqiao/v1/me/history/delete",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.list",
    "category": "playlists",
    "summary": "列出歌单",
    "method": "GET",
    "path": "/yinqiao/v1/me/playlists",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.batch-detail",
    "category": "playlists",
    "summary": "批量读取歌单曲目数",
    "method": "GET",
    "path": "/yinqiao/v1/me/playlists/batch-detail",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.create",
    "category": "playlists",
    "summary": "创建歌单",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.update",
    "category": "playlists",
    "summary": "更新歌单",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/update",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.delete",
    "category": "playlists",
    "summary": "删除歌单",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/delete",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.detail",
    "category": "playlists",
    "summary": "读取歌单详情",
    "method": "GET",
    "path": "/yinqiao/v1/me/playlists/detail",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.tracks.add",
    "category": "playlists",
    "summary": "向歌单添加曲目",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/tracks/add",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.tracks.remove",
    "category": "playlists",
    "summary": "从歌单移除曲目",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/tracks/remove",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.purge-count",
    "category": "playlists",
    "summary": "统计歌单失效曲目",
    "method": "GET",
    "path": "/yinqiao/v1/me/playlists/purge-count",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "playlists.purge",
    "category": "playlists",
    "summary": "清理歌单失效曲目",
    "method": "POST",
    "path": "/yinqiao/v1/me/playlists/purge",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.tracks",
    "category": "search",
    "summary": "搜索曲目",
    "method": "GET",
    "path": "/yinqiao/v1/search/tracks",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.albums",
    "category": "search",
    "summary": "搜索专辑",
    "method": "GET",
    "path": "/yinqiao/v1/search/albums",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.artists",
    "category": "search",
    "summary": "搜索艺术家",
    "method": "GET",
    "path": "/yinqiao/v1/search/artists",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.playlists",
    "category": "search",
    "summary": "搜索歌单",
    "method": "GET",
    "path": "/yinqiao/v1/search/playlists",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.suggest",
    "category": "search",
    "summary": "获取搜索联想",
    "method": "GET",
    "path": "/yinqiao/v1/search/suggest",
    "access": "user",
    "responseMode": "json"
  },
  {
    "id": "search.rebuild",
    "category": "search",
    "summary": "重建搜索索引",
    "method": "POST",
    "path": "/yinqiao/v1/admin/search/rebuild",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tasks.list",
    "category": "tasks",
    "summary": "列出后台任务",
    "method": "GET",
    "path": "/yinqiao/v1/admin/tasks",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tasks.cancel",
    "category": "tasks",
    "summary": "取消后台任务",
    "method": "POST",
    "path": "/yinqiao/v1/admin/tasks/cancel",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tasks.delete",
    "category": "tasks",
    "summary": "删除后台任务记录",
    "method": "POST",
    "path": "/yinqiao/v1/admin/tasks/delete",
    "access": "admin",
    "responseMode": "json"
  },
  {
    "id": "tasks.retry",
    "category": "tasks",
    "summary": "重试后台任务",
    "method": "POST",
    "path": "/yinqiao/v1/admin/tasks/retry",
    "access": "admin",
    "responseMode": "json"
  }
]);
