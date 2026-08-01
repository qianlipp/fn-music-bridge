# 1.0.5 发布候选验证报告

验证日期：2026-08-01

## 自动验证

- API `go test ./...` 和 `go vet ./...` 通过。
- Service `go test -race ./...` 和 `go vet ./...` 通过。
- API、协议服务端响应、管理页和 FPK manifest 版本统一为 `1.0.5`。
- 管理页协议顺序为 OpenSubsonic、Jellyfin Audio、Audio Station、Ampache，前两项显示“优先推荐”。
- 管理页 1200x800 静态渲染检查通过，推荐标记和“三方应用支持”无截断或重叠。
- 应用说明包含普通应用身份、本地数据边界、无破解、无鉴权绕过和无系统改动声明。
- FPK 包结构、普通运行账号、官方依赖、桌面入口和内层校验和通过。
- 解包确认 manifest 为 `version="1.0.5"`，可从 `1.0.4` 执行版本升级。

## 构建产物

- 文件：`dist/yinqiao-1.0.5.fpk`
- 大小：约 6.3 MB
- SHA-256：`ab28de4054c702c3971000c0034b345bb1cf269b9193639f3b009698620343a5`
- `./scripts/verify-package.sh dist/yinqiao-1.0.5.fpk 1.0.5` 通过。

## 真机待测

在 fnOS 应用中心上传 1.0.5 覆盖安装 1.0.4，确认升级过程保留绑定账号、协议开关、
自定义端口和 Jellyfin 会话。安装完成后刷新管理页，版本应显示 `v1.0.5`。
