# 1.0.3 发布候选验证报告

验证日期：2026-08-01

## 问题复现

- 普通 `GET /System/Info/Public`：HTTP 200。
- 同一 GET 附加 `Content-Type: application/json` 且 Body 为空：1.0.2 返回 HTTP 400。
- 箭头音乐日志显示登录 HTTP 200，但 Public Info、Views 和 Items 均因此返回 HTTP 400。

## 修复验证

- 箭头音乐式空 JSON GET 的 Public Info、Views 和 Items 回归测试通过。
- 真正损坏的 JSON POST 仍返回 HTTP 400。
- Yinqiao API：`go test ./...` 通过。
- Yinqiao Service：`go test ./...` 通过。
- Python 自动测试脚本编译与 Shell 语法检查通过。
- FPK 包结构、普通运行权限、官方依赖和内层校验和通过。

## 构建产物

- 文件：`dist/yinqiao-1.0.3.fpk`
- 大小：约 6.3 MB
- SHA-256：`2b566e944b987c1d21c835d2508fb516ef7e1f7baf49a975dbede6835c75fe81`
- `./scripts/verify-package.sh dist/yinqiao-1.0.3.fpk 1.0.3` 通过。

## 真机待测

安装 `yinqiao-1.0.3.fpk` 后，在箭头音乐删除旧连接并用服务器根地址重新添加。应完成
服务发现，显示“飞牛音乐”曲库，并能读取真实歌曲列表。该步骤完成前不宣称箭头音乐
真机问题已经闭环。
