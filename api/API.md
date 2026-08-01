# fnMusic Yinqiao API v1

> 面向独立 Web、iOS、Android、桌面或 AI 生成客户端的稳定接口。
> 局域网基址：`http://<NAS局域网IP>:18090/yinqiao/v1`
> 当前 API 版本：`v1`，随音桥 `1.0.5` 发布

当前 FPK 的 LAN API 不要求 `X-Music-API-Key`，也不经过 fnOS 域名或
统一网关。原音乐服务自身的用户、管理员和曲库权限保持有效；需要业务身份的接口
使用登录接口取得的 Bearer Token。

## 1. 客户端只需要知道的约定

客户端不需要生成原飞牛音乐的 `authx`，也不应直接调用内部
`/music/api/v1`。API 负责内部签名、Cookie 转换和版本兼容。

### 1.1 请求格式

- GET 参数使用 URL Query。
- POST 默认使用 `Content-Type: application/json`。
- 封面上传使用 `multipart/form-data`，表单字段名固定为 `file`。
- 音频、封面和 HLS 返回原始二进制，不使用 JSON 包装。
- 建议每次请求传 `X-Request-ID`；不传时 API 自动生成。

### 1.2 登录和 Bearer Token

浏览器同源客户端可以使用 API 返回的 `music-token` Cookie。独立客户端登录后保存：

```json
{
  "ok": true,
  "data": {
    "userToken": "...",
    "user": {"guid":"...","name":"...","role":"admin"}
  }
}
```

之后发送：

```http
Authorization: Bearer <userToken>
```

不要把 `userToken` 放入 URL。客户端退出登录后必须清除本地 token。

### 1.3 成功响应

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

列表通常返回：

```json
{
  "ok": true,
  "data": {
    "list": [],
    "total": 0,
    "sort": "createdAt,desc"
  },
  "meta": {"requestId":"...","adapter":"yinqiao-v1"}
}
```

### 1.4 错误响应

```json
{
  "ok": false,
  "error": {
    "code": "UPSTREAM_120001",
    "message": "登录状态已失效",
    "upstreamCode": 120001
  },
  "meta": {"requestId":"...","adapter":"yinqiao-v1"}
}
```

| HTTP | 客户端处理 |
| ---: | --- |
| 400 | 请求 JSON 或参数错误 |
| 401 | 原音乐服务 Token 或登录态无效 |
| 403 | 角色、曲库权限或跨域来源不允许 |
| 404 | 资源或 API 路由不存在 |
| 409 | 名称/路径冲突、任务已经运行 |
| 413 | 上传或请求体超过 API 限制 |
| 422 | 业务参数有效但不满足服务约束 |
| 502 | 内部音乐服务不可用或返回未知格式 |

`error.code` 是稳定的机器可读字段；显示给用户时使用 `error.message`。

## 2. 公共模型

### 2.1 Track

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
  accessStatus?: number;     // 0 正常；非 0 表示缺失或无权限等状态
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
  size?: number;
  duration?: number;         // 毫秒
  sampleRate?: number;
  bitrate?: number;
  bitDepth?: number;
  channel?: number;
  channels?: number;
}
```

### 2.2 Album、Artist、Genre、Playlist

```ts
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
```

### 2.3 User 和 LibraryAccess

```ts
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
```

### 2.4 SharedLibrary

```ts
type MetadataPreference =
  | "local_preferred"
  | "cloud_preferred"
  | "local_only";

interface SharedLibrary {
  guid: string;
  name: string;
  path: string;
  autoDownloadLyric: boolean;
  metadataPreference: MetadataPreference;
  accessStatus: number;
  contentLastChangedAt?: number;
  coverIds: string[];
}
```

## 3. 服务发现（API 自身）

| Method | Path | 鉴权 | `data` |
| --- | --- | --- | --- |
| GET | `/` | 可选 API Key | 名称、版本、API 版本、能力数 |
| GET | `/health` | 无 | `{status:"ok",upstream:"configured"}` |
| GET | `/ready` | 可选 API Key | 上游可用时 `{status:"ready",upstream:"ready"}`，否则 HTTP 503 |
| GET | `/capabilities` | 可选 API Key | 85 项路由的 ID、分类、方法、路径、权限和响应模式 |

客户端或 AI Agent 可以先调用 `/capabilities`，但正式代码应使用本文档中的固定接口。

## 4. 初始化与鉴权

### GET `/system/config`

- 权限：公开
- 参数：无
- `data`：`{serverName,serverGUID,nasOAuth?:{url,clientId}}`

### GET `/system/initialization`

- 权限：公开
- `data`：`{initialized:boolean}`

### POST `/auth/initialization/prepare`

```json
{"code":"fnOS OAuth code","deviceId":"32位十六进制设备ID"}
```

返回 `{sessionId,operator:{name,isAdmin}}`。

### POST `/auth/initialization/confirm`

```json
{
  "sessionId":"...",
  "user":{"name":"admin","password":"明文或64位SHA-256"},
  "server":{"name":"我的音乐","lang":"zh-CN"}
}
```

API 会在需要时将密码转换为内部要求的 SHA-256。返回 `{userToken,user}`。

### POST `/auth/login/oauth`

```json
{"code":"fnOS OAuth code","deviceId":"32位十六进制设备ID"}
```

返回 `{userToken,user}`。

### POST `/auth/login/password`

```json
{
  "username":"alice",
  "password":"客户端明文密码",
  "deviceId":"32位十六进制设备ID"
}
```

必须使用 HTTPS。API 接受明文或 64 位 SHA-256，并统一适配内部服务。返回 `{userToken,user}`。

### GET `/auth/me`

- 权限：登录用户
- 返回当前 `MusicUser`。

### POST `/auth/logout`

- 权限：登录用户
- body：空对象或无 body
- 客户端收到成功后清除 token 和 Cookie。

## 5. 用户与设置

### 用户接口

| Method | Path | 权限 | Query/JSON body | `data`/行为 |
| --- | --- | --- | --- | --- |
| GET | `/admin/users` | 管理员 | 无 | 用户数组或 `{list}` |
| GET | `/admin/users/exists` | 管理员 | `username` | `{isExist}` |
| POST | `/admin/users` | 管理员 | `{username,password,sharedLibraryAccess}` | 创建成员用户 |
| POST | `/admin/users/update` | 管理员 | `{guid,username,password?,sharedLibraryAccess}` | 编辑用户 |
| POST | `/account/password` | 登录用户 | `{password}` | 修改当前用户密码 |
| POST | `/admin/users/delete` | 管理员 | `{guid}` | 删除/停用用户 |
| POST | `/admin/users/unban` | 管理员 | `{guid}` | 解除封禁 |

以上密码字段均可传明文，API 会做内部格式转换。创建/编辑用户时：

```json
{
  "username":"bob",
  "password":"secret123",
  "sharedLibraryAccess":{"mode":"partial","guids":["library-guid"]}
}
```

### 设置接口

| Method | Path | 权限 | JSON body | `data`/行为 |
| --- | --- | --- | --- | --- |
| GET | `/admin/settings/server` | 管理员 | - | `{name,lang}` |
| POST | `/admin/settings/server` | 管理员 | `{name,lang}` | 更新服务器设置 |
| GET | `/admin/settings/default-user-access` | 管理员 | - | `{defaultSharedLibraryAccess}` |
| POST | `/admin/settings/default-user-access` | 管理员 | `{defaultSharedLibraryAccess:{mode,guids}}` | 更新默认权限 |

## 6. fnOS 目录与共享音乐库

### 授权目录

| Method | Path | Query | `data` |
| --- | --- | --- | --- |
| GET | `/admin/directories` | 无 | `{list:AuthorizedDir[]}` |
| GET | `/admin/directories/children` | `parent=/vol1/music` | `{list:Array<{path,name}>}` |

这两个接口只返回 fnOS 已授权给应用的目录。

### 共享音乐库

| Method | Path | Query/JSON body | `data`/行为 |
| --- | --- | --- | --- |
| GET | `/admin/libraries` | 无 | `SharedLibrary[]` 或 `{list}` |
| GET | `/admin/libraries/detail` | Query：`guid` | `SharedLibrary` |
| POST | `/admin/libraries` | `{path,autoDownloadLyric,metadataPreference}` | 创建并返回音乐库 |
| POST | `/admin/libraries/update` | `{guid,path,autoDownloadLyric,metadataPreference}` | 更新音乐库 |
| POST | `/admin/libraries/delete` | `{guid}` | 删除音乐库 |
| POST | `/admin/libraries/scan` | `{guid}` | 创建单库扫描任务 |
| POST | `/admin/libraries/scan-all` | 空 body | 扫描全部音乐库 |

创建示例：

```json
{
  "path":"/vol1/music",
  "autoDownloadLyric":true,
  "metadataPreference":"cloud_preferred"
}
```

## 7. 曲目、专辑、艺术家和风格

### 曲目列表

所有列表返回 `{list:Track[],total,sort?}`。

| Method | Path | Query |
| --- | --- | --- |
| GET | `/catalog/tracks` | `page,size,sort?` |
| GET | `/catalog/tracks/by-album` | `albumGUID,page,size,sort?` |
| GET | `/catalog/tracks/by-artist` | `artistGUID,page,size,sort?` |
| GET | `/catalog/tracks/by-genre` | `genreGUID,page,size,sort?` |
| GET | `/catalog/tracks/by-playlist` | `playlistGUID,page,size,sort?` |

`page` 从 1 开始。排序格式为 `field,asc|desc`，例如 `createdAt,desc`、`trackNo,asc`。

### 专辑

| Method | Path | Query | `data` |
| --- | --- | --- | --- |
| GET | `/catalog/albums` | `page,size,sort?` | 专辑分页 |
| GET | `/catalog/albums/by-artist` | `artistGUID,page,size,sort?` | 艺术家专辑分页 |
| GET | `/catalog/albums/detail` | `guid` | `Album` |

### 艺术家

| Method | Path | 权限 | Query/body | `data` |
| --- | --- | --- | --- | --- |
| GET | `/catalog/artists` | 用户 | `page,size,sort?` | 艺术家分页 |
| GET | `/catalog/artists/all` | 用户 | 无 | `{list:Artist[]}` |
| GET | `/catalog/artists/detail` | 用户 | `guid` | `Artist` |
| POST | `/admin/artists` | 管理员 | `{name}` | 新建 `Artist` |

### 风格

| Method | Path | 权限 | Query/body | `data` |
| --- | --- | --- | --- | --- |
| GET | `/catalog/genres` | 用户 | `page,size,sort?` | 风格分页 |
| GET | `/catalog/genres/detail` | 用户 | `guid` | `Genre` |
| POST | `/admin/genres` | 管理员 | `{name}` | 新建 `Genre` |

## 8. 曲目元数据与封面

### GET `/admin/tracks/metadata`

- 权限：管理员
- Query：`guid`
- 返回完整曲目元数据。

### POST `/admin/tracks/metadata`

```json
{
  "guid":"track-guid",
  "title":"歌曲标题",
  "artistGUIDs":["artist-guid"],
  "album":"专辑名",
  "genreGUIDs":["genre-guid"],
  "year":2026,
  "discNo":1,
  "trackNo":3,
  "coverId":"cover-id-or-null"
}
```

不修改的可空字段可以省略；明确清空时传 `null`。

### GET `/media/covers/{coverId}`

- 权限：媒体读取
- Query：可选 `size`，例如 `640`
- 返回：图片二进制。

### POST `/admin/tracks/cover`

- 权限：管理员
- Content-Type：`multipart/form-data`
- 表单字段：`file`
- JSON `data`：至少包含 `{coverId}`。

### POST `/me/playlists/cover`

与曲目封面上传格式相同，权限为登录用户。

## 9. 播放与漫游

### GET `/media/tracks/{guid}/stream`

- 返回原音频或 CUE 区段。
- 支持 `Range: bytes=start-end`。
- API 透传 `206`、`Content-Range`、`Accept-Ranges`、`ETag` 和音频类型。
- 此接口不返回 JSON。

### GET `/media/tracks/{guid}/hls/{filename}`

- `filename` 首次使用 `preset.m3u8`。
- 同一路径继续读取播放列表中的 HLS 分片。
- 此接口不返回 JSON。

### 转码会话

| Method | Path | JSON body | 行为 |
| --- | --- | --- | --- |
| POST | `/playback/transcode/start` | `{guid,output:{codec,bitrate,channel}}` | 启动/准备转码 |
| POST | `/playback/transcode/heartbeat` | `{guid,timestamp}` | 维持会话，建议每 10 秒 |
| POST | `/playback/transcode/stop` | `{guid}` | 结束并释放转码资源 |

推荐输出：

```json
{"guid":"track-guid","output":{"codec":"flac","bitrate":320,"channel":2}}
```

### 漫游播放

| Method | Path | Query | `data` |
| --- | --- | --- | --- |
| GET | `/playback/roam/start` | `deviceId` | `{current,next?}` |
| GET | `/playback/roam/next` | `deviceId,relativeRoamId` | `{current,next?,previous?}` |
| GET | `/playback/roam/previous` | `deviceId,relativeRoamId` | `{current,next?,previous?}` |

每个位置结构为 `{track,roamId}`；移动时使用上次返回的 `roamId`，不要使用曲目 GUID。

## 10. 歌词与用户事件

### GET `/catalog/lyrics`

- Query：`trackGUID`
- `data`：

```ts
interface LyricList {
  list: Array<{
    guid: string;
    content: string;       // LRC 或纯文本
    source: number;        // 1 embedded, 2 sidecar, 3 scraped, 4 user_linked
    updatedAt?: number;
  }>;
  preferred?: string;
}
```

### POST `/activity/events`

```json
{
  "events":[
    {
      "eventType":"track_play",
      "occurredAt":1785370000000,
      "payload":{"trackGUID":"track-guid"}
    }
  ]
}
```

支持：

- `track_play`：`{trackGUID}`
- `lyric_offset_change`：`{trackGUID,lyricGUID,offset}`，偏移为毫秒
- `lyric_preference_change`：曲目和首选歌词 GUID
- `sorting_change`：列表排序标识与排序值

## 11. 收藏、历史和歌单

### 收藏

| Method | Path | Query/body | `data`/行为 |
| --- | --- | --- | --- |
| GET | `/me/favorites` | `page,size,sort?` | 收藏曲目分页 |
| POST | `/me/favorites` | `{trackGUID}` | 收藏 |
| POST | `/me/favorites/remove` | `{trackGUID}` | 取消收藏 |
| GET | `/me/favorites/purge-count` | 无 | `{total}` |
| POST | `/me/favorites/purge` | 空 body | 清理失效收藏 |

### 播放历史

| Method | Path | Query/body | `data`/行为 |
| --- | --- | --- | --- |
| GET | `/me/history` | `page,size,sort?` | 播放历史分页 |
| POST | `/me/history/delete` | `{trackGUIDs:[...]}` | 删除指定历史记录 |

`track_play` 事件负责新增历史和累计播放次数。旧 `0.9.11` 的历史删除 body 缺少官方调用样本，正式客户端启用批量删除前应在目标 NAS 版本做一次集成验证。

### 歌单

| Method | Path | Query/body | `data`/行为 |
| --- | --- | --- | --- |
| GET | `/me/playlists` | 无 | `{list:Playlist[]}` |
| GET | `/me/playlists/batch-detail` | `guids=a,b,c` | `{list:[{guid,trackCount}]}` |
| POST | `/me/playlists` | `{name,coverId?}` | 创建并返回歌单 |
| POST | `/me/playlists/update` | `{guid,name,coverId?}` | 更新歌单 |
| POST | `/me/playlists/delete` | `{guid}` | 删除歌单 |
| GET | `/me/playlists/detail` | `guid` | `Playlist` |
| POST | `/me/playlists/tracks/add` | `{guid,trackGUIDs:[...]}` | 批量添加曲目 |
| POST | `/me/playlists/tracks/remove` | `{guid,trackGUIDs:[...]}` | 批量移除曲目 |
| GET | `/me/playlists/purge-count` | `guid` | `{total}` |
| POST | `/me/playlists/purge` | `{guid}` | 清理失效曲目 |

歌单封面上传见第 8 节。

## 12. 搜索

| Method | Path | Query | `data` |
| --- | --- | --- | --- |
| GET | `/search/tracks` | `q,page,size` | 曲目分页 |
| GET | `/search/albums` | `q,page,size` | 专辑分页 |
| GET | `/search/artists` | `q,page,size` | 艺术家分页 |
| GET | `/search/playlists` | `q,page,size` | 歌单分页 |
| GET | `/search/suggest` | `q` | 四类 `{total,items}` 联想结果 |
| POST | `/admin/search/rebuild` | 空 body | `{albumCount,artistCount,trackCount}` |

索引重建是管理员操作，可能创建后台任务或返回 `UPSTREAM_140001` 表示已经在重建。

## 13. 后台任务

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

| Method | Path | Query/body | `data`/行为 |
| --- | --- | --- | --- |
| GET | `/admin/tasks` | 无 | `Task[]` 或 `{list}` |
| POST | `/admin/tasks/cancel` | `{taskId}` | 请求取消 |
| POST | `/admin/tasks/delete` | `{taskId}` | 删除任务记录 |
| POST | `/admin/tasks/retry` | `{taskId}` | 重试失败任务 |

## 14. 客户端调用示例

### 14.1 密码登录

```bash
curl -sS 'http://192.168.1.10:18090/yinqiao/v1/auth/login/password' \
  -H 'Content-Type: application/json' \
  -d '{
    "username":"alice",
    "password":"secret123",
    "deviceId":"0123456789abcdef0123456789abcdef"
  }'
```

### 14.2 使用 Bearer 获取曲目

```bash
curl -sS 'http://192.168.1.10:18090/yinqiao/v1/catalog/tracks?page=1&size=50&sort=createdAt%2Cdesc' \
  -H 'Authorization: Bearer <userToken>'
```

### 14.3 播放 Range 请求

```bash
curl 'http://192.168.1.10:18090/yinqiao/v1/media/tracks/<guid>/stream' \
  -H 'Authorization: Bearer <userToken>' \
  -H 'Range: bytes=0-1048575' \
  --output part.audio
```

### 14.4 TypeScript 最小客户端

```ts
const baseURL = "/yinqiao/v1";
let token = "";

async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...(init.body ? {"Content-Type": "application/json"} : {}),
      ...(token ? {Authorization: `Bearer ${token}`} : {}),
      ...init.headers,
    },
  });
  const result = await response.json();
  if (!response.ok || !result.ok) {
    const error = new Error(result.error?.message || `HTTP ${response.status}`);
    Object.assign(error, {code: result.error?.code, status: response.status});
    throw error;
  }
  return result.data as T;
}

async function login(username: string, password: string, deviceId: string) {
  const result = await api<{userToken: string; user: MusicUser}>(
    "/auth/login/password",
    {method: "POST", body: JSON.stringify({username, password, deviceId})},
  );
  token = result.userToken;
  return result.user;
}
```

## 15. AI 客户端开发规则

提供本文档给代码生成模型时，应额外要求：

1. 只调用 `/yinqiao/v1`，禁止调用内部 `/music/api/v1`。
2. 不实现 `authx`，不在客户端保存内部签名常量。
3. JSON 统一检查 HTTP 状态和 `ok`；使用 `error.code` 分支处理。
4. `401` 清理 token 并重新登录；`403` 不应自动重试。
5. 分页从 1 开始；对 `total` 做终止判断。
6. 音频使用 Range、原生媒体组件和 Cookie jar；不要把完整音频读进普通 JSON 请求。
7. HLS 客户端需要保持 Cookie 或同一认证会话，以便后续分片请求继续获权。
8. 管理接口只对 `role=admin` 展示，但仍由服务端作最终权限校验。

## 16. 通用官方 API 透传

当官方服务存在尚未列入稳定路由表的接口时，可以调用：

```text
GET|POST|PUT|PATCH|DELETE
/yinqiao/v1/upstream/<API v1 相对路径>
```

例如官方 `/music/api/v1/sys/info`：

```bash
curl 'http://192.168.1.10:18090/yinqiao/v1/upstream/sys/info' \
  -H 'Authorization: Bearer <userToken>'
```

API 会生成 `authx`、转换 Bearer Cookie 并保留 Query、请求体、HTTP 状态、
二进制响应和 Range 头。该入口返回官方原始格式，不使用稳定接口的
`{ok,data,error,meta}` 包装。路径必须规范化，禁止目录穿越。

客户端应优先使用前述稳定接口；透传用于兼容新版本和尚未完成字段归一化的能力。
