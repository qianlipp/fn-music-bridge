# 1.0.2 发布候选验证报告

验证日期：2026-08-01

## 已通过

- Yinqiao API：`go test ./...`
- Yinqiao Service：`go test ./...`
- Jellyfin 代码回归：大小写命名空间、跨页窗口、保留路由、Range 媒体、用户数据和明确不支持语义通过。
- Jellyfin 候选真机：连接真实曲库执行含可回滚写操作的黑盒验收，81 项通过、0 项失败、5 项明确不支持。
- Jellyfin 候选真机数据：57,795 首歌曲、10,268 张专辑、4,381 位歌手；跨页、父级子项和 4 KiB Range 断言通过。
- Jellyfin 写入闭环：临时歌单创建、改名、加歌、移歌、删除通过；收藏恢复原状态；注销后旧 Token 返回 401。
- Jellyfin 自动测试：`python3 -m py_compile autotest/jellyfin_autotest.py` 通过，报告响应会自动隐藏 Token 和密码。
- Shell：全部 `package-src/cmd/*` 与 `scripts/*.sh` 通过 `bash -n`
- JSON：privilege、resource、desktop config、安装/卸载 wizard、FnDepot 模板可解析
- JavaScript：管理页 `app.js` 通过 `node --check`
- FPK：manifest、依赖、平台、第三方来源、生命周期控制、图标、desktop entry 完整
- FPK：不包含官方音乐二进制、前端、数据库或私有数据
- FPK：内层 `app.tgz` MD5 与 manifest 一致
- FPK：外层 SHA-256 校验通过
- 随包文档：隐私、安全、免责声明和第三方标识说明已进入 `app/docs`
- 安全扫描：未发现私钥、凭据文件、数据库、`docker.sock` 或官方应用目录写入

## 产物

```text
dist/yinqiao-1.0.2.fpk
SHA-256 61a69004ffef8de825c15b5807edcdd0bf92e9c09000835e394d38b469878dbc
```

`./scripts/verify-package.sh dist/yinqiao-1.0.2.fpk 1.0.2` 已通过。包内只包含
`yinqiao-api`、`yinqiao-service`、管理页资源和必要的隐私/安全文档。

## 发布前必须真机通过

1. fnOS 能正确解析 `platform=x86` 和 `install_dep_apps=trim.music`。
2. `yinqiao` 普通应用账号可连接 `/var/run/trim_music.socket`，且日志中没有权限错误。
3. 全新安装、启动、停止和重启均正常；appname 已更改，不作为旧包的覆盖升级测试。
4. 卸载保留数据后重装可恢复；卸载删除数据后凭据、密钥、设置和日志消失。
5. 绑定账号、自动 IP、自定义端口、协议开关和五种接口入口正常。
6. 通过正式 HTTPS 地址执行 `autotest/run-jellyfin.sh --mutations`，不得用候选临时端口报告代替。
7. 用箭头音乐完成登录、首页、分页、搜索、封面、歌词、歌单、拖动和连续播放。
8. 至少再选择一款 Jellyfin 客户端完成相同核心流程，记录客户端版本和请求日志。

## 尚未完成

正式 FPK 尚未安装到 NAS，公开 HTTPS 在候选测试时仍是旧服务，因此不能声称 1.0.2
已经线上通过。逐接口状态见 `docs/JELLYFIN_10_10_7_MATRIX.md`，官方 103 个音乐相关
Operation 的结论见 `docs/JELLYFIN_10_10_7_OPERATIONS.md`。

本机静态预览可访问，HTML/CSS/JavaScript 资源返回正常；但本次没有生成新的桌面/小窗口
截图。视觉截图验收必须在公开发布前手工完成，不能把静态结构检查等同于真实渲染通过。
