# Jellyfin 10.10.7 音乐相关 Operation 全量清单

更新日期：2026-08-01

来源为 Jellyfin 官方 `10.10.7` OpenAPI。按 Artists、Audio、Genres、Items、Library、
Lyrics、MediaInfo、Playlists、Search、Session、System、User 和 UserViews 标签抽取，共
103 个 Operation。这里逐项列出，不用分组描述代替单接口结论。

状态口径：`真机通过`指候选二进制连接真实曲库有独立黑盒证据；`代码待复测`指已实现但
没有作为正式 FPK 通过 HTTPS 复测；`条件支持`指只支持可可靠映射的资源类型；`待调研`
指缺少上游真实字段证据；`不支持`统一不声明能力，并返回 501 或不进入音乐兼容路由。

| # | Method | Path | 状态 | 音桥行为 |
| ---: | --- | --- | --- | --- |
| 1 | GET | `/Artists` | 真机通过 | 歌手真实分页和总数 |
| 2 | GET | `/Artists/AlbumArtists` | 真机通过 | 与歌手目录共用真实数据 |
| 3 | GET | `/Artists/{name}` | 真机通过 | 搜索后按名称精确匹配 |
| 4 | GET | `/Audio/{itemId}/stream` | 真机通过 | 原音频 Range 串流 |
| 5 | HEAD | `/Audio/{itemId}/stream` | 真机通过 | 原媒体响应头 |
| 6 | GET | `/Audio/{itemId}/stream.{container}` | 真机通过 | 后缀兼容，不执行转码 |
| 7 | HEAD | `/Audio/{itemId}/stream.{container}` | 代码待复测 | 共用原媒体 HEAD 处理器 |
| 8 | GET | `/Genres` | 真机通过 | 流派真实分页和总数 |
| 9 | GET | `/Genres/{genreName}` | 真机通过 | 按名称精确匹配流派 |
| 10 | GET | `/Items` | 真机通过 | 音乐类型筛选、搜索、父级和跨页窗口 |
| 11 | GET | `/UserItems/Resume` | 待调研 | 上游历史存在近似能力，但断点字段未确认 |
| 12 | GET | `/UserItems/{itemId}/UserData` | 真机通过 | 返回歌曲真实收藏状态 |
| 13 | POST | `/UserItems/{itemId}/UserData` | 代码待复测 | 支持歌曲收藏和 Played=true；拒绝断点及清除历史 |
| 14 | GET | `/Albums/{itemId}/Similar` | 不支持 | 无可靠专辑相似度语义 |
| 15 | GET | `/Artists/{itemId}/Similar` | 不支持 | 无可靠歌手相似度语义 |
| 16 | DELETE | `/Items` | 不支持 | 不允许播放器批量删除官方曲库内容 |
| 17 | GET | `/Items/Counts` | 真机通过 | 真实歌曲、专辑、歌手和歌单计数 |
| 18 | DELETE | `/Items/{itemId}` | 条件支持 | 仅允许删除当前用户歌单 |
| 19 | GET | `/Items/{itemId}/Ancestors` | 不支持 | 无 Jellyfin 层级祖先模型 |
| 20 | GET | `/Items/{itemId}/CriticReviews` | 不支持 | 无影评/评论模型 |
| 21 | GET | `/Items/{itemId}/Download` | 真机通过 | 原文件 Range 下载 |
| 22 | GET | `/Items/{itemId}/File` | 真机通过 | 原文件 Range 读取 |
| 23 | GET | `/Items/{itemId}/Similar` | 不支持 | 不返回虚假空集合 |
| 24 | GET | `/Items/{itemId}/ThemeMedia` | 不支持 | 无主题媒体模型 |
| 25 | GET | `/Items/{itemId}/ThemeSongs` | 不支持 | 无主题音乐模型 |
| 26 | GET | `/Items/{itemId}/ThemeVideos` | 不支持 | 非音乐能力 |
| 27 | GET | `/Libraries/AvailableOptions` | 不支持 | 不暴露 NAS 曲库配置 |
| 28 | POST | `/Library/Media/Updated` | 不支持 | 不接受 Jellyfin 文件监控事件 |
| 29 | GET | `/Library/MediaFolders` | 不支持 | 不暴露 NAS 文件系统目录 |
| 30 | POST | `/Library/Movies/Added` | 不支持 | 非音乐能力 |
| 31 | POST | `/Library/Movies/Updated` | 不支持 | 非音乐能力 |
| 32 | GET | `/Library/PhysicalPaths` | 不支持 | 不暴露 NAS 物理路径 |
| 33 | POST | `/Library/Refresh` | 代码待复测 | 管理员触发飞牛音乐全库扫描 |
| 34 | POST | `/Library/Series/Added` | 不支持 | 非音乐能力 |
| 35 | POST | `/Library/Series/Updated` | 不支持 | 非音乐能力 |
| 36 | GET | `/Movies/{itemId}/Similar` | 不支持 | 非音乐能力 |
| 37 | GET | `/Shows/{itemId}/Similar` | 不支持 | 非音乐能力 |
| 38 | GET | `/Trailers/{itemId}/Similar` | 不支持 | 非音乐能力 |
| 39 | DELETE | `/Audio/{itemId}/Lyrics` | 不支持 | 不删除官方歌词 |
| 40 | GET | `/Audio/{itemId}/Lyrics` | 真机通过 | LRC 转 Jellyfin 时间轴歌词结构 |
| 41 | POST | `/Audio/{itemId}/Lyrics` | 不支持 | 不上传或替换官方歌词 |
| 42 | GET | `/Audio/{itemId}/RemoteSearch/Lyrics` | 不支持 | 无 Jellyfin 在线歌词提供商 |
| 43 | POST | `/Audio/{itemId}/RemoteSearch/Lyrics/{lyricId}` | 不支持 | 无远程歌词下载能力 |
| 44 | GET | `/Providers/Lyrics/{lyricId}` | 不支持 | 无远程歌词缓存模型 |
| 45 | GET | `/Items/{itemId}/PlaybackInfo` | 真机通过 | 只声明 DirectPlay/DirectStream |
| 46 | POST | `/Items/{itemId}/PlaybackInfo` | 真机通过 | 与 GET 返回等价直连决策 |
| 47 | POST | `/LiveStreams/Close` | 不支持 | 无转码/直播会话 |
| 48 | POST | `/LiveStreams/Open` | 不支持 | 无转码/直播会话 |
| 49 | GET | `/Playback/BitrateTest` | 不支持 | 不生成 Jellyfin 带宽测试数据 |
| 50 | POST | `/Playlists` | 真机通过 | 创建歌单，可附带初始曲目 |
| 51 | GET | `/Playlists/{playlistId}` | 真机通过 | 歌单详情 |
| 52 | POST | `/Playlists/{playlistId}` | 真机通过 | 歌单改名 |
| 53 | DELETE | `/Playlists/{playlistId}/Items` | 真机通过 | 按歌曲/条目 ID 移除 |
| 54 | GET | `/Playlists/{playlistId}/Items` | 真机通过 | 歌曲真实分页和总数 |
| 55 | POST | `/Playlists/{playlistId}/Items` | 真机通过 | 批量添加歌曲 |
| 56 | POST | `/Playlists/{playlistId}/Items/{itemId}/Move/{newIndex}` | 不支持 | 上游无可靠重排接口 |
| 57 | GET | `/Playlists/{playlistId}/Users` | 不支持 | 无协作歌单用户模型 |
| 58 | DELETE | `/Playlists/{playlistId}/Users/{userId}` | 不支持 | 无协作歌单用户模型 |
| 59 | GET | `/Playlists/{playlistId}/Users/{userId}` | 不支持 | 无协作歌单用户模型 |
| 60 | POST | `/Playlists/{playlistId}/Users/{userId}` | 不支持 | 无协作歌单用户模型 |
| 61 | GET | `/Search/Hints` | 真机通过 | 歌手、专辑和歌曲搜索提示 |
| 62 | GET | `/Auth/PasswordResetProviders` | 不支持 | 账号密码由飞牛音乐管理 |
| 63 | GET | `/Auth/Providers` | 不支持 | 只接受飞牛音乐密码登录 |
| 64 | GET | `/Sessions` | 不支持 | 不暴露或伪造其他播放器会话 |
| 65 | POST | `/Sessions/Capabilities` | 真机通过 | 接受当前客户端能力声明 |
| 66 | POST | `/Sessions/Capabilities/Full` | 真机通过 | 接受完整能力声明 |
| 67 | POST | `/Sessions/Logout` | 真机通过 | 删除 Token，旧 Token 随后返回 401 |
| 68 | POST | `/Sessions/Viewing` | 代码待复测 | 接受当前客户端查看状态，不控制设备 |
| 69 | POST | `/Sessions/{sessionId}/Command` | 不支持 | 不远程控制其他客户端 |
| 70 | POST | `/Sessions/{sessionId}/Command/{command}` | 不支持 | 不远程控制其他客户端 |
| 71 | POST | `/Sessions/{sessionId}/Message` | 不支持 | 不向其他会话推送消息 |
| 72 | POST | `/Sessions/{sessionId}/Playing` | 不支持 | 不控制其他会话播放 |
| 73 | POST | `/Sessions/{sessionId}/Playing/{command}` | 不支持 | 不控制其他会话播放状态 |
| 74 | POST | `/Sessions/{sessionId}/System/{command}` | 不支持 | 不执行远程系统命令 |
| 75 | DELETE | `/Sessions/{sessionId}/User/{userId}` | 不支持 | 无共享会话用户模型 |
| 76 | POST | `/Sessions/{sessionId}/User/{userId}` | 不支持 | 无共享会话用户模型 |
| 77 | POST | `/Sessions/{sessionId}/Viewing` | 不支持 | 不控制其他客户端界面 |
| 78 | GET | `/System/Endpoint` | 真机通过 | 返回本地网络属性 |
| 79 | GET | `/System/Info` | 真机通过 | 登录后服务信息 |
| 80 | GET | `/System/Info/Public` | 真机通过 | 公开发现信息和稳定 ServerId |
| 81 | GET | `/System/Logs` | 不支持 | 不通过协议暴露 NAS 日志 |
| 82 | GET | `/System/Logs/Log` | 不支持 | 不通过协议下载 NAS 日志 |
| 83 | GET | `/System/Ping` | 真机通过 | 连通性探测 |
| 84 | POST | `/System/Ping` | 真机通过 | 连通性探测 |
| 85 | POST | `/System/Restart` | 不支持 | 不允许播放器重启服务/NAS |
| 86 | POST | `/System/Shutdown` | 不支持 | 不允许播放器关闭服务/NAS |
| 87 | GET | `/System/WakeOnLanInfo` | 不支持 | 不伪造 WOL 信息 |
| 88 | GET | `/Users` | 真机通过 | 只返回当前登录用户 |
| 89 | POST | `/Users` | 不支持 | 不通过播放器更新用户 |
| 90 | POST | `/Users/AuthenticateByName` | 真机通过 | 转发飞牛音乐账号密码验证 |
| 91 | POST | `/Users/AuthenticateWithQuickConnect` | 不支持 | Quick Connect 明确关闭 |
| 92 | POST | `/Users/Configuration` | 不支持 | 不保存完整 Jellyfin 用户配置 |
| 93 | POST | `/Users/ForgotPassword` | 不支持 | 密码找回由飞牛音乐负责 |
| 94 | POST | `/Users/ForgotPassword/Pin` | 不支持 | 密码找回由飞牛音乐负责 |
| 95 | GET | `/Users/Me` | 真机通过 | 当前登录用户 |
| 96 | POST | `/Users/New` | 不支持 | 不通过播放器创建飞牛音乐用户 |
| 97 | POST | `/Users/Password` | 不支持 | 不通过播放器修改飞牛音乐密码 |
| 98 | GET | `/Users/Public` | 代码待复测 | 返回空数组，不泄露用户列表 |
| 99 | DELETE | `/Users/{userId}` | 不支持 | 不允许删除飞牛音乐用户 |
| 100 | GET | `/Users/{userId}` | 真机通过 | 仅允许当前用户 ID |
| 101 | POST | `/Users/{userId}/Policy` | 不支持 | 不允许提升飞牛音乐权限 |
| 102 | GET | `/UserViews` | 真机通过 | 单一音乐库视图 |
| 103 | GET | `/UserViews/GroupingOptions` | 代码待复测 | 单音乐库无可用分组，返回空数组 |

音桥还实现了部分旧 Jellyfin/Emby 客户端常用但未出现在上述抽取标签中的兼容路径，例如
`/Items/Root`、`/Items/Latest`、`/Items/Suggestions`、`/MusicGenres`、
`/UserFavoriteItems`、`/UserPlayedItems`、`/PlayingItems`、`/Sessions/Playing*`、
`/ScheduledTasks` 和 `/Audio/{id}/universal`。这些路径的逐项状态见
`JELLYFIN_10_10_7_MATRIX.md`。

正式发布门槛：安装新 FPK 后通过公网 HTTPS 执行 `autotest/run-jellyfin.sh --mutations`，
只有新的响应证据通过后，才可把 `代码待复测` 升级为 `真机通过`。
