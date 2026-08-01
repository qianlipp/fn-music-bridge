# 音桥 1.0.4

1.0.4 解决 Jellyfin 客户端在音桥重启或升级后继续使用旧 Token，导致曲库、
搜索、随机和收藏全部返回 HTTP 401 的问题，并修正箭头音乐首页查询的数据映射。

## 主要变更

- Jellyfin Token 到绑定用户的映射使用 AES-256-GCM 加密保存在音桥私有数据目录。
- 服务重启时不保存或复用飞牛上游 Token，而是用本机加密凭据重新登录后恢复客户端会话。
- Jellyfin 退出登录和管理页删除绑定账号时，持久 Token 同步撤销。
- `Filters=IsFavorite` 读取真实收藏曲目，收藏专辑和歌手从收藏歌曲中去重派生。
- `SortBy=DatePlayed` 读取播放历史，并返回 `PlayCount`、`Played` 和 `LastPlayedDate`。
- `SortBy=Random` 使用真实曲库总数选择随机页段，再对返回歌曲打乱。
- `SearchTerm` 继续使用飞牛音乐本地搜索索引，保留服务端分页和结果过滤。

## 升级提示

1.0.3 及更旧版本从未持久化 Jellyfin Token，因此升级到 1.0.4 后需要在箭头音乐中
删除并重新添加一次 Jellyfin 连接。从这次新登录开始，后续音桥重启将自动恢复会话。
