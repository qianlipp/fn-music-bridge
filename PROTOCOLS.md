# 播放协议支持

适用 FPK：`yinqiao 1.0.5`。所有协议共用本机 Yinqiao API 和飞牛音乐账号权限，
不会直接读取数据库，也不会修改官方 `trim.music` FPK。

## 服务总览

| 协议 | 客户端服务器地址 | 登录方式 | 状态 |
| --- | --- | --- | --- |
| OpenSubsonic/Subsonic 1.16.1 | `http://NAS_IP:CLIENT_PORT` | 飞牛音乐账号密码；Token 模式需先登记 | 支持，默认开启 |
| Synology DSM WebAPI / Audio Station | `http://NAS_IP:CLIENT_PORT` | 直接转发飞牛音乐账号密码 | 支持，默认开启 |
| Jellyfin 10.10 Audio | `http://NAS_IP:CLIENT_PORT` | 直接转发飞牛音乐账号密码 | 支持，默认开启 |
| Ampache API 6 | `http://NAS_IP:CLIENT_PORT` | 摘要登录，需先登记 | 支持，默认开启 |

`CLIENT_PORT` 默认是 `14040`，可在管理页“系统设置”中修改；各协议也可单独关闭。

机器可读能力声明：

```text
GET http://NAS_IP:14040/yinqiao/capabilities
```

响应分别包含 `supported` 和 `unsupported`，其中 `unsupportedStatus` 固定为
`不支持`。标准协议发现端点只声明真实实现的能力，不会声明无法工作的接口。

## OpenSubsonic/Subsonic

客户端选择 Subsonic、OpenSubsonic 或 Navidrome，服务器填写到端口，不要手工追加
`/rest`。实现能力：

- 服务探测、用户角色和音乐目录；
- 歌手、专辑、歌曲、流派浏览和详情；
- 搜索、随机歌曲、收藏；
- 歌单读取、创建、编辑、删除和歌曲增删；
- 原音频 Range 串流、下载和封面；
- 普通歌词、逐行同步歌词；
- 播放上报、扫描状态及管理员扫描。

不支持：播客、网络电台、视频、聊天、分享、Jukebox、书签和服务端播放队列持久化。
这些读取接口中一部分按 Subsonic 兼容惯例返回空集合；写接口不会伪造落库成功。

## Synology Audio Station

客户端选择 `Audio Station`，服务器填写 `http://NAS_IP:14040`。客户端先调用
`SYNO.API.Info` 发现接口，再通过 `SYNO.API.Auth` 使用飞牛音乐账号密码登录。
协议层签发 `_sid`，飞牛音乐 Token 不暴露给客户端。

已声明并实现：

| API | 方法 | 映射能力 |
| --- | --- | --- |
| `SYNO.API.Info` | `query` | 返回本适配器真实支持的 API 路径和版本 |
| `SYNO.API.Auth` | `login`, `logout` | 飞牛音乐密码登录和协议会话 |
| `SYNO.AudioStation.Info` | `getinfo` | 服务版本和入口 |
| `Album` | `list` | 专辑分页、名称和歌手过滤 |
| `Artist` | `list` | 歌手分页与搜索过滤 |
| `Song` | `list`, `search`, `getinfo`, `setrating` | 歌曲、详情、检索；5 星映射为收藏 |
| `Genre` | `list` | 流派分页 |
| `Folder` | `list`, `getinfo` | 以专辑生成稳定的虚拟音乐目录 |
| `Search` | `list` | 歌手、专辑和歌曲综合搜索 |
| `Cover` | `getsongcover`, `getcover` | 歌曲、专辑和歌手封面 |
| `Stream` | `stream` | 原音频 Range 串流 |
| `Download` | `download` | 原文件下载 |
| `Lyrics` | `getlyrics` | LRC 或纯文本歌词 |
| `Playlist` | `list`, `getinfo`, `create`, `delete`, `rename`, `updatesongs` | 歌单完整常用操作 |

不支持：QuickConnect、OTP、DSM 系统管理、RemotePlayer、MediaServer、网络电台、
歌词插件搜索、智能歌单、歌单分享、WebPlayer 队列和作曲家索引。上述 API 不出现在
`SYNO.API.Info` 返回值中；直接调用会收到 Synology 风格的明确错误。

## Jellyfin Audio

客户端选择 `Jellyfin`，服务器填写 `http://NAS_IP:14040`。适配器也兼容反向代理使用
`/Audio` 基础路径，例如 `https://example.com/Audio`。适配器实现 Jellyfin
音乐客户端实际需要的服务子集，不宣称自己是完整影视服务器。

已实现的主要路径：

| Method | Path | 能力 |
| --- | --- | --- |
| GET | `/System/Ping` | 连通性 |
| GET | `/System/Info/Public`, `/System/Info`, `/System/Endpoint` | 服务发现 |
| POST | `/Users/AuthenticateByName` | 飞牛音乐账号密码登录 |
| GET | `/Users/Me`, `/Users/{id}`, `/Users/{id}/Views` | 用户与音乐视图 |
| GET | `/Items/Counts` | 轻量读取歌曲、专辑、歌手和歌单数量 |
| GET | `/Users/{id}/Items`, `/Items`, `/Items/Latest` | 专辑、歌曲、歌单和搜索列表 |
| GET | `/Items/Root`, `/Items/Suggestions` | 音乐库根节点和随机可播放建议 |
| GET | `/Users/{id}/Items/Latest`, `/Search/Hints` | 首页最新内容和搜索提示 |
| GET | `/Users/{id}/Items/{id}`, `/Items/{id}` | 媒体详情 |
| GET | `/Artists`, `/Artists/AlbumArtists`, `/Genres`, `/MusicGenres` | 歌手和流派 |
| GET | `/Artists/{name}`, `/Genres/{name}`, `/MusicGenres/{name}` | 按名称读取歌手或流派 |
| GET | `/Items/{id}/Images/Primary` | 封面 |
| GET/POST | `/Items/{id}/PlaybackInfo` | 直连播放决策 |
| GET/HEAD | `/Audio/{id}/stream`, `/Audio/{id}/universal` | 原音频 Range 串流 |
| GET | `/Items/{id}/Download`, `/Items/{id}/File` | 原文件读取和下载 |
| GET | `/Audio/{id}/Lyrics` | 普通或时间轴歌词 |
| POST/DELETE | `/Users/{id}/FavoriteItems/{id}` | 歌曲收藏和取消收藏 |
| GET | `/UserItems/{id}/UserData` | 收藏与播放状态 |
| GET/POST | `/Playlists` | 读取或创建歌单 |
| GET/POST/DELETE | `/Playlists/{id}/Items` | 歌单曲目读取、添加和移除 |
| POST/DELETE | `/Items/{playlistId}` | 重命名或删除歌单 |
| POST | `/Sessions/Capabilities*`, `/Sessions/Playing*` | 客户端能力与播放状态上报 |
| POST | `/Sessions/Logout` | 注销并使当前 AccessToken 失效 |
| POST | `/Users/{id}/PlayedItems/{id}` | 标记歌曲已播放 |
| GET | `/ScheduledTasks`, `/ScheduledTasks/{id}` | 扫描状态和任务详情 |
| POST | `/Library/Refresh` | 管理员触发扫描 |

1.0.4 起 Jellyfin AccessToken 加密保存在音桥私有数据目录，服务重启时会重新
登录飞牛音乐并恢复客户端会话。收藏、播放历史、随机和搜索分别使用对应的
飞牛音乐数据源，不再统一退化为全曲库首页。

明确不支持：Similar、Instant Mix、歌词上传/删除、歌单曲目重排、协作歌单用户、视频、
Live TV、频道、插件、远程会话控制、SyncPlay、WebSocket 事件、字幕、硬件转码、
Jellyfin 系统管理，以及专辑/歌手收藏的新增和取消。收藏专辑/歌手浏览列表从收藏歌曲派生。登录策略会关闭视频、转码、远控和内容删除；
进入 Jellyfin 路由但无法映射的接口返回 HTTP `501`，其他非音乐路径可能返回 `404`。

逐 Method/Path 状态、真实测试数据和待复测变体见 `docs/JELLYFIN_10_10_7_MATRIX.md`。

## Ampache

保留 API 6 XML/JSON 兼容入口：

```text
/server/xml.server.php
/server/json.server.php
```

支持握手、探测、歌手、专辑、歌曲、流派、歌单、搜索、封面和 Range 播放。
不支持播客、视频、电台、目录管理、用户管理和本机播放器控制。

Ampache 和部分 Subsonic 客户端只发送密码摘要，服务端无法从摘要恢复飞牛音乐密码，
首次使用前需在桌面管理窗口完成一次加密登记。Synology 和 Jellyfin 登录会直接收到
密码并转发验证，因此不要求预登记。

## 协议开关

FPK 默认全部开启：

```text
YINQIAO_ENABLE_SUBSONIC=true
YINQIAO_ENABLE_AMPACHE=true
YINQIAO_ENABLE_SYNOLOGY=true
YINQIAO_ENABLE_JELLYFIN=true
```

设为 `false` 后对应路径返回 404，Yinqiao API 和其他协议不受影响。
