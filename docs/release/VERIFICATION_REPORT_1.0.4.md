# 1.0.4 发布候选验证报告

验证日期：2026-08-01

## 自动验证

- 登录签发 Jellyfin Token 后销毁第一个服务实例，用相同私有数据目录启动第二个实例，原 Token 可读取曲库。
- 持久会话文件不包含明文 Token 或用户名。
- 调用 `/Sessions/Logout` 后再次重启服务，原 Token 返回 HTTP 401。
- 箭头音乐实际参数的收藏歌曲、收藏专辑、最近播放、随机歌曲和搜索回归测试通过。
- 损坏 JSON 仍被拒绝，空 JSON GET 仍按无参数请求处理。
- Service `go test -race ./...` 和 `go vet ./...` 通过。
- API `go test ./...` 和 `go vet ./...` 通过。
- 管理页协议顺序已统一为 OpenSubsonic、Jellyfin Audio、Audio Station、Ampache；前两项显示“优先推荐”。
- 管理页 1200x800 静态渲染检查通过，推荐标记和“三方应用支持”标题无截断或重叠。
- “关于与安全”及 FPK 应用说明明确普通应用身份、本地数据边界、无破解、无鉴权绕过和无系统改动。
- FPK 包结构、普通运行账号、官方依赖、桌面入口和内层校验和通过。

## 构建产物

- 文件：`dist/yinqiao-1.0.4.fpk`
- 大小：约 6.3 MB
- SHA-256：`d769961581d565fd4c2994b5aad14dabdcd8761f976030e3eaea06e0013db0a7`
- `./scripts/verify-package.sh dist/yinqiao-1.0.4.fpk` 通过。

## 真机待测

安装 1.0.4 后需在箭头音乐重新添加一次连接，然后验证曲库、搜索、随机、最近和
收藏页面。自动测试通过不等于箭头音乐真机已闭环。
