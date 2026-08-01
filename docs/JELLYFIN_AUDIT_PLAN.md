# Jellyfin 10.10.7 音乐接口审计与实施计划

更新日期：2026-08-01

## 1. 范围与完成标准

基线固定为 Jellyfin 官方 `10.10.7` OpenAPI：

- https://api.jellyfin.org/openapi/stable/jellyfin-openapi-10.10.7.json
- 箭头音乐官方 `1.2.4` APK：
  https://github.com/amcfy-music/amcfy-music/releases/tag/1.2.4

只实现能够映射到飞牛音乐的音乐客户端能力。视频、Live TV、频道、插件、远程控制、
服务器关机重启等没有底层能力的接口明确返回 `501`，不返回虚假成功。

状态定义：

| 状态 | 含义 |
| --- | --- |
| 现网已通过 | 已在受控真实 HTTPS 环境使用测试账号验证响应与数据 |
| 现网失败 | 已通过真实 HTTPS 复现错误、超时或错误数据 |
| 候选真机通过 | 候选二进制在 NAS 临时端口连接真实飞牛服务通过，但正式 FPK/HTTPS 尚未复测 |
| 源码待部署 | 代码和单元测试已通过，但尚未安装到 NAS，不算完成 |
| 计划实现 | 已确认飞牛音乐存在可映射能力，尚未完成 |
| 明确不支持 | 飞牛音乐没有对应能力或不应通过播放器协议开放 |

接口只有同时满足以下条件才改为“现网已通过”：

1. 正常路径单元测试通过；
2. 大小写、`/emby`、`/Audio` 命名空间等适用兼容测试通过；
3. 真实 HTTPS 返回正确状态、结构、数据数量和动态实体 ID；
4. 写接口完成创建、读取、修改、删除闭环，测试数据已清理；
5. 箭头音乐不再在对应流程报错，服务日志没有隐藏的上游错误。

## 2. 真实基线与候选结果

### 2.1 `1.0.2` 候选真机结果

候选二进制曾在 NAS 本机临时 `14042` 端口连接真实 `18090` 音桥 API，使用同一黑盒客户端
完成只读和可回滚写入验收：

- 81 项通过、0 项失败、5 项明确不支持；
- 真分页总数：歌曲 57,795、专辑 10,268、歌手 4,381、流派 136；
- `StartIndex=45&Limit=20` 标准和混合大小写路径均返回完整 20 条；
- 音频 stream/universal/download/file 均真实返回 206 Range；
- 临时歌单完成创建、改名、添加、移除和删除；
- 收藏切换后恢复原状态；注销后旧 Token 返回 401。

证据见 `autotest/reports/20260801-192549-798f65-jellyfin/` 和
`docs/JELLYFIN_10_10_7_MATRIX.md`。测试后临时进程、文件、凭据和临时远程访问授权均已清理。

这不等于正式域名已升级，因此矩阵使用“候选真机通过”，不写“现网已通过”。

### 2.2 `1.0.1` 正式 HTTPS 历史基线

现网版本：`1.0.1`。

已通过真实 HTTPS 的基础读取：

- `/System/Info`、`/System/Endpoint`
- `/Users`、`/Users/Me`、`/Users/{id}/Views`、`/UserViews`
- `/Items` 的 `Audio`、`MusicAlbum`、`MusicArtist`、`MusicGenre`、`Playlist`
- `/Items/Latest`
- `/Artists`、`/Artists/AlbumArtists`
- `/Genres`、`/MusicGenres`
- `/Search/Hints`
- `/ScheduledTasks`
- `/DisplayPreferences/{id}`

这些“HTTP 200”仍有待修数据问题：

- 歌曲和专辑的 `TotalRecordCount` 只有 `50`，上游真实分页总数未透传；
- 当前实现会先取一页再本地分页，`StartIndex` 大于 50 时会错误返回空列表；
- 因此列表接口目前只能算“可返回样例数据”，不能算完整打通。

已通过真实日志确认的失败：

| 接口 | 现象 | 根因 | 当前状态 |
| --- | --- | --- | --- |
| `GET /Items/Counts` | 扫描到第 28/69 页后超时 | `Counts` 被当成歌曲 ID | 源码待部署 |
| `GET /Playlists` | HTTP 501 | 只实现了创建歌单的 POST | 源码待部署 |
| `GET /Items/Root` | 8 秒超时 | `Root` 被当成歌曲 ID | 计划实现 |
| `GET /Items/Suggestions` | 8 秒超时 | `Suggestions` 被当成歌曲 ID | 计划实现 |
| 所有歌曲/专辑列表 | 总数固定为 50 | 丢失上游 `total` 并错误本地分页 | 计划重构 |

相对上述历史基线，`1.0.2` 工作区已经完成：

- 保留真实总数的曲目、专辑、歌手、流派和歌单跨页读取；
- `Counts`、`Root`、`Suggestions`、`File` 等保留路径显式路由；
- 固定路由段和多命名空间大小写兼容；
- stream、universal、download 和 file 原音频 Range；
- 用户数据、歌曲收藏、播放上报和会话注销；
- 歌单读取、创建、改名、加歌、移歌和删除闭环；
- 不支持 Method、Similar 和 Instant Mix 明确返回 501；
- 脱敏请求日志、专项黑盒客户端和对应单元测试。

以上仍需安装正式 FPK 并完成真实 HTTPS 和箭头音乐复测后，才能标记“现网已通过”。

## 3. 分阶段实施顺序

### 阶段 A：路由与分页基础

这是后续所有接口的前置条件。

| 接口/问题 | 实现要求 | 真实测试 |
| --- | --- | --- |
| 路由保留字 | `Counts`、`Root`、`Suggestions` 必须先于 `/Items/{id}` 匹配 | 每个接口 2 秒内响应，不触发 `/catalog/tracks` 全库扫描 |
| 大小写 | 只对固定路由段做不区分大小写匹配，动态 ID 保留原值 | 标准、全小写和混合大小写各一次 |
| 命名空间 | `/`、`/emby`、`/jellyfin`、`/Audio`、`/Audio/emby` | 同一接口在所有适用前缀返回等价结构 |
| 分页 | 将 `StartIndex/Limit` 换算为上游 `page/size` | 测试 0、第一页末尾、跨页和超出总数 |
| 总数 | 使用上游 `total`，不能用当前页长度代替 | 与上游真实总数一致，不再固定为 50 |
| 排序 | 支持箭头音乐实际发送的 `SortBy/SortOrder` | 验证首尾 ID 和稳定顺序 |
| 查询日志 | 记录 method/path/query/status/duration/client，隐藏凭据 | 日志可关联请求且不包含 token/password |

### 阶段 B：发现、认证和用户

| Method | Path | 计划 | 判定 |
| --- | --- | --- | --- |
| GET/POST | `/System/Ping` | 两种方法均返回 Jellyfin 兼容内容 | 必须实现 |
| GET | `/System/Info/Public` | 服务发现、版本和稳定 ServerId | 必须实现 |
| GET | `/System/Info`, `/System/Endpoint` | 登录后信息与反代外部地址 | 必须实现 |
| GET | `/Branding/Configuration` | 返回空品牌配置 | 必须实现 |
| GET | `/QuickConnect/Enabled` | 明确返回 false | 必须实现 |
| POST | `/Users/AuthenticateByName` | 用户名密码转发飞牛音乐登录 | 必须实现 |
| GET | `/Users`, `/Users/Me`, `/Users/{id}` | 当前登录用户 | 必须实现 |
| GET | `/Users/Public` | 返回空公开用户列表 | 必须实现 |
| GET | `/UserViews`, `/Users/{id}/Views` | 单一音乐视图 | 必须实现 |
| POST/DELETE | 用户管理接口 | 不允许播放器管理飞牛音乐用户 | 明确不支持 |

### 阶段 C：曲库浏览

| Method | Path | 飞牛映射 | 计划 |
| --- | --- | --- | --- |
| GET | `/Items/Counts` | 曲目、专辑、歌手、歌单分页 total | 必须实现 |
| GET | `/Items/Root` | 单一音乐根目录 | 必须实现 |
| GET | `/Items` | 曲目/专辑/歌手/流派/歌单分页 | 必须重构分页 |
| GET | `/Items/Latest` | 按创建时间读取最新音乐 | 必须实现 |
| GET | `/Items/Suggestions` | 漫游或随机曲目 | 必须实现真实数据 |
| GET | `/Items/{id}` | 动态类型详情 | 必须实现 |
| GET | `/Users/{id}/Items` | 兼容旧 Jellyfin/Emby 客户端 | 必须实现 |
| GET | `/Artists`, `/Artists/AlbumArtists` | 歌手分页 | 必须重构分页 |
| GET | `/Artists/{name}` | 按名称查询歌手 | 可以实现 |
| GET | `/Genres`, `/MusicGenres` | 流派分页 | 必须重构分页 |
| GET | `/Genres/{name}`, `/MusicGenres/{name}` | 流派详情 | 可以实现 |
| GET | `/Search/Hints` | 曲目、专辑、歌手搜索 | 必须实现分页与去重 |
| GET | `/Albums|Artists|Items/{id}/Similar` | 飞牛漫游/相关推荐 | 有真实数据才实现，否则明确不支持 |
| GET | Instant Mix 系列 | 飞牛漫游接口 | 调研响应后实现 |

### 阶段 D：图片、歌词和播放

| Method | Path | 计划 |
| --- | --- | --- |
| GET/HEAD | `/Items/{id}/Images/{type}` | 封面、Content-Type、Content-Length、缓存头 |
| GET/HEAD | 带 index/tag/format/size 的图片路径 | 忽略不影响内容的变体参数，保留二进制语义 |
| GET | `/Audio/{id}/Lyrics` | 时间轴歌词；无歌词返回空结构而不是 5xx |
| POST/DELETE | 歌词上传/删除 | 底层无普通用户能力，明确不支持 |
| GET/POST | `/Items/{id}/PlaybackInfo` | DirectPlay/DirectStream 数据与可用 URL |
| GET/HEAD | `/Audio/{id}/stream[.{container}]` | 原音频、Range、HEAD 和容器后缀 |
| GET/HEAD | `/Audio/{id}/universal` | 能直连则实现；转码参数无法满足时明确说明 |
| GET | `/Items/{id}/Download`, `/Items/{id}/File` | 原文件 Range 下载 |

每个媒体接口使用动态取得的真实歌曲和封面 ID；音频只读取前 4 KiB，不下载整首。

### 阶段 E：用户数据

| Method | Path | 飞牛映射 | 计划 |
| --- | --- | --- | --- |
| GET | `/UserItems/{id}/UserData` | 收藏和播放状态 | 可以实现 |
| POST/DELETE | `/UserFavoriteItems/{id}` | 收藏/取消收藏 | 必须实现并恢复原状态 |
| POST/DELETE | `/Users/{uid}/FavoriteItems/{id}` | 旧客户端兼容 | 必须保留 |
| POST/DELETE | `/UserPlayedItems/{id}` | 播放历史事件/删除历史 | 可以实现后闭环测试 |
| POST | `/Users/{uid}/PlayedItems/{id}` | 旧客户端兼容 | 必须保留 |
| Rating 接口 | 飞牛没有等价评分模型 | 明确不支持，不返回虚假成功 |

### 阶段 F：播放列表

使用唯一临时名称 `Yinqiao Audit <run-id>`，无论测试成功或失败都在 finally 清理。

| Method | Path | 计划 |
| --- | --- | --- |
| GET/POST | `/Playlists` | 列表和创建 |
| GET/POST | `/Playlists/{id}` | 详情和重命名 |
| GET/POST/DELETE | `/Playlists/{id}/Items` | 读取、添加、移除 |
| POST | `/Playlists/{id}/Items/{itemId}/Move/{index}` | 底层能可靠重排才实现 |
| DELETE | `/Items/{playlistId}` | 旧客户端删除歌单兼容 |
| Users 共享接口 | 飞牛歌单无 Jellyfin 协作权限模型 | 明确不支持 |

### 阶段 G：会话和后台任务

| Method | Path | 计划 |
| --- | --- | --- |
| POST | `/Sessions/Capabilities`, `/Sessions/Capabilities/Full` | 接受客户端能力并返回 204 |
| POST | `/Sessions/Playing*`, `/PlayingItems/*` | 播放开始、进度、停止事件 |
| POST | `/Sessions/Logout` | 删除当前协议令牌 |
| GET | `/ScheduledTasks`, `/ScheduledTasks/{id}` | 读取音乐扫描状态 |
| POST/DELETE | `/ScheduledTasks/Running/{id}` | 仅能映射扫描任务时实现 |
| POST | `/Library/Refresh` | 仅管理员真实触发扫描；单独显式测试 |
| 远程会话控制 | 不应伪装 NAS 本地播放器 | 明确不支持 |

## 4. 自动化与真实测试产物

新增独立 Jellyfin 审计客户端，输出：

```text
autotest/reports/<run-id>-jellyfin/
  report.json
  report.md
  responses/
```

每条记录包含：

- operationId、method、path 和测试参数；
- HTTP 状态、响应时间、响应大小；
- 结构校验、动态 ID、分页总数；
- 是否执行了写操作及清理结果；
- 对应脱敏响应证据；
- `现网已通过/失败/跳过/明确不支持` 状态。

测试顺序：只读全量 -> 可恢复写操作 -> 管理员动作。管理员扫描不会默认执行。

## 5. 发布闸门

1. 不在完成接口审计前发布“完整 Jellyfin 兼容”表述；
2. 每个版本只处理一组可验证问题，NAS 升级后立即跑真实 HTTPS 回归；
3. 箭头音乐完成登录、首页、歌曲、专辑、歌手、搜索、歌单、封面、歌词和播放流程；
4. 日志中无未识别的箭头音乐路径、全库误扫描、上游 4xx/5xx 或未清理临时数据；
5. 最终支持矩阵对每个未支持能力写明原因，不以空数组冒充真实能力。
