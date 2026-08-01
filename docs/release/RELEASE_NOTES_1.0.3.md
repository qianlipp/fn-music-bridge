# 音桥 1.0.3

1.0.3 修复箭头音乐连接 Jellyfin 后提示服务器失败、无法发现音乐库的问题。

## 根因

箭头音乐会为无请求体 GET 请求附加 `Content-Type: application/json`。旧版音桥尝试解析
空 Body 并返回 `400 Invalid request body`，导致登录虽然成功，服务发现、曲库视图和歌曲
列表仍全部失败。

## 修复

- 空 JSON Body 作为无参数请求继续处理。
- `/System/Info/Public`、`/Users/{id}/Views` 和 `/Users/{id}/Items` 增加箭头音乐请求头回归测试。
- 真正损坏的 JSON 仍返回 HTTP 400。
- 保留 1.0.2 的分页、搜索、封面、歌词、Range 播放、歌单和会话兼容修复。

## 安装后验证

删除箭头音乐中旧的 Jellyfin 连接，使用不带 `/Audio` 等 API 后缀的服务器根地址重新添加。
登录后应显示“飞牛音乐”曲库，并能读取歌曲、专辑、歌手和流派。
