# Jellyfin 10.10.7 音乐接口实施矩阵

更新日期：2026-08-01

## 1. 状态口径

本矩阵以 Jellyfin 官方 `10.10.7` OpenAPI 为基线，只把能够映射到飞牛音乐的能力列为音桥支持。
官方 103 个音乐相关 Operation 的逐 Method/Path 清单见
`docs/JELLYFIN_10_10_7_OPERATIONS.md`。

| 状态 | 严格含义 |
| --- | --- |
| 候选真机通过 | `1.0.2` 候选二进制曾在 NAS 本机临时端口连接真实飞牛音乐服务，黑盒断言和脱敏证据均通过 |
| 代码待复测 | 源码已实现或共用已验证处理器，但该 Method/变体没有单独完成真机黑盒断言 |
| 明确不支持 | 底层无可靠等价能力，返回 `501` 或不进入 Jellyfin 路由，不以空数据冒充成功 |
| 待调研 | 飞牛可能存在近似能力，但缺少足够真实响应证据，当前不声明支持 |

这里的“候选真机通过”不等于正式域名已经升级。公开 HTTPS 在测试时仍运行 `1.0.1`；候选版本仅在 NAS
本机临时 `14042` 完成验收，测试后进程、文件、凭据和临时远程访问授权均已清理。

真实报告：

- `autotest/reports/20260801-192549-798f65-jellyfin/report.md`
- 结果：81 项通过、0 项失败、5 项明确不支持；临时歌单删除、收藏恢复原状态。

## 2. 发现、认证和用户

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| GET/POST | `/System/Ping` | 候选真机通过 | 返回 `Jellyfin Server` |
| GET | `/System/Info/Public` | 候选真机通过 | 返回稳定 ServerId 和兼容版本 `10.10.7` |
| GET | `/System/Info` | 候选真机通过 | 登录后服务信息 |
| GET | `/System/Endpoint` | 候选真机通过 | 返回局域网端点属性 |
| GET | `/Branding/Configuration` | 候选真机通过 | 空品牌配置 |
| GET | `/QuickConnect/Enabled` | 候选真机通过 | 明确返回 `false` |
| POST | `/Users/AuthenticateByName` | 候选真机通过 | 用户名密码转发飞牛音乐鉴权，返回协议 AccessToken |
| GET | `/Users` | 候选真机通过 | 仅返回当前登录用户 |
| GET | `/Users/Me` | 候选真机通过 | 当前登录用户 |
| GET | `/Users/{userId}` | 候选真机通过 | 仅允许读取当前用户 |
| GET | `/Users/Public` | 代码待复测 | 返回空公开用户数组，不泄露飞牛音乐用户 |
| GET | `/UserViews` | 候选真机通过 | 返回单一音乐视图 |
| GET | `/Users/{userId}/Views` | 候选真机通过 | 旧 Jellyfin/Emby 视图兼容 |
| GET | `/UserViews/GroupingOptions` | 代码待复测 | 单音乐库没有可用分组，返回空数组 |
| POST | `/Sessions/Logout` | 候选真机通过 | 删除当前 AccessToken；复测旧 Token 返回 401 |
| POST | `/Users`, `/Users/New`, `/Users/{id}/Policy` | 明确不支持 | 不允许播放器管理飞牛音乐用户 |
| DELETE | `/Users/{id}` | 明确不支持 | 不允许删除飞牛音乐用户 |
| POST | `/Users/Password` | 明确不支持 | 不允许经播放器修改飞牛音乐密码 |
| POST | `/Users/Configuration` | 明确不支持 | 音桥不保存完整 Jellyfin 用户配置模型 |
| POST | Quick Connect/找回密码接口 | 明确不支持 | 飞牛音乐没有 Jellyfin Quick Connect/找回密码模型 |
| GET | `/Auth/Providers`, `/Auth/PasswordResetProviders` | 明确不支持 | 仅提供飞牛音乐用户名密码入口 |

固定路由段和 `/Audio`、`/emby`、`/jellyfin` 命名空间不区分大小写；动态 ID 原样保留。混合大小写
`/AuDiO/EmBy/iTeMs` 已真机跨页验证。

## 3. 曲库、分页和搜索

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| GET | `/Items/Counts` | 候选真机通过 | 真实返回歌曲 57,795、专辑 10,268、歌手 4,381、流派单列读取 |
| GET | `/Items/Root` | 候选真机通过 | 返回音乐 CollectionFolder，不再误当歌曲 ID |
| GET | `/Items` | 候选真机通过 | Audio/MusicAlbum/MusicArtist/MusicGenre/Playlist 分页 |
| GET | `/Users/{userId}/Items` | 候选真机通过 | 旧客户端兼容入口 |
| GET | `/Items/Latest` | 候选真机通过 | 返回真实最新音乐数据 |
| GET | `/Items/Suggestions` | 候选真机通过 | 从真实曲库随机页生成可播放建议，不声称个性化推荐 |
| GET | `/Items/{itemId}` | 候选真机通过 | 歌曲、专辑、歌手、流派、歌单动态详情 |
| GET | `/Artists`, `/Artists/AlbumArtists` | 候选真机通过 | 真实分页和总数 |
| GET | `/Artists/{name}` | 候选真机通过 | 通过飞牛搜索后精确匹配名称 |
| GET | `/Genres`, `/MusicGenres` | 候选真机通过 | 真实分页和总数 |
| GET | `/Genres/{name}`, `/MusicGenres/{name}` | 候选真机通过 | 名称精确匹配 |
| GET | `/Search/Hints` | 候选真机通过 | 歌手、专辑、歌曲搜索提示 |
| GET | `?ParentId=album/artist/genre/playlist` | 候选真机通过 | 分别映射对应的飞牛曲目分页接口 |
| GET | `/UserItems/Resume` | 待调研 | 可考虑映射 `/me/history`，但历史响应和断点字段尚未真机确认 |
| GET | `/Albums|Artists|Items/{id}/Similar` | 明确不支持 | 当前统一 501；不再返回虚假空集合 |
| GET | 所有 Instant Mix 路径 | 明确不支持 | 飞牛漫游没有可靠的指定种子相似播放语义，统一 501 |
| GET | Ancestors/CriticReviews/ThemeMedia/ThemeSongs | 明确不支持 | 飞牛音乐无对应元数据模型 |
| GET | ThemeVideos/Movies/Shows/Trailers Similar | 明确不支持 | 非音乐能力 |
| GET | Library PhysicalPaths/MediaFolders/AvailableOptions | 明确不支持 | 不向播放器暴露 NAS 文件系统和库管理配置 |

分页已真实验证 `StartIndex=45&Limit=20` 跨越上游两页，返回 20 条且总数保持 57,795。上游每页可靠
上限为 50，音桥按窗口逐页请求，不再把当前页长度当作总数。

## 4. 图片、歌词和媒体

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| GET/HEAD | `/Items/{id}/Images/Primary` | 候选真机通过 | 返回真实封面二进制；GET 和 HEAD 均通过 |
| GET/HEAD | `/Items/{id}/Images/{type}/{index}/...` | 代码待复测 | 共用封面处理器，忽略只影响缩放的 Jellyfin 变体参数 |
| GET | `/Audio/{id}/Lyrics` | 候选真机通过 | 解析 LRC 为 Jellyfin 时间轴歌词结构 |
| POST/DELETE | `/Audio/{id}/Lyrics` | 明确不支持 | 不支持上传、替换或删除飞牛歌词，统一 501 |
| GET/POST | `/Items/{id}/PlaybackInfo` | 候选真机通过 | 只声明 DirectPlay/DirectStream，不声明转码 |
| GET | `/Audio/{id}/stream` | 候选真机通过 | 原音频，Range 真实返回 206 |
| HEAD | `/Audio/{id}/stream` | 候选真机通过 | 返回真实媒体头 |
| GET | `/Audio/{id}/stream.{container}` | 候选真机通过 | 容器后缀兼容，仍返回原文件 |
| HEAD | `/Audio/{id}/stream.{container}` | 代码待复测 | 共用同一媒体处理器 |
| GET | `/Audio/{id}/universal` | 候选真机通过 | 不转码，直接返回原音频 Range 数据 |
| HEAD | `/Audio/{id}/universal` | 代码待复测 | 共用同一媒体处理器 |
| GET | `/Items/{id}/Download` | 候选真机通过 | 原文件 Range 下载 |
| GET | `/Items/{id}/File` | 候选真机通过 | 原文件 Range 下载；不再误当歌曲 ID 扫描 |
| HEAD | Download/File | 代码待复测 | 处理器支持 HEAD，尚无独立真机证据 |
| POST | `/LiveStreams/Open`, `/LiveStreams/Close` | 明确不支持 | 无 Jellyfin 转码/直播会话 |
| GET | `/Playback/BitrateTest` | 明确不支持 | 不提供 Jellyfin 带宽测试数据生成器 |
| Remote lyrics/provider 接口 | 明确不支持 | 不伪装 Jellyfin 在线歌词提供商 |

三种已测媒体入口读取同一首真实歌曲前 4 KiB，均得到 `206` 和相同
`Content-Range: bytes 0-4095/14055261`，没有下载整首文件。

## 5. 收藏、历史和播放状态

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| GET | `/UserItems/{id}/UserData` | 候选真机通过 | 返回真实歌曲收藏状态；播放位置当前为 0 |
| POST | `/UserItems/{id}/UserData` | 代码待复测 | 支持歌曲收藏和 `Played=true`；断点与清除历史明确不支持 |
| POST/DELETE | `/UserFavoriteItems/{id}` | 候选真机通过 | 收藏/取消收藏；测试后恢复原状态 |
| POST/DELETE | `/Users/{uid}/FavoriteItems/{id}` | 代码待复测 | 旧客户端兼容，仍只允许歌曲收藏 |
| POST | `/UserPlayedItems/{id}` | 候选真机通过 | 上报真实 `track_play` 事件 |
| DELETE | `/UserPlayedItems/{id}` | 待调研 | 飞牛存在历史删除能力，但 body 尚未完成真机确认 |
| POST | `/Users/{uid}/PlayedItems/{id}` | 代码待复测 | 旧客户端播放上报 |
| POST/DELETE | `/PlayingItems/{id}[/Progress]` | 代码待复测 | 开始/进度返回 204，停止时上报播放事件 |
| POST | `/Sessions/Playing`, `/Progress` | 候选真机通过 | 接收当前设备播放状态 |
| POST | `/Sessions/Playing/Ping` | 候选真机通过 | 返回 204 |
| POST | `/Sessions/Playing/Stopped` | 代码待复测 | 停止时上报播放事件 |
| Rating 接口 | 明确不支持 | 飞牛收藏不等同 0-5 星评分，不返回虚假成功 |

## 6. 播放列表

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| GET | `/Playlists` | 候选真机通过 | 当前用户歌单真实列表 |
| POST | `/Playlists` | 候选真机通过 | 创建歌单并可附带初始曲目 |
| GET | `/Playlists/{id}` | 候选真机通过 | 歌单详情 |
| POST | `/Playlists/{id}` | 候选真机通过 | 歌单改名 |
| GET | `/Playlists/{id}/Items` | 候选真机通过 | 真实分页、真实总数 |
| POST | `/Playlists/{id}/Items` | 候选真机通过 | 批量添加歌曲 |
| DELETE | `/Playlists/{id}/Items` | 候选真机通过 | 按 PlaylistItemId/歌曲 ID 移除歌曲 |
| DELETE | `/Items/{playlistId}` | 候选真机通过 | 兼容旧客户端删除歌单 |
| POST | `/Playlists/{id}/Items/{itemId}/Move/{index}` | 明确不支持 | 飞牛无可靠顺序重排接口，返回 501 |
| Playlists Users 系列 | 明确不支持 | 飞牛歌单没有 Jellyfin 协作用户权限模型 |

真实写入闭环：创建含 1 首歌的临时歌单 -> 改名 -> 添加第二首 -> 确认总数 2 -> 移除第二首 ->
确认总数 1 -> 删除歌单。所有清理动作通过。

## 7. 会话、任务和管理

| Method | Path | 状态 | 实际行为 |
| --- | --- | --- | --- |
| POST | `/Sessions/Capabilities`, `/Full` | 候选真机通过 | 接受客户端能力，返回 204 |
| POST | `/Sessions/Viewing` | 代码待复测 | 接收当前设备查看状态，返回 204 |
| GET | `/Sessions` | 明确不支持 | 不暴露或伪造其他设备会话 |
| Sessions remote command 系列 | 明确不支持 | 不控制 NAS 本机或其他播放器 |
| GET | `/ScheduledTasks` | 候选真机通过 | 返回飞牛音乐扫描任务状态 |
| GET | `/ScheduledTasks/{taskId}` | 候选真机通过 | `1.0.2` 正式源码使用 `yinqiao-scan`；候选真机曾验证同一处理器的旧内部 ID |
| POST | `/ScheduledTasks/Running/yinqiao-scan` | 代码待复测 | 管理员触发飞牛音乐全库扫描 |
| DELETE | `/ScheduledTasks/Running/{id}` | 明确不支持 | 上游没有已验证的安全取消扫描接口 |
| POST | `/Library/Refresh` | 代码待复测 | 映射管理员全库扫描；自动测试默认不执行 |
| System Logs/Restart/Shutdown/WOL | 明确不支持 | 播放器协议不应获得 NAS 管理能力 |
| Plugins/Live TV/Video/Channels/SyncPlay | 明确不支持 | 非飞牛音乐服务能力 |

## 8. 发布前剩余验证

1. 安装新 FPK 后，通过正式 HTTPS 地址重跑 `autotest/run-jellyfin.sh --mutations`；临时候选真机结果不能替代正式安装验证。
2. 用箭头音乐完成首页、分页、搜索、歌单、封面、歌词和连续播放人工流程，并对齐请求日志。
3. 对所有“代码待复测”变体逐个增加黑盒用例；通过前不升级状态。
4. 对播放历史删除、Resume 和扫描触发先抓取真实飞牛响应，再决定实现；禁止依据接口名称猜 body。
5. 视频、转码、远程控制和服务器管理能力保持明确不支持。
