# 飞牛音乐服务能力与 HTTP API 说明

> 分析对象：`trim.music 0.9.11`（x86-64 fnOS 官方包）
> 文档日期：2026-07-30
> API 外部基址：`/music/api/v1`
> 后端内部路由前缀：`/api/v1`

## 1. 文档范围与可信度

本文档说明当前包中 Go 服务端实际向 Web 客户端提供的能力，依据如下材料交叉整理：

- 官方前端的完整 API 路由注册表：`analysis/pretty/vendor-shared.js`
- 官方前端的请求封装、页面调用参数和响应归一化：`analysis/pretty/app-shared.js`、`analysis/pretty/main.js`
- 当前 H5 已验证使用的客户端：`src-h5/src/api/client.ts`
- Go 服务二进制字符串、SQLite 建表脚本及 fnOS 包生命周期脚本

接口说明使用以下标记：

- **已确认**：方法、路径和主要字段均可从官方前端调用代码直接确认。
- **路由确认**：方法和路径已确认，但官方 UI 没有留下足够调用样本，个别字段仍需真机抓包确认。
- **内部接口**：只在 Go 二进制中发现，未进入官方前端 API 契约，不建议 H5 直接依赖。

本包没有 OpenAPI/Swagger 文件，Go 二进制也已剥离符号。因此本文不是厂商发布的正式开放 API，而是针对 `0.9.11` 的兼容开发文档。

## 2. 能力总览

官方前端稳定注册了 **82 个唯一路径、85 个 HTTP 操作**。操作数比路径数多 3，是因为以下路径同时支持 GET 和 POST：

- `/settings/server`
- `/settings/user`
- `/track/metadata`

| 服务域 | 稳定操作数 | 主要能力 |
| --- | ---: | --- |
| 初始化与鉴权 | 8 | 初始化状态、NAS OAuth、本地密码登录、当前用户、登出 |
| 用户与权限 | 7 | 用户列表、查重、创建、编辑、改密、删除、解除封禁 |
| 系统与设置 | 5 | 系统配置、服务器名称/语言、默认曲库访问权限 |
| fnOS 授权目录 | 2 | 获取应用获权目录、浏览子目录 |
| 共享音乐库 | 7 | 新建、编辑、删除、扫描单库/全部、元数据策略 |
| 曲目 | 15 | 列表、关联列表、漫游、元数据、直出、HLS、转码会话 |
| 专辑 | 3 | 专辑列表、艺术家专辑、专辑详情 |
| 艺术家 | 4 | 创建、分页列表、全量列表、详情 |
| 风格 | 3 | 创建、列表、详情 |
| 歌词与事件 | 2 | 多来源歌词、播放/歌词/排序偏好事件 |
| 收藏 | 5 | 收藏列表、添加/取消、失效项统计与清理 |
| 播放历史 | 2 | 历史列表、删除历史 |
| 歌单 | 11 | CRUD、批量详情、曲目增删、失效项清理、封面上传 |
| 搜索 | 6 | 曲目/专辑/艺术家/歌单搜索、联想、索引重建 |
| 后台任务 | 4 | 任务列表、取消、删除、重试 |
| 静态封面 | 1 | 按封面 ID 和尺寸读取图片 |
| **合计** | **85** | |

除 HTTP API 外，服务端还承担以下后台能力：

1. SQLite 数据持久化和版本迁移。
2. 音乐文件扫描、增量文件监控、逻辑删除与恢复。
3. 音频标签、技术参数、CUE、内嵌/伴随封面和歌词提取。
4. 云端元数据刮削与歌词下载任务。
5. Bleve 全文索引、拼音字段和索引重建。
6. 原文件 Range 流式输出、服务端转码、HLS 会话维护。
7. fnOS OAuth、授权目录、文件存储、媒体和系统信息服务对接。

## 3. HTTP 协议约定

### 3.1 同源路径

浏览器访问使用：

```text
https://<NAS 域名>/music/api/v1/<endpoint>
```

应用没有用户可见的独立端口，`/music` 由 fnOS Web 网关转发到包内 Go 进程。因此 H5 应保持同源部署，不应自行拼接 NAS IP、端口或另建 API 域名。

### 3.2 标准响应包络

绝大多数 JSON 接口返回：

```json
{
  "code": 0,
  "msg": "",
  "data": {}
}
```

- 成功码：`0` 或 `200`
- 业务失败：HTTP 可能仍为 200，但 `code` 非成功码
- 登录过期：业务码 `120001` 或 HTTP `401`
- 文件流、封面和 HLS 响应不使用 JSON 包络

### 3.3 请求头与 Cookie

JSON 请求的通用头：

```http
Accept: application/json
Accept-Language: zh-CN
authx: nonce=<6位随机数>&timestamp=<毫秒时间戳>&sign=<md5>
Content-Type: application/json   # 仅有 JSON body 时
```

请求必须设置 `credentials: include`。登录后前端写入：

```http
music-token=<userToken>; Path=/; SameSite=Strict
```

设备 ID 保存在 `localStorage['music-device__id']`，格式为 32 位十六进制字符串。同一用户、同一设备 ID 对应一个持久登录令牌记录。

### 3.4 `authx` 签名算法

已还原的常量：

```text
signingPrefix = NDzZTVxnRKP8Z0jXg1VAMonaG8akvh
clientKey     = 6D5602D4-A342-4799-A0F0-BB795E7167D0
```

算法：

1. 生成 100000-999999 的 `nonce` 和毫秒 `timestamp`。
2. GET：参数按键名排序，使用 URL 编码查询串，空格编码为 `%20`；对 `decodeURIComponent(query)` 计算 MD5。
3. 非 GET：对 JSON 序列化后的 body 计算 MD5；无 body 时对空字符串计算 MD5。
4. 拼接：

```text
signingPrefix + "_" + pathname + "_" + nonce + "_" + timestamp
  + "_" + payloadMd5 + "_" + clientKey
```

5. 对拼接结果再次计算 MD5，写入 `authx`。

签名路径必须包含 `/music/api/v1`。查询参数用于 GET 签名，不能在签名后再调整顺序或增加参数。该机制是客户端兼容校验，不是可保密的密钥体系。

媒体 URL 由 `<audio>`、图片和 HLS 加载器直接请求，无法附加自定义 `authx`；这些接口主要依靠同源 `music-token` Cookie 和服务端的媒体路由策略。

### 3.5 参数与时间单位

- 分页：`page` 从 1 开始，`size` 为页大小。
- 排序：`sort=<field>,<asc|desc>`，例如 `createdAt,desc`、`trackNo,asc`。
- 多 ID 查询有两种形式：逗号字符串 `guids=a,b`，或 JSON 数组 `trackGUIDs: [...]`。
- `guid`/`*GUID` 是不可遍历的资源 ID，不是 SQLite 自增 ID。
- API 中 `createdAt`、`updatedAt` 通常为 Unix 秒。
- 曲目 `duration`、歌词偏移和音频规格中的时长通常为毫秒；前端播放前除以 1000。

## 4. 权限模型

| 级别 | 含义 |
| --- | --- |
| `公开初始化` | 无音乐登录态可调用，但仍需合法 `authx`；用于配置、初始化和登录 |
| `登录用户` | 需要 `music-token`；只能访问获权共享库中的资源 |
| `管理员` | 后端会校验 `admin` 角色；用于曲库、用户、服务器设置、索引和管理操作 |
| `媒体读取` | 需要同源登录 Cookie；用于 `<audio>`、封面和 HLS 子请求 |

用户角色为 `admin`、`member`。共享库访问模式为：

```ts
type LibraryAccess =
  | { mode: "all"; guids: [] }
  | { mode: "partial"; guids: string[] }
  | { mode: "none"; guids: [] };
```

读取用户时，服务端可能将 `guids` 展开为 `sharedLibraries` 对象数组。管理端保存时必须重新提交 `{mode, guids}`。

## 5. 公共数据模型

### 5.1 分页响应

```ts
interface Page<T> {
  list: T[];
  total: number;
  sort?: string;
}
```

### 5.2 曲目

```ts
interface Track {
  guid: string;
  title: string;
  coverId?: string;
  year?: number;
  duration: number;             // 毫秒
  trackNo?: number;
  discNo?: number;
  isFavorite?: boolean;
  isCue?: boolean;
  hasLyric?: boolean;
  accessStatus?: number;        // 非 0 表示文件缺失或无权限等不可用状态
  createdAt: number;            // Unix 秒
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
  size?: number;                // byte
  duration?: number;            // 毫秒
  sampleRate?: number;
  bitrate?: number;
  bitDepth?: number;
  channel?: number;
  channels?: number;
}
```

### 5.3 专辑、艺术家、风格与歌单

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
  originalReleaseDate?: string;
  originalReleaseYear?: number;
  barcode?: string;
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

### 5.4 共享音乐库

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

### 5.5 用户

```ts
interface MusicUser {
  guid: string;
  name: string;
  role: "admin" | "member";
  isBanned: boolean;
  bannedUntil?: number | null;
  lastAccessedAt?: number | null;
  createdAt?: number;
  updatedAt?: number;
  trimNasUserId?: string | null;
  trimNasUsername?: string | null;
  sharedLibraryAccess?: {
    mode: "all" | "partial" | "none";
    sharedLibraries: SharedLibrary[];
  };
}
```

## 6. 稳定 HTTP API

以下路径均相对于 `/music/api/v1`。

### 6.1 初始化与鉴权（8）

#### `GET /sys/config` — 已确认

- 权限：公开初始化
- 参数：无
- 返回：`{serverName, serverGUID, nasOAuth?: {url, clientId}}`
- 用途：读取服务显示名称、服务端唯一 ID 和 fnOS OAuth 登录入口。

#### `GET /initialization/state` — 已确认

- 权限：公开初始化
- 返回：`{initialized: boolean}`
- 用途：决定进入初始化流程还是登录流程。

#### `POST /initialization/prepare` — 已确认

```json
{"code":"<NAS OAuth code>","deviceId":"<32 hex>"}
```

- 权限：公开初始化；OAuth 操作者必须是 NAS 管理员
- 返回：`{sessionId, operator: {name, isAdmin}}`
- 用途：验证 fnOS 授权码并创建短期初始化会话。

#### `POST /initialization/confirm` — 已确认

```json
{
  "sessionId": "...",
  "user": {"name":"admin","password":"<sha256 hex>"},
  "server": {"name":"我的音乐","lang":"zh-CN"}
}
```

- 权限：有效初始化会话
- 返回：`{userToken, user}`
- 副作用：创建首个管理员、保存服务器配置、标记应用已初始化。

#### `POST /user/auth-login` — 已确认

```json
{"code":"<NAS OAuth code>","deviceId":"<32 hex>"}
```

- 权限：公开初始化
- 返回：`{userToken, user}`
- 用途：使用 fnOS OAuth 登录或绑定的 NAS 身份登录。

#### `POST /user/password-login` — 已确认

```json
{"username":"alice","password":"<sha256 hex>","deviceId":"<32 hex>"}
```

- 权限：公开初始化
- 返回：`{userToken, user}`
- 注意：密码在浏览器端先做 SHA-256，不能发送明文。

#### `GET /user/me` — 已确认

- 权限：登录用户
- 返回：当前 `MusicUser`
- 用途：启动应用时恢复登录态、角色和共享库权限。

#### `POST /user/logout` — 已确认

- 权限：登录用户
- body：无
- 返回：空数据
- 客户端后续应清除 `music-token` Cookie。

### 6.2 用户与权限管理（7）

本节接口均为管理员能力，当前用户改自己的密码除外。

| 方法与路径 | 请求 | 返回/行为 |
| --- | --- | --- |
| `GET /user/list` | 无 | `MusicUser[]` 或 `{list: MusicUser[]}`；管理员优先排序 |
| `GET /user/exists` | Query：`username` | `{isExist: boolean}` |
| `POST /user/create` | `{username,password,sharedLibraryAccess}` | 创建 `member`；密码为 SHA-256 |
| `POST /user/edit` | `{guid,username,password?,sharedLibraryAccess}` | 修改用户；空密码字段应省略 |
| `POST /user/passwd-change` | `{password}` | 修改当前用户密码；密码为 SHA-256 |
| `POST /user/delete` | `{guid}` | 逻辑删除/停用用户 |
| `POST /user/unbanned` | `{guid}` | 解除登录封禁状态 |

输入限制可从官方 UI 约束确认：用户名最长 32 字符；密码 6-127 字符。最终合法性仍以后端为准。

### 6.3 系统与用户默认设置（4）

| 方法与路径 | 权限 | 请求 | 返回/行为 |
| --- | --- | --- | --- |
| `GET /settings/server` | 管理员 | 无 | `{name,lang}` |
| `POST /settings/server` | 管理员 | `{name,lang}` | 更新服务器显示名和语言 |
| `GET /settings/user` | 管理员 | 无 | `{defaultSharedLibraryAccess}` |
| `POST /settings/user` | 管理员 | `{defaultSharedLibraryAccess:{mode,guids}}` | 设置新用户的默认曲库权限 |

服务器名称长度为 1-32 字符。

### 6.4 fnOS 授权目录（2）

#### `GET /app-center/authed-dir/list` — 已确认

- 权限：管理员
- 返回：`{list: AuthorizedDir[]}`

```ts
interface AuthorizedDir {
  path: string;
  storageType: number;
  cloudStorageType?: number;
  permission: string;
  uname: string;
  address: string;
  comment: string;
  username: string;
  proto?: string;
  port?: number;
}
```

该接口只返回 fnOS 已授权给应用的根目录，不是任意文件系统浏览器。

#### `GET /app-center/authed-dir/sub/list` — 已确认

- 权限：管理员
- Query：`parent=<absolute path>`
- 返回：`{list: Array<{path,name}>}`
- 用途：逐级浏览授权目录，选择共享音乐库路径。

### 6.5 共享音乐库（7）

| 方法与路径 | 请求 | 返回/行为 |
| --- | --- | --- |
| `GET /shared-library/list` | 无 | `SharedLibrary[]` 或带 `list/items/libraries` 的包装对象 |
| `GET /shared-library/detail` | Query：`guid` | `SharedLibrary` |
| `POST /shared-library/create` | `{path,autoDownloadLyric,metadataPreference}` | 创建音乐库并返回详情 |
| `POST /shared-library/edit` | `{guid,path,autoDownloadLyric,metadataPreference}` | 更新音乐库并返回详情 |
| `POST /shared-library/delete` | `{guid}` | 删除音乐库配置和关联索引 |
| `POST /shared-library/scan` | `{guid}` | 为单个音乐库创建扫描任务 |
| `POST /shared-library/scan-all` | 无 | 扫描全部音乐库 |

权限：管理员。服务端约束包括：

- 最多 200 个目录。
- 路径不可重复。
- 不能与已添加目录形成不允许的父子目录冲突。
- 目录必须在 fnOS 授权范围内且具备所需写权限。
- `autoDownloadLyric` 控制扫描/刮削阶段是否自动拉取歌词。
- `metadataPreference` 控制本地标签和云端元数据的优先级。

扫描不是同步完成；进度通过后台任务接口读取。

### 6.6 曲目查询与关联列表（5）

所有列表均返回 `Page<Track>`，并按当前用户的共享库权限过滤。

| 方法与路径 | Query |
| --- | --- |
| `GET /track/list` | `page,size,sort?` |
| `GET /track/album-detail/list` | `albumGUID,page,size,sort?` |
| `GET /track/artist-detail/list` | `artistGUID,page,size,sort?` |
| `GET /track/genre-detail/list` | `genreGUID,page,size,sort?` |
| `GET /track/playlist-detail/list` | `playlistGUID,page,size,sort?` |

常见排序字段包括 `createdAt`、`updatedAt`、`title`、`trackNo`、`trackAddedAt`。专辑曲目默认使用 `trackNo,asc`；歌单中前端会把 `createdAt` 映射为 `trackAddedAt`。

### 6.7 曲目漫游（3）

“漫游”是服务端维护的随机/推荐式连续播放队列。

| 方法与路径 | Query | 返回 |
| --- | --- | --- |
| `GET /track/roam-start` | `deviceId` | `{current:{track,roamId},next?:{track,roamId}}` |
| `GET /track/roam-next` | `deviceId,relativeRoamId` | `{current,next?,previous?}` |
| `GET /track/roam-previous` | `deviceId,relativeRoamId` | `{current,next?,previous?}` |

`roamId` 是队列相对位置标识，不是 `trackGUID`。客户端应保存当前返回项的 `roamId` 作为下一次移动的 `relativeRoamId`。

### 6.8 曲目元数据（2）

#### `GET /track/metadata` — 已确认

- Query：`guid`
- 返回：完整曲目元数据，包含曲目、艺术家、风格、专辑、封面和音频规格。

#### `POST /track/metadata` — 已确认

```json
{
  "guid": "track-guid",
  "title": "标题或 null",
  "artistGUIDs": ["artist-guid"],
  "album": "专辑名或 null",
  "genreGUIDs": ["genre-guid"],
  "year": 2026,
  "discNo": 1,
  "trackNo": 3,
  "coverId": "cover-id-or-null",
  "coverGUID": "兼容字段"
}
```

- 权限：管理员
- 返回：空数据或更新后的元数据（官方前端会重新 GET）
- 注意：艺术家和风格提交 GUID；不存在的值应先调用创建接口。

### 6.9 封面（3）

#### `GET /static/cover` — 已确认

- 权限：媒体读取
- Query：`coverId`，可选 `size`（当前 H5 使用 `640`）
- 返回：图片二进制和相应 `Content-Type`

#### `POST /static/cover/track` — 已确认

#### `POST /static/cover/playlist` — 已确认

- 权限：登录用户；曲目封面编辑通常要求管理员，歌单封面要求歌单所有者
- Content-Type：`multipart/form-data`
- 表单字段：`file`
- 返回：至少包含 `{coverId}`，部分版本也可能返回 `{guid}`
- 注意：上传时不要手工设置 multipart boundary。

### 6.10 播放、HLS 与转码（5）

#### `GET /track/stream` — 已确认

- 权限：媒体读取
- Query：`guid`
- 返回：原音频文件或 CUE 对应区段
- 能力：支持 HTTP Range，用于浏览器原生播放、拖动和分段缓存。

#### `GET /track/hls/:guid/preset.m3u8` — 已确认

- 权限：媒体读取
- 返回：HLS 播放列表
- 后端实际通配路由为 `/track/hls/:guid/:filename`，除 `preset.m3u8` 外还承载播放列表引用的分片文件。

#### `POST /track/transcode` — 已确认

```json
{
  "guid": "track-guid",
  "output": {
    "codec": "flac",
    "bitrate": 320,
    "channel": 2
  }
}
```

- 权限：登录用户
- 返回：`{status, errno?, errmsg?}`
- 状态：前端接受 `success`、`ready`；`failed` 表示失败，其他状态表示尚未就绪。
- 质量档位：官方前端把 `128`、`256`、默认档分别映射为 `bitrate: 128/256/320`，编码器均提交 `codec: "flac"`；`channel` 按输出设备能力选择。
- 用途：在浏览器不能原生解码时预备服务端转码/HLS 会话。

#### `POST /track/transcode/heartbeat` — 已确认

- body：`{guid,timestamp}`
- 调用周期：官方前端约 10 秒
- 用途：维持当前转码会话，避免服务端回收。

#### `POST /track/transcode/quit` — 已确认

- body：`{guid}`
- 用途：停止并清理当前曲目的转码资源。

### 6.11 专辑（3）

| 方法与路径 | Query | 返回 |
| --- | --- | --- |
| `GET /album/list` | `page,size,sort?` | `Page<Album>` |
| `GET /album/artist-detail/list` | `artistGUID,page,size,sort?` | `Page<Album>` |
| `GET /album/detail` | `guid` | `Album` |

常见专辑排序字段：`name`、`releaseDate`、`trackCount`、`newTrackAddedAt`。专辑详情不内嵌保证完整的曲目列表，应结合 `/track/album-detail/list`。

### 6.12 艺术家（4）

| 方法与路径 | 参数 | 返回 |
| --- | --- | --- |
| `POST /artist/create` | body：`{name}` | `Artist` |
| `GET /artist/list` | `page,size,sort?` | `Page<Artist>` |
| `GET /artist/list-all` | 无 | `{list: Artist[]}` |
| `GET /artist/detail` | `guid` | `Artist` |

艺术家名称长度为 1-30 字符。艺术家的专辑和曲目分别使用 `/album/artist-detail/list` 与 `/track/artist-detail/list`。

### 6.13 风格（3）

| 方法与路径 | 参数 | 返回 |
| --- | --- | --- |
| `POST /genre/create` | body：`{name}` | `Genre` |
| `GET /genre/list` | `page,size,sort?` | `Page<Genre>` |
| `GET /genre/detail` | `guid` | `Genre` |

风格名称长度为 1-30 字符。风格曲目通过 `/track/genre-detail/list` 获取。

### 6.14 歌词（1）

#### `GET /lyric/list` — 已确认

- Query：`trackGUID`
- 返回：

```ts
interface LyricListResponse {
  list: Array<{
    guid: string;
    content: string;       // LRC 或纯文本
    source: number;        // 1 embedded, 2 sidecar, 3 scraped, 4 user_linked
    updatedAt?: number;
  }>;
  preferred?: string;      // 当前用户偏好的 lyric guid
}
```

歌词可能有多个来源。客户端应优先选 `preferred`，否则可选择最近更新的非空内容。用户级歌词偏好和偏移通过事件接口保存。

### 6.15 用户事件与偏好（1）

#### `POST /event/report` — 已确认

```json
{
  "events": [
    {
      "eventType": "track_play",
      "occurredAt": 1785370000000,
      "payload": {"trackGUID":"..."}
    }
  ]
}
```

支持的事件类型：

| `eventType` | 已确认 payload | 后台作用 |
| --- | --- | --- |
| `track_play` | `{trackGUID}` | 新增/更新播放历史与播放次数 |
| `lyric_offset_change` | `{trackGUID,lyricGUID,offset}` | 保存用户歌词偏移，`offset` 为毫秒 |
| `lyric_preference_change` | 路由确认；包含曲目和歌词 GUID | 保存用户对某曲目的首选歌词 |
| `sorting_change` | 路由确认；包含排序标识和排序值 | 保存用户在各列表的排序偏好 |

接口支持在一次请求中批量上报多个事件。官方前端在页面离开时也可能通过 `navigator.sendBeacon` 上报播放事件。

### 6.16 收藏（5）

| 方法与路径 | 参数 | 返回/行为 |
| --- | --- | --- |
| `GET /favorite-track/list` | `page,size,sort?` | `Page<Track>` |
| `POST /favorite-track/create` | `{trackGUID}` | 收藏曲目 |
| `POST /favorite-track/delete` | `{trackGUID}` | 取消收藏 |
| `GET /favorite-track/purge-track-count` | 无 | `{total}` |
| `POST /favorite-track/purge-track` | 无 | 清理文件缺失/无权限等失效收藏项 |

收藏列表的服务端排序字段 `createdAt` 在 UI 中显示为 `favoriteAt`。

### 6.17 播放历史（2）

| 方法与路径 | 参数 | 返回/行为 |
| --- | --- | --- |
| `GET /play-history/list` | 路由确认；预计支持 `page,size,sort?` | 播放历史曲目和分页信息 |
| `POST /play-history/delete` | 路由确认；删除目标字段需真机抓包确认 | 删除指定或一批历史记录 |

历史数据由 `track_play` 事件维护：同一用户、同一曲目累计 `playCount`，并更新最后播放时间。上述两个接口虽然已在官方 API 注册，但当前保留的 UI 调用代码不足以确认删除 body 的精确字段，接入前应在 fnOS 真机验证。

### 6.18 歌单（11）

| 方法与路径 | 参数 | 返回/行为 |
| --- | --- | --- |
| `GET /playlist/list` | 无 | `{list: Playlist[]}` |
| `GET /playlist/batch-detail` | `guids=a,b,c`，单批最多按官方 UI 100 个 | `{list:Array<{guid,trackCount}>}` |
| `POST /playlist/create` | `{name,coverId?}` | `Playlist` |
| `POST /playlist/edit` | `{guid,name,coverId?}` | 更新歌单 |
| `POST /playlist/delete` | `{guid}` | 删除歌单 |
| `GET /playlist/detail` | `guid` | `Playlist` |
| `POST /playlist/add-track` | `{guid,trackGUIDs:string[]}` | 批量加入曲目 |
| `POST /playlist/remove-track` | `{guid,trackGUIDs:string[]}` | 批量移除曲目 |
| `GET /playlist/purge-track-count` | `guid` | `{total}` |
| `POST /playlist/purge-track` | `{guid}` | 清理歌单中的失效曲目 |
| `POST /static/cover/playlist` | multipart 字段 `file` | 上传歌单封面，返回 `{coverId}` |

约束：歌单名称 1-32 字符；每用户最多 99999 个歌单；名称重复返回专用错误码。歌单曲目列表通过 `/track/playlist-detail/list` 分页读取。

### 6.19 搜索（6）

| 方法与路径 | 参数 | 返回 |
| --- | --- | --- |
| `GET /search/track` | `q,page,size` | `Page<Track>` |
| `GET /search/album` | `q,page,size` | `Page<Album>` |
| `GET /search/artist` | `q,page,size` | `Page<Artist>` |
| `GET /search/playlist` | `q,page,size` | `Page<Playlist>` |
| `GET /search/suggest` | `q` | `{track,album,artist,playlist}`，各含 `{total,items}` |
| `POST /search/index/rebuild` | 无 | `{albumCount,artistCount,trackCount}` |

搜索基于本地 Bleve 索引，索引包含名称原文、拉丁全拼和首字母等字段。索引重建为管理员操作；已有重建进行时返回 `140001`。

### 6.20 后台任务（4）

```ts
interface Task {
  id: string;
  type: string;              // 如 fileScan、cloudScrape、lyricDownload、download
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

| 方法与路径 | 参数 | 返回/行为 |
| --- | --- | --- |
| `GET /task/list` | 无 | `Task[]` 或 `{list:Task[]}` |
| `POST /task/cancel` | `{taskId}` | 请求取消任务 |
| `POST /task/delete` | `{taskId}` | 删除已完成任务记录 |
| `POST /task/retry` | `{taskId}` | 重试可重试的失败任务 |

`ext.taskIds` 表示 UI 聚合任务对应的多个底层任务；此时控制操作要逐个提交底层 `taskId`。

## 7. 错误码

| Code | 名称 | 含义 |
| ---: | --- | --- |
| 100001 | `Unknown` | 未分类错误 |
| 100002 | `InvalidArgs` | 参数不完整或格式错误 |
| 100003 | `AdminRequired` | 仅管理员可操作 |
| 100004 | `Forbidden` | 当前用户无权限 |
| 100005 | `NotFound` | 资源不存在或已删除 |
| 110001 | `AppAlreadyInitialized` | 应用已初始化 |
| 110002 | `AppNotInitialized` | 应用尚未初始化 |
| 110003 | `InitRequiresNASAdmin` | 初始化操作者不是 NAS 管理员 |
| 110004 | `InitSessionNotFound` | 初始化会话不存在或过期 |
| 120001 | `Unauthorized` | 登录态无效/过期 |
| 120002 | `UserDisabled` | 用户已停用 |
| 120003 | `OAuthUserAlreadyExists` | OAuth 用户已存在 |
| 120004 | `UsernameExists` | 用户名已存在 |
| 120005 | `InvalidUsername` | 用户名格式错误 |
| 120006 | `PasswordRequired` | 必须提供密码 |
| 130001 | `CueMissingOffset` | CUE 曲目缺少开始或结束偏移 |
| 140001 | `SearchIndexRebuildInProgress` | 搜索索引正在重建 |
| 150001 | `SharedLibraryNameExists` | 音乐库名称已存在 |
| 150002 | `SharedLibraryPathExists` | 目录已添加为音乐库 |
| 150003 | `SharedLibraryNoWritePermission` | 目录无写权限 |
| 150004 | `SharedLibraryHitMaxCount` | 音乐库达到 200 个上限 |
| 150005 | `SharedLibrarySubPathExists` | 与已有音乐库发生子目录冲突 |
| 160001 | `PlaylistNameExists` | 歌单名称已存在 |
| 160002 | `PlaylistHitMaxCount` | 歌单达到 99999 个上限 |

## 8. 二进制可见但非官方前端契约的接口

以下路径出现在 Go 二进制中，但不在官方前端的 82 路径注册表内。由于无法从已剥离符号的二进制稳定恢复 HTTP 方法、权限和完整 schema，应视为内部、开发态或服务间接口。

| 路径/路径组 | 推断能力 | 使用建议 |
| --- | --- | --- |
| `/api/v1/sys/info` | 后端/系统版本和依赖服务信息 | 仅诊断，不依赖 |
| `/api/v1/fn/status` | fnOS 依赖状态/健康检查 | 仅诊断 |
| `/api/v1/user/temp-token` | 签发临时令牌；二进制含 `X-Trim-Music-Temp-Token` | 可能供下载/媒体子请求使用 |
| `/api/v1/track/delete` | 管理员逻辑删除曲目 | 未确认方法与 body |
| `/api/v1/track/recover` | 恢复逻辑删除曲目 | 未确认方法与 body |
| `/api/v1/track/audio-info` | 读取底层音频技术信息 | 未确认返回 schema |
| `/api/v1/folder-view/list` | 文件夹视图 | 未进入正式 UI 契约 |
| `/api/v1/download/track` | 创建曲目下载任务 | 内部下载流程 |
| `/api/v1/download/track/detail` | 下载任务/曲目详情 | 内部下载流程 |
| `/api/v1/download/track/transcode/prepare` | 准备下载转码 | 内部下载流程 |
| `/api/v1/download/track/transcode/status` | 查询下载转码状态 | 内部下载流程 |
| `/api/v1/download/track/transcode/delete` | 清理下载转码任务 | 内部下载流程 |
| `/api/v1/download/track/transcode/file` | 获取转码后的下载文件 | 可能返回二进制 |
| `/api/v1/detail/album`、`/detail/artist`、`/detail/lyrics/:id` | 云元数据详情调试接口 | 很可能受 `app.enable-dev-apis` 控制 |
| `/api/v1/search/tracks/best`、`/search/lyrics/best` | 云端最佳匹配调试接口 | 很可能仅开发配置启用 |
| `/api/v1/track/search`、`/track/lyrics`、`/media` | 云刮削/媒体服务内部适配接口 | 不作为 H5 API 使用 |

二进制还出现 `/app-center/authed-dir/sub/create` 字符串。正式前端只调用 `/sub/list`；该路径可能属于 fnOS 目录授权服务的内部调用或未启用功能，不计入稳定接口。

## 9. 后台服务逻辑

### 9.1 音乐库扫描流水线

```text
fnOS 授权目录
  -> 文件扫描/文件监控
  -> 音频探测与标签/CUE/封面/歌词提取
  -> artist / album / track / genre 归一化入库
  -> 可选云元数据刮削与歌词下载
  -> Bleve 搜索索引更新
  -> Task 状态供前端轮询
```

支持创建、修改、移动和删除事件的增量同步。物理文件删除后，`audio_file.is_physical_file_deleted` 和 `track.is_audio_file_deleted` 记录失效状态；管理删除另由 `track.is_admin_deleted` 表示。这解释了收藏和歌单中的“失效项统计/清理”能力。

### 9.2 元数据和歌词来源

`metadataPreference` 决定本地标签与云元数据的组合：

- `local_preferred`：本地优先，缺失时可用云端补全。
- `cloud_preferred`：云端优先，本地作为回退。
- `local_only`：仅使用本地文件、标签、CUE 和伴随资源。

歌词来源存储值：`1=embedded`、`2=sidecar`、`3=scraped`、`4=user_linked`。每个用户可独立保存首选歌词和毫秒偏移。

### 9.3 播放决策

服务端提供三条播放路径：

1. 浏览器支持的格式直接调用 `/track/stream`，并使用 Range 请求。
2. 不兼容格式调用 `/track/transcode`，再通过 `/track/hls/...` 播放，期间发送心跳并在结束时退出。
3. 原 PC 前端还包含 WebAssembly FFmpeg 客户端解码，但这是前端能力，不属于 Go 服务 API；移动 H5 不应把它当作唯一回退路径。

### 9.4 外部 fnOS 服务依赖

从二进制和包配置可确认服务端会连接 fnOS 的 Unix Socket/RPC 服务，能力包括：

- NAS OAuth 和管理员身份校验。
- 应用授权目录及子目录读取。
- 文件存储/文件监控事件。
- 媒体播放链接和音频信息。
- 系统信息、机器 ID 和服务版本。

这也是为什么服务端二进制不能脱离 fnOS 直接当普通 Linux 音乐服务器运行。

## 10. 数据持久化边界

SQLite schema 共覆盖 22 张业务表：

| 分类 | 表 |
| --- | --- |
| 应用 | `app_state` |
| 用户鉴权 | `user`、`user_oauth`、`user_token` |
| 曲库权限 | `shared_library`、`shared_library_member` |
| 媒体索引 | `audio_file`、`artist`、`album`、`track` |
| 多对多元数据 | `album_artist`、`track_artist`、`genre`、`track_genre` |
| 歌词 | `lyric`、`user_track_lyric_preference`、`user_track_lyric_offset` |
| 用户音乐数据 | `favorite_track`、`play_history`、`playlist`、`playlist_track` |
| UI 偏好 | `user_sorting_preference` |

因此收藏、历史、歌单、歌词偏移、排序、用户权限和登录 token 都是服务端持久能力，并非浏览器本地状态。

## 11. H5/PWA 接入注意事项

1. 保持部署在 `/music` 同源路径，共用 `music-token` 和 fnOS 反向代理。
2. 所有 JSON API 统一走签名客户端，不在业务组件内手写 fetch。
3. 启动顺序建议为：`initialization/state -> sys/config -> user/me`；`120001` 统一跳转登录。
4. 管理页根据 `user.role === 'admin'` 展示，但安全边界必须以后端校验为准。
5. 音频、封面和 HLS URL 不要带 `authx` 查询参数，也不要跨域代理，否则 Cookie/Range/HLS 相对路径会失效。
6. iOS 后台播放依赖原生 `<audio>`/Media Session 和 PWA 生命周期；优先直出或服务端 HLS。
7. PWA 离线缓存不应无界缓存音乐文件。API 和用户数据不应放入公共 Cache Storage；音频缓存需要按用户、容量和退出登录清理。
8. 对本文标注“路由确认”的少数接口，以及全部内部接口，应先在可回滚 fnOS 测试机抓包验证再接入生产 UI。

## 12. 覆盖结论

- 官方稳定契约：**82 个路径、85 个 HTTP 操作，已全部列入第 6 节**。
- 后台核心能力：鉴权、权限、曲库、扫描、元数据、歌词、搜索、播放、转码、任务与用户数据持久化均由原 Go 服务提供。
- H5 可以完全复用这些能力，无需新增后端；主要兼容点是 `authx`、Cookie、同源媒体 URL、管理员权限和 fnOS 授权目录。
- 内部/开发路径已单独列入第 8 节，不计入稳定接口总数。
