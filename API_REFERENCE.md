# Yinqiao API 说明书

> 适用版本：音桥 `1.0.5`、Yinqiao API `v1`、OpenSubsonic/Subsonic `1.16.1`、Synology Audio Station、Jellyfin Audio
> 文档依据：当前项目实际路由表和协议分派代码
> 更新日期：2026-08-01

本文同时说明两层接口：

- **Yinqiao API**：完整桥接飞牛音乐后台，适合 Flutter、Web、桌面和自研客户端。
- **OpenSubsonic/Subsonic**：把 Yinqiao API 中常用的音乐能力映射为标准播放协议。
- **Synology Audio Station**：兼容 DSM WebAPI 和 Audio Station 音乐客户端调用。
- **Jellyfin Audio**：兼容 Jellyfin 客户端所需的音乐服务子集。
- **Ampache**：保留 API 6 XML/JSON 音乐接口。

| 服务 | 默认地址 | 鉴权 | 当前能力 |
| --- | --- | --- | --- |
| Yinqiao API | `http://NAS_IP:18090/yinqiao/v1` | fnmusic Bearer Token | 85 条稳定路由、通用官方 API 透传 |
| OpenSubsonic | `http://NAS_IP:14040/rest/{method}.view` | fnmusic 用户名和密码或 Subsonic Token | 35 条实际/派生能力、16 条空集合兼容、4 条无操作兼容 |
| Synology Audio Station | `http://NAS_IP:14040/webapi/*` | fnmusic 用户名和密码换取 `_sid` | 14 个已声明 API 模块 |
| Jellyfin Audio | `http://NAS_IP:14040` | `AuthenticateByName` 换取 AccessToken | 服务发现、音乐库、媒体和播放常用路径 |
| Ampache | `http://NAS_IP:14040/server/{xml,json}.server.php` | 登记后的摘要握手 | API 6 核心浏览与播放动作 |

外网部署时只需替换协议、域名和端口，后面的路径保持不变。自研客户端需要完整后台能力时优先使用 Yinqiao API；需要接入现有播放器时使用 Subsonic。

---

# 第一部分：Yinqiao API

## 1. 调用约定

### 1.1 基址

```text
http://NAS_IP:18090/yinqiao/v1
```

下文 Yinqiao API 路径都相对于这个基址。例如 `/catalog/tracks` 的完整地址是：

```text
http://NAS_IP:18090/yinqiao/v1/catalog/tracks
```

### 1.2 请求格式

- `GET` 参数放在 URL Query。
- `POST` 默认使用 `Content-Type: application/json`。
- 封面上传使用 `multipart/form-data`，文件字段名为 `file`。
- 音频、封面、HLS 播放列表和分片返回原始数据，不使用 JSON 包装。
- 支持传入 `X-Request-ID`；未传时 Yinqiao API 自动生成。
- 当前独立 LAN Yinqiao API 不要求 `X-Music-Yinqiao API-Key`。

### 1.3 权限级别

| 标记 | 含义 |
| --- | --- |
| `公开` | 不需要 fnmusic 登录态 |
| `用户` | 需要任意有效 fnmusic 用户 Token，并受该用户曲库权限限制 |
| `管理员` | 需要 fnmusic 管理员 Token |
| `媒体` | 需要有效用户登录态；支持 Bearer Token 或 Yinqiao API 写入的 `music-token` Cookie |

独立客户端登录后，在后续请求中发送：

```http
Authorization: Bearer <userToken>
```

不要把 Token 放进 URL。收到 `401` 后应清除本地 Token 并重新登录。

### 1.4 JSON 成功响应

```json
{
  "ok": true,
  "data": {},
  "meta": {
    "requestId": "f8ac...",
    "adapter": "yinqiao-v1"
  }
}
```

分页接口通常返回：

```json
{
  "ok": true,
  "data": {
    "list": [],
    "total": 0,
    "sort": "createdAt,desc"
  },
  "meta": {
    "requestId": "f8ac...",
    "adapter": "yinqiao-v1"
  }
}
```

### 1.5 JSON 错误响应

```json
{
  "ok": false,
  "error": {
    "code": "UPSTREAM_120001",
    "message": "登录状态已失效",
    "upstreamCode": 120001
  },
  "meta": {
    "requestId": "f8ac...",
    "adapter": "yinqiao-v1"
  }
}
```

| HTTP | 含义和客户端处理 |
| ---: | --- |
| `400` | 请求体、路径或参数格式错误 |
| `401` | fnmusic Token/登录态无效，重新登录 |
| `403` | 用户角色、曲库权限或来源不允许，不应自动重试 |
| `404` | 资源或 Yinqiao API 路由不存在 |
| `409` | 名称、路径冲突，或任务已经运行 |
| `413` | 请求体或上传文件超过限制 |
| `422` | 参数格式有效，但不符合上游业务约束 |
| `502` | 本机飞牛音乐后台不可达或响应格式未知 |

业务分支使用稳定字段 `error.code`，界面提示使用 `error.message`。

### 1.6 分页和排序

- `page` 从 `1` 开始。
- `size` 是每页数量。
- `sort` 格式为 `字段,asc` 或 `字段,desc`，例如 `createdAt,desc`、`trackNo,asc`。
- 客户端应结合 `total` 判断是否加载完毕。

## 2. 公共数据模型

字段可能随飞牛音乐版本增加；客户端应忽略未知字段，不要因新增字段解析失败。

```ts
interface Track {
  guid: string;
  title: string;
  coverId?: string;
  year?: number;
  duration: number;          // 毫秒
  trackNo?: number;
  discNo?: number;
  isFavorite?: boolean;
  isCue?: boolean;
  hasLyric?: boolean;
  accessStatus?: number;     // 0 正常；非 0 通常表示缺失或无权限
  createdAt: number;         // Unix 秒
  updatedAt: number;
  artists?: Artist[];
  album?: Album;
  audioSpec?: AudioSpec;
}

interface AudioSpec {
  path?: string;
  extension?: string;
  format?: string;
  container?: string;
  codec?: string;
  size?: number;             // 字节
  duration?: number;         // 毫秒
  sampleRate?: number;
  bitrate?: number;          // bit/s
  bitDepth?: number;
  channel?: number;
  channels?: number;
}

interface Artist {
  guid: string;
  name: string;
  coverId?: string;
  albumCount?: number;
  trackCount?: number;
}

interface Album {
  guid: string;
  name: string;
  coverId?: string;
  releaseDate?: string;
  originalReleaseYear?: number;
  trackCount?: number;
  artists?: Artist[];
}

interface Genre {
  guid: string;
  name: string;
  coverId?: string;
  trackCount?: number;
}

interface Playlist {
  guid: string;
  name: string;
  coverId?: string | null;
  trackCount?: number;
  createdAt: number;
  updatedAt: number;
}

type LibraryAccess =
  | {mode: "all"; guids: []}
  | {mode: "partial"; guids: string[]}
  | {mode: "none"; guids: []};

interface MusicUser {
  guid: string;
  name: string;
  role: "admin" | "member";
  isBanned?: boolean;
  bannedUntil?: number | null;
  lastAccessedAt?: number | null;
  sharedLibraryAccess?: {
    mode: "all" | "partial" | "none";
    sharedLibraries: SharedLibrary[];
  };
}

interface SharedLibrary {
  guid: string;
  name: string;
  path: string;
  autoDownloadLyric: boolean;
  metadataPreference: "local_preferred" | "cloud_preferred" | "local_only";
  accessStatus: number;
  contentLastChangedAt?: number;
  coverIds: string[];
}
```

## 3. Yinqiao API 自身与服务发现

这些地址仍位于 Yinqiao API 基址之下。

| Method | Path | 权限 | 参数 | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/` | 公开 | 无 | Yinqiao API 名称、版本、API 版本、稳定能力数 |
| `GET` | `/health` | 公开 | 无 | 进程存活：`{status:"ok",upstream:"configured"}` |
| `GET` | `/ready` | 公开 | 无 | 上游可用时返回 ready；不可用时 HTTP `503` |
| `GET` | `/capabilities` | 公开 | 无 | 返回 85 条稳定路由的 ID、分类、方法、路径、权限和响应模式 |

`/health` 只表示 Yinqiao API 进程正常，`/ready` 才表示飞牛音乐上游可调用。

## 4. 初始化与鉴权（8）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/system/config` | 公开 | 无 | `{serverName,serverGUID,nasOAuth?}` |
| `GET` | `/system/initialization` | 公开 | 无 | `{initialized:boolean}` |
| `POST` | `/auth/initialization/prepare` | 公开 | `{code,deviceId}` | `{sessionId,operator}` |
| `POST` | `/auth/initialization/confirm` | 公开 | `{sessionId,user:{name,password},server:{name,lang}}` | `{userToken,user}` |
| `POST` | `/auth/login/oauth` | 公开 | `{code,deviceId}` | `{userToken,user}` |
| `POST` | `/auth/login/password` | 公开 | `{username,password,deviceId}` | `{userToken,user}`，同时写入 Cookie |
| `GET` | `/auth/me` | 用户 | 无 | 当前 `MusicUser` |
| `POST` | `/auth/logout` | 用户 | 空 body | 退出并使当前登录态失效 |

`deviceId` 建议使用持久化的 32 位十六进制随机值。密码登录示例：

```bash
curl -sS 'http://192.168.1.10:18090/yinqiao/v1/auth/login/password' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","deviceId":"0123456789abcdef0123456789abcdef"}'
```

Yinqiao API 接受明文密码或 64 位 SHA-256 字符串，并适配飞牛音乐内部密码格式。生产环境传明文密码时必须使用 HTTPS 或可信局域网。

## 5. 用户与权限（7）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/users` | 管理员 | 无 | 用户数组或 `{list}` |
| `GET` | `/admin/users/exists` | 管理员 | Query `username` | `{isExist}` |
| `POST` | `/admin/users` | 管理员 | `{username,password,sharedLibraryAccess}` | 创建成员用户 |
| `POST` | `/admin/users/update` | 管理员 | `{guid,username,password?,sharedLibraryAccess}` | 编辑用户 |
| `POST` | `/account/password` | 用户 | `{password}` | 修改当前用户密码 |
| `POST` | `/admin/users/delete` | 管理员 | `{guid}` | 删除/停用用户 |
| `POST` | `/admin/users/unban` | 管理员 | `{guid}` | 解除用户封禁 |

曲库权限示例：

```json
{
  "username": "bob",
  "password": "secret123",
  "sharedLibraryAccess": {
    "mode": "partial",
    "guids": ["library-guid"]
  }
}
```

## 6. 服务器设置（4）

| Method | Path | 权限 | JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/settings/server` | 管理员 | - | `{name,lang}` |
| `POST` | `/admin/settings/server` | 管理员 | `{name,lang}` | 更新服务器设置 |
| `GET` | `/admin/settings/default-user-access` | 管理员 | - | `{defaultSharedLibraryAccess}` |
| `POST` | `/admin/settings/default-user-access` | 管理员 | `{defaultSharedLibraryAccess:{mode,guids}}` | 更新新用户默认曲库权限 |

## 7. fnOS 授权目录（2）

| Method | Path | 权限 | Query | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/directories` | 管理员 | 无 | `{list:AuthorizedDir[]}` |
| `GET` | `/admin/directories/children` | 管理员 | `parent=/vol1/music` | `{list:Array<{path,name}>}` |

这里只能看到 fnOS 已授权给飞牛音乐应用的目录，不会绕过 fnOS 文件权限。

## 8. 共享音乐库（7）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/libraries` | 管理员 | 无 | `SharedLibrary[]` 或 `{list}` |
| `GET` | `/admin/libraries/detail` | 管理员 | Query `guid` | `SharedLibrary` |
| `POST` | `/admin/libraries` | 管理员 | `{path,autoDownloadLyric,metadataPreference}` | 创建曲库 |
| `POST` | `/admin/libraries/update` | 管理员 | `{guid,path,autoDownloadLyric,metadataPreference}` | 更新曲库 |
| `POST` | `/admin/libraries/delete` | 管理员 | `{guid}` | 删除曲库 |
| `POST` | `/admin/libraries/scan` | 管理员 | `{guid}` | 扫描单个曲库 |
| `POST` | `/admin/libraries/scan-all` | 管理员 | 空 body | 扫描全部曲库 |

创建示例：

```json
{
  "path": "/vol1/music",
  "autoDownloadLyric": true,
  "metadataPreference": "cloud_preferred"
}
```

## 9. 曲目目录（5）

所有接口都需要用户权限，返回 `{list:Track[],total,sort?}`。

| Method | Path | Query | 说明 |
| --- | --- | --- | --- |
| `GET` | `/catalog/tracks` | `page,size,sort?` | 全部可访问曲目 |
| `GET` | `/catalog/tracks/by-album` | `albumGUID,page,size,sort?` | 指定专辑曲目 |
| `GET` | `/catalog/tracks/by-artist` | `artistGUID,page,size,sort?` | 指定艺术家曲目 |
| `GET` | `/catalog/tracks/by-genre` | `genreGUID,page,size,sort?` | 指定风格曲目 |
| `GET` | `/catalog/tracks/by-playlist` | `playlistGUID,page,size,sort?` | 指定歌单曲目 |

## 10. 专辑、艺术家和风格（10）

### 10.1 专辑（3）

| Method | Path | 权限 | Query | 返回 |
| --- | --- | --- | --- | --- |
| `GET` | `/catalog/albums` | 用户 | `page,size,sort?` | 专辑分页 |
| `GET` | `/catalog/albums/by-artist` | 用户 | `artistGUID,page,size,sort?` | 艺术家专辑分页 |
| `GET` | `/catalog/albums/detail` | 用户 | `guid` | `Album` |

### 10.2 艺术家（4）

| Method | Path | 权限 | Query/JSON body | 返回 |
| --- | --- | --- | --- | --- |
| `POST` | `/admin/artists` | 管理员 | `{name}` | 新建 `Artist` |
| `GET` | `/catalog/artists` | 用户 | `page,size,sort?` | 艺术家分页 |
| `GET` | `/catalog/artists/all` | 用户 | 无 | `{list:Artist[]}` |
| `GET` | `/catalog/artists/detail` | 用户 | `guid` | `Artist` |

### 10.3 风格（3）

| Method | Path | 权限 | Query/JSON body | 返回 |
| --- | --- | --- | --- | --- |
| `POST` | `/admin/genres` | 管理员 | `{name}` | 新建 `Genre` |
| `GET` | `/catalog/genres` | 用户 | `page,size,sort?` | 风格分页 |
| `GET` | `/catalog/genres/detail` | 用户 | `guid` | `Genre` |

## 11. 曲目元数据（2）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/tracks/metadata` | 管理员 | Query `guid` | 完整曲目元数据 |
| `POST` | `/admin/tracks/metadata` | 管理员 | 见下方 | 更新曲目元数据 |

```json
{
  "guid": "track-guid",
  "title": "歌曲标题",
  "artistGUIDs": ["artist-guid"],
  "album": "专辑名",
  "genreGUIDs": ["genre-guid"],
  "year": 2026,
  "discNo": 1,
  "trackNo": 3,
  "coverId": "cover-id-or-null"
}
```

不修改的可空字段可以省略；需要明确清空的字段传 `null`。

## 12. 封面与媒体（3）

| Method | Path | 权限 | 参数 | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/media/covers/{coverId}` | 媒体 | Query `size?` | 图片二进制，例如 `size=640` |
| `POST` | `/admin/tracks/cover` | 管理员 | multipart 字段 `file` | JSON `data` 至少含 `{coverId}` |
| `POST` | `/me/playlists/cover` | 用户 | multipart 字段 `file` | JSON `data` 至少含 `{coverId}` |

## 13. 播放、HLS、转码与漫游（8）

### 13.1 原始流和 HLS（2）

| Method | Path | 权限 | 参数 | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/media/tracks/{guid}/stream` | 媒体 | Header `Range?` | 原音频或 CUE 区段，支持 `200/206` |
| `GET` | `/media/tracks/{guid}/hls/{filename}` | 媒体 | `filename` 首次使用 `preset.m3u8` | HLS 播放列表或分片 |

Yinqiao API 透传 `Content-Type`、`Content-Length`、`Content-Range`、`Accept-Ranges`、`ETag`、`Last-Modified` 和缓存相关响应头。

### 13.2 转码会话（3）

| Method | Path | 权限 | JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `POST` | `/playback/transcode/start` | 用户 | `{guid,output:{codec,bitrate,channel}}` | 启动/准备转码 |
| `POST` | `/playback/transcode/heartbeat` | 用户 | `{guid,timestamp}` | 维持会话，建议约 10 秒一次 |
| `POST` | `/playback/transcode/stop` | 用户 | `{guid}` | 结束并释放转码资源 |

### 13.3 漫游播放（3）

| Method | Path | 权限 | Query | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/playback/roam/start` | 用户 | `deviceId` | `{current,next?}` |
| `GET` | `/playback/roam/next` | 用户 | `deviceId,relativeRoamId` | `{current,next?,previous?}` |
| `GET` | `/playback/roam/previous` | 用户 | `deviceId,relativeRoamId` | `{current,next?,previous?}` |

每个位置为 `{track,roamId}`。切换歌曲时使用上一次返回的 `roamId`，不是曲目 GUID。

## 14. 歌词和活动事件（2）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/catalog/lyrics` | 用户 | Query `trackGUID` | `{list:[{guid,content,source,updatedAt?}],preferred?}` |
| `POST` | `/activity/events` | 用户 | `{events:[...]}` | 批量上报播放和偏好事件 |

歌词 `content` 是 LRC 或纯文本。`source` 当前含义：`1` 内嵌、`2` 同名歌词文件、`3` 抓取、`4` 用户关联。

```json
{
  "events": [
    {
      "eventType": "track_play",
      "occurredAt": 1785370000000,
      "payload": {"trackGUID": "track-guid"}
    }
  ]
}
```

支持的事件类型：

| `eventType` | `payload` |
| --- | --- |
| `track_play` | `{trackGUID}` |
| `lyric_offset_change` | `{trackGUID,lyricGUID,offset}`，`offset` 为毫秒 |
| `lyric_preference_change` | 曲目 GUID 和首选歌词 GUID |
| `sorting_change` | 列表标识和排序值 |

## 15. 收藏和历史（7）

### 15.1 收藏（5）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/me/favorites` | 用户 | `page,size,sort?` | 收藏曲目分页 |
| `POST` | `/me/favorites` | 用户 | `{trackGUID}` | 收藏曲目 |
| `POST` | `/me/favorites/remove` | 用户 | `{trackGUID}` | 取消收藏 |
| `GET` | `/me/favorites/purge-count` | 用户 | 无 | `{total}` |
| `POST` | `/me/favorites/purge` | 用户 | 空 body | 清理失效收藏 |

### 15.2 播放历史（2）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/me/history` | 用户 | `page,size,sort?` | 播放历史分页 |
| `POST` | `/me/history/delete` | 用户 | `{trackGUIDs:[...]}` | 删除指定历史记录 |

上报 `track_play` 会新增历史并影响播放次数。历史批量删除在旧 `0.9.11` 后台上缺少完整官方调用样本，上线前应针对目标 NAS 版本做集成验证。

## 16. 歌单（10）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/me/playlists` | 用户 | 无 | `{list:Playlist[]}` |
| `GET` | `/me/playlists/batch-detail` | 用户 | `guids=a,b,c` | `{list:[{guid,trackCount}]}` |
| `POST` | `/me/playlists` | 用户 | `{name,coverId?}` | 创建并返回歌单 |
| `POST` | `/me/playlists/update` | 用户 | `{guid,name,coverId?}` | 更新歌单 |
| `POST` | `/me/playlists/delete` | 用户 | `{guid}` | 删除歌单 |
| `GET` | `/me/playlists/detail` | 用户 | Query `guid` | `Playlist` |
| `POST` | `/me/playlists/tracks/add` | 用户 | `{guid,trackGUIDs:[...]}` | 批量添加曲目 |
| `POST` | `/me/playlists/tracks/remove` | 用户 | `{guid,trackGUIDs:[...]}` | 批量移除曲目 |
| `GET` | `/me/playlists/purge-count` | 用户 | Query `guid` | `{total}` |
| `POST` | `/me/playlists/purge` | 用户 | `{guid}` | 清理歌单中的失效曲目 |

## 17. 搜索（6）

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/search/tracks` | 用户 | `q,page,size` | 曲目分页 |
| `GET` | `/search/albums` | 用户 | `q,page,size` | 专辑分页 |
| `GET` | `/search/artists` | 用户 | `q,page,size` | 艺术家分页 |
| `GET` | `/search/playlists` | 用户 | `q,page,size` | 歌单分页 |
| `GET` | `/search/suggest` | 用户 | `q` | 四类 `{total,items}` 联想结果 |
| `POST` | `/admin/search/rebuild` | 管理员 | 空 body | 重建索引，返回 `{albumCount,artistCount,trackCount}` |

索引重建可能创建后台任务；已在重建时上游可能返回 `UPSTREAM_140001`。

## 18. 后台任务（4）

```ts
interface Task {
  id: string;
  type: string;
  name: string;
  total: number;
  successCount: number;
  failCount: number;
  done: boolean;
  cancelling: boolean;
  canceled: boolean;
  canceledCount: number;
  createdAt: number;
  doneAt: number;
  retryable?: boolean;
  ext?: {libraryGUID?: string; taskIds?: string[]};
}
```

| Method | Path | 权限 | Query/JSON body | 返回/行为 |
| --- | --- | --- | --- | --- |
| `GET` | `/admin/tasks` | 管理员 | 无 | `Task[]` 或 `{list}` |
| `POST` | `/admin/tasks/cancel` | 管理员 | `{taskId}` | 请求取消任务 |
| `POST` | `/admin/tasks/delete` | 管理员 | `{taskId}` | 删除任务记录 |
| `POST` | `/admin/tasks/retry` | 管理员 | `{taskId}` | 重试失败任务 |

## 19. 通用官方 API 透传

尚未进入 85 条稳定路由的飞牛音乐接口，可通过以下入口调用：

```text
GET|POST|PUT|PATCH|DELETE /upstream/{officialApiPath}
```

例如飞牛音乐内部接口 `/music/api/v1/sys/info`：

```bash
curl 'http://192.168.1.10:18090/yinqiao/v1/upstream/sys/info' \
  -H 'Authorization: Bearer <userToken>'
```

Yinqiao API 会生成内部 `authx`，把 Bearer 登录态转换为上游 Cookie，并保留 Query、请求体、HTTP 状态、二进制内容和 Range 头。需要注意：

- 返回的是飞牛音乐**原始响应**，不使用 `{ok,data,error,meta}` 包装。
- 调用者权限仍由飞牛音乐上游校验，透传不会绕过用户或管理员权限。
- 路径必须规范化，目录穿越会被拒绝。
- 该入口用于兼容新版本和未归一化能力；长期客户端应优先使用稳定路由。

## 20. Yinqiao API 完整调用示例

```bash
# 1. 登录并从响应中取得 data.userToken
curl -sS 'http://192.168.1.10:18090/yinqiao/v1/auth/login/password' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","deviceId":"0123456789abcdef0123456789abcdef"}'

# 2. 浏览曲目
curl -sS 'http://192.168.1.10:18090/yinqiao/v1/catalog/tracks?page=1&size=50&sort=createdAt%2Cdesc' \
  -H 'Authorization: Bearer <userToken>'

# 3. Range 播放
curl 'http://192.168.1.10:18090/yinqiao/v1/media/tracks/<guid>/stream' \
  -H 'Authorization: Bearer <userToken>' \
  -H 'Range: bytes=0-1048575' \
  --output part.audio
```

---

# 第二部分：OpenSubsonic/Subsonic API

## 21. 连接方式

播放器中填写：

```text
服务器：http://NAS_IP
端口：14040
用户名：fnmusic 用户名
密码：fnmusic 密码
```

通常不要在播放器地址中追加 `/rest`；播放器会自动请求 `/rest/{method}.view`。

自研客户端的标准端点：

```text
GET|POST http://NAS_IP:14040/rest/{method}.view
```

- `GET` 使用 URL Query。
- `POST` 使用 `application/x-www-form-urlencoded` 表单。
- XML 是默认响应格式。
- `f=json` 返回 JSON；`f=jsonp&callback=name` 返回 JSONP。
- 当前协议版本字段为 `1.16.1`，服务端版本字段为 `1.0.5`。

## 22. Subsonic 鉴权

除 `getOpenSubsonicExtensions` 外，所有 `/rest/` 方法都要求：

| 参数 | 必需 | 说明 |
| --- | --- | --- |
| `u` | 是 | fnmusic 用户名 |
| `p` | 二选一 | 明文密码，或 `enc:` 加密码 UTF-8 字节的十六进制 |
| `t` + `s` | 二选一 | `t = md5(password + salt)`，`s` 为客户端随机盐 |
| `v` | 标准客户端应传 | 客户端声明的 Subsonic API 版本；当前服务端不强制校验 |
| `c` | 标准客户端应传 | 客户端名称；用于日志定位 |
| `f` | 否 | `xml`、`json` 或 `jsonp`；默认 XML |

### 22.1 密码模式

第一次成功使用 `p` 登录时，网关会直接调用 Yinqiao API 的 fnmusic 登录接口，并把凭据加密登记到本机凭据库。

```bash
curl -sS 'http://192.168.1.10:14040/rest/ping.view?u=alice&p=secret123&v=1.16.1&c=myapp&f=json'
```

`enc:` 示例：密码 `abc` 的 UTF-8 十六进制是 `616263`，所以参数为 `p=enc:616263`。这只是协议编码，不是安全加密，外网仍必须使用 HTTPS。

### 22.2 Token + Salt 模式

```text
s = 每次请求生成的随机字符串
t = MD5(原始密码 + s)，输出小写十六进制
```

摘要无法还原原密码，而网关必须用原密码登录 fnmusic，所以该模式首次使用前需要登记一次凭据：

```bash
curl -sS -X POST 'http://192.168.1.10:14040/yinqiao/enroll' \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}'
```

成功响应：

```json
{"ok":true,"username":"alice"}
```

登记接口会先向 fnmusic 实际验证账号密码，再加密保存在 NAS 本机。使用 `p` 成功登录也会自动完成登记。

## 23. 音桥服务管理接口

这些接口不在 `/rest` 下，不使用 Subsonic 响应包装。

| Method | Path | 参数 | 返回/行为 |
| --- | --- | --- | --- |
| `GET` | `/`、`/yinqiao` | 无 | JSON：音桥版本、协议路径、登记和状态入口 |
| `GET` | `/yinqiao/status` | 无 | JSON：版本、Yinqiao API ready 状态、已绑定用户名、协议开关 |
| `GET` | `/yinqiao/network` | 无 | JSON：自动识别的 NAS 局域网播放器地址、候选私网地址和协议端口 |
| `GET` | `/yinqiao/enroll` | 无 | 返回凭据登记 HTML 页面 |
| `POST` | `/yinqiao/enroll` | JSON/Form `{username,password}` | 验证并登记 fnmusic 凭据 |
| `GET` | `/yinqiao/accounts` | 无 | 返回所有已绑定用户名，不返回密码 |
| `POST` | `/yinqiao/accounts` | JSON `{username,password}` | 验证并新增或更新一个 fnmusic 账号 |
| `DELETE` | `/yinqiao/accounts?username=alice` | 查询参数或 JSON `{username}` | 删除一个账号并清理其协议会话 |
| `GET` | `/yinqiao/settings` | 无 | 返回客户端端口、主机覆盖和四种协议开关 |
| `PUT` | `/yinqiao/settings` | JSON，可提交完整设置或部分字段 | 持久化设置并即时应用；端口占用返回 `409 PORT_UNAVAILABLE` |

设置请求示例：

```json
{
  "protocolPort": 15050,
  "advertisedHost": "192.168.1.10",
  "protocols": {
    "openSubsonic": true,
    "synology": true,
    "jellyfin": true,
    "ampache": false
  }
}
```

`14040` 固定保留为 fnOS 管理入口；`protocolPort` 是播放器使用的附加监听端口，
允许 `1024-65535`，但不能使用 Yinqiao API 保留端口 `18090`。主机留空时自动选择私网 IP。

状态响应示例：

```json
{
  "ok": true,
	"version": "1.0.5",
  "bridge": {"status":"ready","upstream":"ready"},
  "boundUsers": ["alice"],
  "protocols": {
    "openSubsonic": {"enabled":true,"version":"1.16.1"},
    "ampache": {"enabled":true,"version":"6.0.0"}
  }
}
```

网络信息响应示例：

```json
{
  "ok": true,
  "address": "http://NAS_IP:14040",
  "addresses": ["http://NAS_IP:14040"],
  "port": "14040"
}
```

## 24. Subsonic 响应格式

JSON 成功响应：

```json
{
  "subsonic-response": {
    "status": "ok",
    "version": "1.16.1",
    "type": "yinqiao",
	"serverVersion": "1.0.5",
    "openSubsonic": true
  }
}
```

JSON 失败响应：

```json
{
  "subsonic-response": {
    "status": "failed",
    "version": "1.16.1",
    "type": "yinqiao",
	"serverVersion": "1.0.5",
    "openSubsonic": true,
    "error": {"code":40,"message":"Wrong username or password"}
  }
}
```

Subsonic 业务错误通常仍返回 HTTP `200`，客户端必须检查 `subsonic-response.status`，不能只检查 HTTP 状态。

| Subsonic code | 含义 |
| ---: | --- |
| `0` | 上游或网关通用错误 |
| `10` | 必需参数缺失、参数不合法或不支持的列表类型 |
| `40` | fnmusic 用户名或密码错误 |
| `41` | Token 登录尚未完成一次凭据登记 |
| `50` | 当前 fnmusic 用户没有该操作权限 |
| `70` | 资源不存在或方法未实现 |

## 25. 已实际实现的 Subsonic 方法（35）

表中的参数不包含每次请求都要携带的公共鉴权参数 `u/p` 或 `u/t/s`、`v`、`c`、`f`。

### 25.1 探测、用户和能力（5）

| Method | 参数 | 返回节点 | 实际行为 |
| --- | --- | --- | --- |
| `ping` | 无 | 无业务节点 | 验证协议服务和 fnmusic 登录 |
| `getLicense` | 无 | `license` | 固定返回 `valid=true` |
| `getMusicFolders` | 无 | `musicFolders.musicFolder[]` | 固定返回根目录 `{id:"fnmusic",name:"fnmusic"}` |
| `getUser` | `username?` | `user` | 返回当前 fnmusic 用户及映射角色；不能查询其他用户 |
| `getOpenSubsonicExtensions` | 无，且无需鉴权 | `openSubsonicExtensions[]` | 声明 `songLyrics` v1 和 `formPost` v1 |

`getUser` 的管理员角色来自 fnmusic `admin/administrator`；支持流、下载、封面和歌单，不声明上传、播客、分享、视频转码或点唱机角色。

### 25.2 浏览目录（7）

| Method | 参数 | 返回节点 | Yinqiao API 映射/行为 |
| --- | --- | --- | --- |
| `getArtists` | 无 | `artists.index[].artist[]` | `/catalog/artists/all`，按艺术家名首字符分组 |
| `getIndexes` | 无 | `indexes.index[].artist[]` | 与 `getArtists` 相同，仅根节点名不同 |
| `getArtist` | `id` | `artist.album[]` | 艺术家详情 + 该艺术家的专辑 |
| `getAlbum` | `id` | `album.song[]` | 专辑详情 + 最多 10000 首曲目 |
| `getSong` | `id` | `song` | 返回单曲；管理员优先查元数据，普通用户从可见曲库分页定位 |
| `getMusicDirectory` | `id` | `directory.child[]` | `id=fnmusic` 返回最多 500 张专辑；其他 ID 作为专辑 GUID 返回曲目 |
| `getGenres` | 无 | `genres.genre[]` | 最多读取 1000 个风格，含歌曲数、专辑数和 GUID |

`getSong` 对尚未进入进程内缓存的普通用户可能需要分页扫描曲库，曲库很大时首次调用会较慢；成功浏览/搜索过的曲目会进入缓存。

### 25.3 专辑和曲目列表（6）

| Method | 参数 | 返回节点 | 实际行为和限制 |
| --- | --- | --- | --- |
| `getAlbumList` | `type,size?,offset?` | `albumList.album[]` | 旧版目录结构 |
| `getAlbumList2` | `type,size?,offset?` | `albumList2.album[]` | ID3 目录结构；`size` 默认 10，范围 1-500 |
| `getRandomSongs` | `size?` | `randomSongs.song[]` | 从前 `max(size*4,100)` 首可见曲目中随机抽取；`size` 默认 10、最大 500 |
| `getSongsByGenre` | `genre,size?,offset?` | `songsByGenre.song[]` | 先把风格名解析为 GUID，再读取曲目；当前实现读取 `size`，不是标准参数 `count` |
| `getStarred` | `size?,offset?` | `starred.song[]` | 返回当前用户收藏曲目 |
| `getStarred2` | `size?,offset?` | `starred2.song[]` | 与 `getStarred` 相同 |

`getAlbumList/getAlbumList2` 的 `type` 当前处理如下：

| `type` | 当前语义 |
| --- | --- |
| `alphabeticalByName` | 按专辑名升序 |
| `alphabeticalByArtist` | 请求上游按艺术家升序 |
| `newest`、`recent`、`frequent`、`highest`、`starred`、`random`、空值 | 都按 `createdAt,desc` 返回；尚未实现各自标准语义 |
| 其他值 | code `10`，`Unsupported album list type` |

`getRandomSongs` 当前忽略标准筛选参数 `genre`、`fromYear`、`toYear`、`musicFolderId`。

### 25.4 搜索（2）

| Method | 参数 | 返回节点 | 实际行为 |
| --- | --- | --- | --- |
| `search2` | `query,artistCount?,artistOffset?,albumCount?,albumOffset?,songCount?,songOffset?` | `searchResult2` | 同时搜索艺术家、专辑和曲目 |
| `search3` | 同上 | `searchResult3` | 与 `search2` 相同，使用新版根节点 |

- 各 `Count` 默认 `20`，范围 `0-500`；设为 `0` 可跳过对应类型。
- 各 `Offset` 默认 `0`。
- `query=*` 或空值执行浏览。
- 若飞牛音乐搜索接口对某类请求返回参数错误，网关会退回目录列表并在本地过滤。
- 当前没有把 `musicFolderId` 用作额外过滤条件。

### 25.5 歌单（5）

| Method | 参数 | 返回节点 | Yinqiao API 映射/行为 |
| --- | --- | --- | --- |
| `getPlaylists` | 无 | `playlists.playlist[]` | 当前用户歌单列表 |
| `getPlaylist` | `id` | `playlist.entry[]` | 歌单详情 + 最多 10000 首曲目 |
| `createPlaylist` | `name?`,`playlistId?`,`songId*` | `playlist.entry[]` | 无 `playlistId` 时按 `name` 新建；有 ID 时向指定歌单追加歌曲 |
| `updatePlaylist` | `playlistId`,`name?`,`songIdToAdd*`,`songIndexToRemove*` | 无业务节点 | 改名、批量添加、按当前歌单的零基下标移除 |
| `deletePlaylist` | `id` | 无业务节点 | 删除当前用户歌单 |

带 `*` 的参数可重复出现。创建新歌单时网关优先使用第一首歌曲的封面；找不到时尝试使用曲库中的专辑封面。

### 25.6 收藏和播放上报（3）

| Method | 参数 | 返回节点 | Yinqiao API 映射/行为 |
| --- | --- | --- | --- |
| `star` | `id*` | 无业务节点 | 把每个曲目 GUID 加入 fnmusic 收藏 |
| `unstar` | `id*` | 无业务节点 | 逐个取消收藏 |
| `scrobble` | `id*`,`time*`,`submission?` | 无业务节点 | 批量上报 `track_play`；`time` 为 Unix 毫秒，缺省取当前时间 |

当前 `star/unstar` 只处理曲目参数 `id`，忽略 `albumId` 和 `artistId`。`scrobble` 接受但不区分 `submission` 语义。

### 25.7 音频、下载和封面（3）

| Method | 参数 | 返回 | Yinqiao API 映射/行为 |
| --- | --- | --- | --- |
| `stream` | `id` | 音频二进制 | 原音频/CUE 区段，支持 Range |
| `download` | `id` | 音频二进制 | 当前与 `stream` 使用同一原始流，不强制 attachment |
| `getCoverArt` | `id`,`size?` | 图片二进制 | ID 可为封面、曲目、专辑、艺术家或歌单 GUID；网关会尝试解析真实 coverId |

媒体接口转发 `200/206`、`Content-Type`、`Content-Length`、`Content-Range`、`Accept-Ranges`、`Content-Disposition`、`ETag`、`Last-Modified` 和 `Cache-Control`。当前忽略 Subsonic 转码参数 `maxBitRate`、`format`、`estimateContentLength`、`converted`。

### 25.8 歌词（2）

| Method | 参数 | 返回节点 | 实际行为 |
| --- | --- | --- | --- |
| `getLyrics` | `title?`,`artist?` | `lyrics` | 搜索最匹配曲目，返回原始 LRC/纯文本 `value`；找不到时成功返回空歌词 |
| `getLyricsBySongId` | `id` | `lyricsList.structuredLyrics[]` | OpenSubsonic `songLyrics` v1；将 LRC 转为逐行歌词 |

结构化歌词行格式：

```json
{
  "lyricsList": {
    "structuredLyrics": [{
      "displayArtist": "",
      "displayTitle": "",
      "lang": "und",
      "synced": true,
      "line": [
        {"start": 45338, "value": "歌词文本"}
      ]
    }]
  }
}
```

`start` 是毫秒。纯文本歌词的 `synced=false`，行不带 `start`。

### 25.9 扫描（2）

| Method | 参数 | 返回节点 | Yinqiao API 映射/行为 |
| --- | --- | --- | --- |
| `getScanStatus` | 无 | `scanStatus` | 从后台任务推导 `{scanning,count}` |
| `startScan` | 无 | `scanStatus` | 调用扫描全部共享曲库 |

这两项依赖 Yinqiao API 管理员接口，因此应使用 fnmusic 管理员账号；普通用户会收到 code `50`。

## 26. 空集合兼容方法（16）

以下方法被识别并返回 `status=ok`，但当前**没有桥接实际业务数据**。它们只用于避免部分播放器在探测可选能力时直接报错。

| Method | 返回空节点 | 当前实际能力 |
| --- | --- | --- |
| `getNowPlaying` | `nowPlaying` | 无实时在线播放列表 |
| `getBookmarks` | `bookmarks` | 无书签数据 |
| `getInternetRadioStations` | `internetRadioStations` | 无网络电台数据 |
| `getShares` | `shares` | 无分享数据 |
| `getVideos` | `videos` | 无视频数据 |
| `getPodcasts` | `podcasts` | 无播客数据 |
| `getNewestPodcasts` | `newestPodcasts` | 无最新播客数据 |
| `getPlayQueue` | `playQueue` | 无服务端播放队列 |
| `getChatMessages` | `chatMessages` | 无聊天数据 |
| `getSimilarSongs` | `similarSongs` | 无相似歌曲算法 |
| `getSimilarSongs2` | `similarSongs2` | 无相似歌曲算法 |
| `getTopSongs` | `topSongs` | 无热门歌曲统计 |
| `getArtistInfo` | `artistInfo` | 无外部艺术家简介 |
| `getArtistInfo2` | `artistInfo2` | 无外部艺术家简介 |
| `getAlbumInfo` | `albumInfo` | 无外部专辑简介 |
| `getAlbumInfo2` | `albumInfo2` | 无外部专辑简介 |

客户端不能把 `status=ok` 理解为这些数据已经持久化或生效。

## 27. 无操作成功方法（4）

以下方法返回 `status=ok`，但不修改 Yinqiao API 或 fnmusic 数据：

| Method | 被忽略的典型参数 | 当前实际能力 |
| --- | --- | --- |
| `setRating` | `id,rating` | 不保存评分 |
| `savePlayQueue` | `id*,current,position` | 不保存队列 |
| `createBookmark` | `id,position,comment` | 不创建书签 |
| `deleteBookmark` | `id` | 不删除书签 |

## 28. 未实现方法

除第 25-27 节列出的方法外，其余 `/rest/{method}.view` 返回：

```json
{
  "subsonic-response": {
    "status": "failed",
    "error": {
      "code": 70,
      "message": "Subsonic method is not implemented: <method>"
    }
  }
}
```

当前没有实现的主要协议族包括：播客管理、分享管理、网络电台写操作、聊天写操作、书签持久化、播放队列持久化、相似度推荐、Jukebox、视频转码和服务端播放控制。

## 29. Subsonic 数据对象字段

### 29.1 Song

实际有值的字段才会输出，固定布尔/评分字段除外。

```ts
interface SubsonicSong {
  id: string;                // fnmusic Track.guid
  parent?: string;           // Album.guid
  isDir: false;
  title: string;
  album?: string;
  artist?: string;
  albumId?: string;
  artistId?: string;         // 第一位艺术家 GUID
  coverArt?: string;
  duration?: number;         // 秒
  track?: number;
  discNumber?: number;
  year?: number;
  size?: number;             // 字节
  bitRate?: number;          // kbit/s
  samplingRate?: number;
  bitDepth?: number;
  channelCount?: number;
  suffix?: string;
  contentType?: string;
  created?: string;          // RFC 3339
  starred?: string;          // 收藏时间，RFC 3339
  type: "music";
  isVideo: false;
  path?: string;
  userRating: 0;
  averageRating: 0;
}
```

### 29.2 Album、Artist、Playlist

```ts
interface SubsonicAlbum {
  id: string;
  name: string;
  artist?: string;
  artistId?: string;
  coverArt?: string;
  songCount?: number;
  duration?: number;         // 秒
  year?: number;
  created?: string;
}

interface SubsonicArtist {
  id: string;
  name: string;
  coverArt?: string;
  albumCount?: number;
}

interface SubsonicPlaylist {
  id: string;
  name: string;
  owner: string;
  public: false;
  songCount?: number;
  duration?: number;         // 秒
  created?: string;
  changed?: string;
  coverArt?: string;
}
```

## 30. Subsonic 调用示例

### 30.1 JSON 探测和浏览

```bash
BASE='http://192.168.1.10:14040'
AUTH='u=alice&p=secret123&v=1.16.1&c=myapp&f=json'

curl -sS "$BASE/rest/ping.view?$AUTH"
curl -sS "$BASE/rest/getArtists.view?$AUTH"
curl -sS "$BASE/rest/search3.view?$AUTH&query=周杰伦&songCount=20&albumCount=10&artistCount=10"
```

### 30.2 播放和封面

```bash
curl "$BASE/rest/stream.view?$AUTH&id=<track-guid>" \
  -H 'Range: bytes=0-1048575' \
  --output part.audio

curl "$BASE/rest/getCoverArt.view?$AUTH&id=<cover-or-entity-guid>&size=640" \
  --output cover.jpg
```

### 30.3 表单 POST 创建歌单

```bash
curl -sS -X POST "$BASE/rest/createPlaylist.view" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'u=alice' \
  --data-urlencode 'p=secret123' \
  --data-urlencode 'v=1.16.1' \
  --data-urlencode 'c=myapp' \
  --data-urlencode 'f=json' \
  --data-urlencode 'name=测试歌单' \
  --data-urlencode 'songId=<track-guid-1>' \
  --data-urlencode 'songId=<track-guid-2>'
```

## 31. Yinqiao API 与 Subsonic 能力对应

| 需求 | 建议接口 | 原因 |
| --- | --- | --- |
| 登录、用户、曲库权限 | Yinqiao API | 能拿到完整 fnmusic 用户和权限模型 |
| 浏览、搜索、播放、歌词 | 两者均可 | Subsonic 适合通用播放器，Yinqiao API 字段更完整 |
| 收藏、歌单、播放上报 | 两者均可 | Subsonic 已映射常见写操作 |
| 用户管理、服务器设置 | Yinqiao API | Subsonic 没有对应后台管理模型 |
| 添加目录、创建/扫描曲库 | Yinqiao API | 需要 fnmusic 管理员能力 |
| 编辑元数据、上传封面 | Yinqiao API | Subsonic 当前不提供这些写接口 |
| HLS、转码会话、漫游播放 | Yinqiao API | Subsonic 当前仅转发原始音频流 |
| 任务取消、删除、重试 | Yinqiao API | Subsonic 只提供扫描开始和状态 |
| 现成手机/桌面播放器接入 | Subsonic | 客户端无需理解飞牛音乐私有模型 |

## 32. 客户端实现注意事项

1. Yinqiao API 客户端只调用 `/yinqiao/v1`，不要直接依赖内部 `/music/api/v1` 或自行生成 `authx`。
2. Yinqiao API JSON 同时检查 HTTP 状态和 `ok`；Subsonic 同时检查 HTTP 状态和 `subsonic-response.status`。
3. 音频必须使用流式读取和 Range，不要把完整音频加载进普通 JSON 请求。
4. Yinqiao API Token 使用安全存储；Subsonic 外网请求必须走 HTTPS，因为兼容客户端可能直接发送密码。
5. 用户最终能看到哪些曲目，始终由 fnmusic 账号的曲库权限决定。
6. 管理功能只对 fnmusic 管理员开放，协议层不会提升权限。
7. 底层飞牛音乐升级后，稳定 Yinqiao API 路由由 Yinqiao API 负责适配；通用透传返回原始模型，客户端自行承担版本变化。
8. 第 26、27 节方法是客户端兼容占位，不应作为数据确实存在或操作确实落库的依据。

---

# 第三部分：Synology Audio Station 兼容 API

## 33. 连接与鉴权

服务器根地址：

```text
http://NAS_IP:14040
```

发现接口：

```http
GET /webapi/query.cgi?api=SYNO.API.Info&version=1&method=query&query=all
```

登录接口接受 JSON、表单或 Query 参数：

```http
POST /webapi/entry.cgi
Content-Type: application/json

{
  "api": "SYNO.API.Auth",
  "version": 6,
  "method": "login",
  "session": "audiostation",
  "account": "fnmusic用户名",
  "passwd": "fnmusic密码"
}
```

成功返回 `data.sid`，之后放在 `_sid` 参数或 `id` Cookie 中。密码由协议层转发给
Yinqiao API 登录接口验证，客户端不会得到飞牛音乐 Token。

## 34. 已声明的 Audio Station API

| API | 支持方法 | 关键参数 |
| --- | --- | --- |
| `SYNO.API.Info` | `query` | `query=all`、精确 API 或 `SYNO.AudioStation.*` |
| `SYNO.API.Auth` | `login`, `logout` | `account`, `passwd`, `_sid` |
| `SYNO.AudioStation.Info` | `getinfo` | 无 |
| `SYNO.AudioStation.Album` | `list` | `offset`, `limit`, `filter`, `artist` |
| `SYNO.AudioStation.Artist` | `list` | `offset`, `limit`, `filter` |
| `SYNO.AudioStation.Song` | `list`, `search`, `getinfo`, `setrating` | `id`, `keyword`, `album`, `artist`, `genre`, `rating_filter`, `rating` |
| `SYNO.AudioStation.Genre` | `list` | `offset`, `limit` |
| `SYNO.AudioStation.Folder` | `list`, `getinfo` | `id`, `offset`, `limit` |
| `SYNO.AudioStation.Search` | `list` | `keyword`, `offset`, `limit` |
| `SYNO.AudioStation.Cover` | `getsongcover`, `getcover` | `id` 或专辑/歌手名称 |
| `SYNO.AudioStation.Stream` | `stream` | `id`, `_sid`，支持 Range |
| `SYNO.AudioStation.Download` | `download` | `id`, `_sid` |
| `SYNO.AudioStation.Lyrics` | `getlyrics` | `id` |
| `SYNO.AudioStation.Playlist` | `list`, `getinfo`, `create`, `delete`, `rename`, `updatesongs` | `id`, `name`, `new_name`, `songs`, `offset`, `limit` |

成功响应统一为 `{"success":true,"data":...}`。协议错误返回
`{"success":false,"error":{"code":数字,"message":"..."}}`。

## 35. Audio Station 不支持

`QuickConnect`、OTP、DSM 管理、`RemotePlayer`、`MediaServer`、`Radio`、
`LyricsSearch` 插件、智能歌单、歌单分享、`WebPlayer` 队列、作曲家索引和实时 MP3
转码均为 **不支持**。这些模块不会出现在 `SYNO.API.Info` 中。

---

# 第四部分：Jellyfin Audio 兼容 API

## 36. 连接与鉴权

服务器通常填写 `http://NAS_IP:14040`。使用反向代理时也支持 `/Audio` 基础路径，
例如 `https://example.com/Audio`；服务发现、登录、浏览、图片和音频请求都可携带该前缀。

```http
POST /Users/AuthenticateByName
Content-Type: application/json

{"Username":"fnmusic用户名","Pw":"fnmusic密码"}
```

保存返回的 `AccessToken`，后续可使用任一标准形式：

```http
X-Emby-Token: <AccessToken>
X-MediaBrowser-Token: <AccessToken>
Authorization: MediaBrowser Client="client", Device="phone", DeviceId="id", Version="1", Token="<AccessToken>"
```

媒体 URL 也接受 `api_key=<AccessToken>`，以兼容不能给音频请求添加 Header 的客户端。

1.0.4 起，AccessToken 到绑定用户的映射使用应用密钥加密保存。服务重启时会用本机
加密凭据重新向飞牛音乐登录，不会持久化飞牛上游 Token。`/Sessions/Logout`
和删除绑定账号会同步撤销已保存的 Jellyfin Token。

## 37. Jellyfin 已支持路径

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/System/Ping` | 无鉴权连通性探测 |
| GET | `/System/Info/Public` | 无鉴权服务发现 |
| GET | `/System/Info`, `/System/Endpoint` | 登录后服务信息 |
| GET | `/Users`, `/Users/Me`, `/Users/{userId}` | 当前用户 |
| GET | `/Users/{userId}/Views` | 单一 `music` 媒体库视图 |
| GET | `/Items/Counts` | 轻量读取歌曲、专辑、歌手和歌单数量 |
| GET | `/Users/{userId}/Items`, `/Items` | 按 `IncludeItemTypes` 浏览和搜索 |
| GET | `/Items/Root`, `/Items/Suggestions` | 音乐库根节点和随机可播放建议 |
| GET | `/Items/Latest` | 最新专辑和歌曲 |
| GET | `/Users/{userId}/Items/Latest`, `/Search/Hints` | 客户端首页和搜索提示 |
| GET | `/Users/{userId}/Items/{id}`, `/Items/{id}` | 媒体详情 |
| GET | `/Artists`, `/Artists/AlbumArtists`, `/Genres`, `/MusicGenres` | 歌手和流派 |
| GET | `/Artists/{name}`, `/Genres/{name}`, `/MusicGenres/{name}` | 名称精确查询 |
| GET | `/Items/{id}/Images/Primary` | 封面二进制 |
| GET/POST | `/Items/{id}/PlaybackInfo` | 只声明 DirectPlay/DirectStream |
| GET/HEAD | `/Audio/{id}/stream`, `/Audio/{id}/universal` | 原音频与 Range |
| GET | `/Items/{id}/Download`, `/Items/{id}/File` | 原文件读取和下载 |
| GET | `/Audio/{id}/Lyrics` | Jellyfin 时间轴歌词结构 |
| GET | `/UserItems/{id}/UserData` | 收藏和播放状态 |
| POST/DELETE | `/Users/{userId}/FavoriteItems/{trackId}` | 歌曲收藏 |
| GET/POST | `/Playlists` | 读取或创建歌单 |
| GET/POST/DELETE | `/Playlists/{playlistId}/Items` | 歌单歌曲读取、添加和移除 |
| POST/DELETE | `/Items/{playlistId}` | 歌单重命名和删除 |
| POST | `/Sessions/Playing`, `/Sessions/Playing/Progress`, `/Sessions/Playing/Stopped` | 播放事件；Stopped 写入播放历史 |
| POST | `/Sessions/Capabilities`, `/Sessions/Capabilities/Full`, `/Sessions/Logout` | 客户端能力登记与注销 |
| POST | `/Users/{userId}/PlayedItems/{trackId}` | 标记播放并写入历史 |
| GET | `/ScheduledTasks`, `/ScheduledTasks/{taskId}` | 音乐扫描状态和任务详情 |
| POST | `/Library/Refresh` | 管理员触发扫描 |

`IncludeItemTypes` 支持 `MusicAlbum`、`MusicArtist`、`MusicGenre`、`Audio` 和
`Playlist`。常用筛选包括 `ParentId`、`SearchTerm`、`ArtistIds`、`Artists`、
`IsFavorite`、`StartIndex` 和 `Limit`。

- `Filters=IsFavorite` 的歌曲读取 `/me/favorites`；专辑和歌手列表从收藏歌曲去重派生。
- `SortBy=DatePlayed` 读取 `/me/history`，并映射播放次数和最后播放时间。
- `SortBy=Random` 先读取曲库总数，再选取随机页段，因此候选范围不限于首页。
- `SearchTerm` 分别路由到飞牛音乐曲目、专辑和歌手搜索索引。

## 38. Jellyfin 不支持

Similar、Instant Mix、歌词上传/删除、歌单重排、协作歌单用户、视频、Live TV、频道、
插件、远程会话控制、SyncPlay、WebSocket、字幕、硬件转码、Jellyfin 后台管理，以及
专辑/歌手收藏的新增和取消均为 **不支持**；收藏专辑/歌手浏览列表从收藏歌曲派生。登录返回的 `Policy` 和媒体 `MediaSources` 会如实关闭
相关能力；进入兼容路由但没有可靠映射的路径返回 HTTP `501`，非音乐路径可能返回 `404`。

逐接口状态与真实黑盒证据见 `docs/JELLYFIN_10_10_7_MATRIX.md`；官方 103 个音乐相关
Operation 的逐行结论见 `docs/JELLYFIN_10_10_7_OPERATIONS.md`。候选测试结果不能替代
正式 FPK 安装后的公网 HTTPS 复测。

---

# 第五部分：统一协议能力声明

## 39. GET `/yinqiao/capabilities`

无需鉴权，返回 OpenSubsonic、Ampache、Synology Audio Station 和 Jellyfin Audio
四个兼容层的开关、基路径、`supported`、`unsupported`，以及：

```json
{"unsupportedStatus":"不支持"}
```

这个端点是适配器自身的权威声明；底层 Yinqiao API 的 85 项后台能力仍以
`http://NAS_IP:18090/yinqiao/v1/capabilities` 为准。
