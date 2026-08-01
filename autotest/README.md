# 协议自动化验收客户端

## Jellyfin Audio

`jellyfin_autotest.py` 以 Jellyfin `10.10.7` 音乐客户端接口为基线，逐项验证服务发现、
登录、用户视图、真实分页和总数、搜索、动态详情、封面、歌词、四种音频入口、会话、任务、
歌单和收藏。每个请求保存脱敏响应证据；音频只读取前 4 KiB。

只读测试：

```bash
JELLYFIN_URL='https://music.example.com' \
JELLYFIN_USER='test-user' \
JELLYFIN_PASSWORD='test-password' \
./run-jellyfin.sh
```

可回滚写入测试：

```bash
./run-jellyfin.sh --mutations
```

写入轮次会创建唯一临时歌单，完成改名、加歌、移歌后在 `finally` 删除；歌曲收藏会先读取
原状态，切换后恢复。`UserPlayedItems` 会留下测试账号的播放历史，这是该轮唯一不可回滚的
业务记录。脚本最后注销协议会话并验证旧 Token 返回 401。

报告目录为 `reports/<run-id>-jellyfin/`。HTTP 200 不是通过的充分条件：脚本还校验跨页条数、
真实总数、动态 ID、父级子项、Range 长度、`Content-Range`、写入后读取和恢复结果。

## OpenSubsonic

这套脚本针对音桥实际公开的 OpenSubsonic/Subsonic 1.16.1
接口做黑盒测试。它不依赖第三方 Python 包，支持 JSON、XML、JSONP、GET、表单 POST、
三种鉴权方式、音频 Range、封面、动态曲库 ID、兼容占位接口和可逆写操作。

## 运行

只读和协议兼容测试：

```bash
cd yinqiao/autotest
SUBSONIC_URL='https://fnmadapter.example.com' \
SUBSONIC_USER='test-user' \
SUBSONIC_PASSWORD='test-password' \
./run.sh
```

加入歌单创建/更新/删除、临时收藏/取消收藏、scrobble 播放上报：

```bash
SUBSONIC_URL='https://fnmadapter.example.com' \
SUBSONIC_USER='test-user' \
SUBSONIC_PASSWORD='test-password' \
./run.sh --mutations
```

`startScan` 会触发真实全库扫描，仅管理员且明确需要时启用：

```bash
./run.sh --mutations --admin-actions
```

常用参数：

```text
--timeout 30          单个请求超时秒数
--output DIR          报告根目录，默认 autotest/reports
--keep-playlist       保留测试创建的临时歌单
--verbose             实时打印每个测试的结果和错误
```

## 产物

每次运行生成独立目录：

```text
reports/YYYYMMDD-HHMMSS-xxxxxxxx/
  report.md            人工排查报告
  report.json          完整结构化结果
  responses/           每次 HTTP 请求的响应证据
```

`report.md` 会给出通过、失败、跳过数量，按接口列出覆盖状态，并保留 HTTP 状态、
Subsonic 错误码、响应摘要、请求耗时和证据文件路径。媒体响应只保存前 64 KiB，
音频测试发送 `Range: bytes=0-4095`，不会下载整首歌曲。

测试过程可能产生的状态变化：

- `--mutations` 创建唯一命名的临时歌单，完成后在 `finally` 清理；
- 只挑选当前未收藏曲目测试 `star`，随后调用 `unstar` 恢复；
- `scrobble` 会留下真实播放历史，这是唯一默认不可逆的测试写入；
- `setRating`、播放队列和书签在当前网关中是兼容占位实现，只验证协议响应；
- `startScan` 只有 `--admin-actions` 才会调用。

后端日志排查时，请用报告顶部的 Run ID、开始/结束时间和客户端名
`yinqiao-autotest-<Run ID>` 对齐服务日志。
