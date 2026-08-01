# 飞牛音乐 0.9.11 FPK 解包与架构分析

分析日期：2026-07-30

## 1. 结论摘要

这个文件不是加密或私有二进制容器。`trim.music-0.9.11.fpk` 是一层 gzip 压缩的 tar，应用正文 `app.tgz` 又是一层 gzip 压缩的 tar。

包内是一个典型的 fnOS 原生应用：

- 外层 `manifest`、`config/`、`cmd/`、`wizard/` 和 `ui/` 由应用中心读取。
- `app.tgz` 内含一个 x86-64 Linux Go 服务、SQLite 建表脚本、桌面入口和已经构建好的 Web 前端。
- Go 服务同时负责 API、鉴权、音乐库扫描、元数据刮削、全文检索、播放/转码代理和静态文件服务。
- Web 前端是 React 单页应用，API 基址为 `/music/api/v1`。
- 当前业务 UI 明确写死 `min-w-[1120px]`。侧栏在 1100px 以下只会收起，不会切换为手机导航。这是 PC-only 的直接原因。
- 登录、欢迎和初始化页面没有该最小宽度限制，说明项目内部已有局部移动适配，但主业务壳被主动限制为桌面版。
- 包中没有 TypeScript/React 源码和 source map，只有压缩后的 Vite bundle；Go 二进制也使用 `-s -w` 去除了调试符号。因此第二阶段不应直接维护压缩 bundle，而应新建 H5 前端并复用现有后端协议。

## 2. 解包结果

原始包：

```text
文件        trim.music-0.9.11.fpk
类型        gzip compressed data
SHA-256     e1ff7946870353cf903443ce3f0d428ea2aa6669a9491208a6f46d0ec645d1f1
压缩大小    18,326,311 bytes
解压 tar    18,462,720 bytes
```

解包目录：

```text
unpacked/trim.music-0.9.11/
├── package/               # FPK 外层
│   ├── manifest
│   ├── ICON.PNG
│   ├── ICON_256.PNG
│   ├── app.tgz
│   ├── cmd/
│   ├── config/
│   ├── i18n/
│   └── wizard/
└── app/                   # app.tgz 内容
    ├── trim-music         # Go 后端
    ├── sqlite.sql
    ├── sqlite_v2.sql
    ├── static/            # Web 前端
    ├── ui/                # fnOS 桌面入口
    └── syslog/
```

共解出 237 个文件，解包后约 63 MB。原始 FPK 未被修改。

## 3. fnOS FPK 格式

从公开的 fnpack 模板、第三方构建仓库和这个官方包可以相互验证以下约定：

| 文件 | 作用 |
| --- | --- |
| `manifest` | 应用名、版本、架构、最低系统版本、安装类型和桌面入口 |
| `ICON.PNG` / `ICON_256.PNG` | 应用中心图标 |
| `app.tgz` | 真正安装到应用目录的程序和资源 |
| `cmd/main` | 应用中心调用的 `start/stop/status` 入口 |
| `cmd/install_*` | 安装前后生命周期 |
| `cmd/upgrade_*` | 升级前后生命周期 |
| `cmd/uninstall_*` | 卸载前后生命周期 |
| `config/privilege` | 运行用户和组权限 |
| `config/resource` | systemd 或 Docker 等资源声明 |
| `i18n/*` | 应用中心名称、描述、维护者等本地化字段 |
| `wizard/*` | 安装/卸载向导表单 |
| `app/ui/config` | fnOS 桌面图标和 URL 启动方式 |

外部资料：

- [fnOS 开发者门户](https://developer.fnnas.com/)提供 `fnpack` 工具。
- [公开 FPK 模板与构建说明](https://github.com/cropflre/nowen-note/blob/main/scripts/fpk/README.md)展示了 `manifest`、`config`、`cmd`、`app/ui` 和 `fnpack` 的组合方式。
- [fnOS Apps 构建仓库](https://github.com/conversun/fnos-apps)提供了大量实际 FPK 构建样本。

重新打包时应使用官方 `fnpack`，不要把 `tar -czf` 当成最终发行流程。tar/gzip 足以解包研究，但应用中心还会校验 manifest、目录、生命周期脚本及平台约束。

## 4. 当前包的 fnOS 配置

`manifest` 的核心值：

```ini
version="0.9.11"
appname="trim.music"
os_min_version="1.2.0300"
desktop_uidir="ui"
desktop_applaunchname="trim.music"
source="official"
install_type="root"
arch="x86_64"
native_app="true"
```

含义：

- 这是飞牛官方原生应用，不是 Docker 项目。
- 仅提供 x86-64 NAS 服务端二进制。H5 手机访问不会改变 NAS 端架构限制。
- 安装和运行权限是 root。
- 桌面入口来自 `app/ui/config`，打开同源地址 `/music?ver=0.0.1`。
- `checkport="false"`，应用没有对用户暴露独立端口；由系统内置 Web 路由转发 `/music`。

`app/ui/config` 使用 fnOS 接受的宽松 JSON，包含尾逗号，不应拿严格 JSON parser 直接重写。

## 5. 生命周期和启动链路

启动链路：

```text
fnOS 应用中心
  -> cmd/main start
  -> cmd/config 组装 TRIM_APPDEST/TRIM_PKGVAR 等路径
  -> 以 TRIM_PKGVAR 为工作目录
  -> 后台启动 TRIM_APPDEST/trim-music
  -> 写入 trim.music.pid
```

停止时先校验 PID 对应的 `/proc/<pid>/exe` 是否为目标二进制；PID 文件失效时再按完整二进制路径查找进程。卸载向导允许保留数据，选择删除时会清理 `${TRIM_PKGVAR}` 和 `/var/apps/trim.music/meta/project-logo`。

运行时状态、数据库、索引、日志、封面和歌词等都应位于 fnOS 分配的 `TRIM_PKGVAR`，升级 `app.tgz` 不应覆盖这些持久数据。

## 6. 后端架构

### 6.1 技术栈

`trim-music` 的 Go build info 可完整读取：

- Go 1.25.0，模块版本 `trim-music v0.9.11`
- 构建目标 `linux/amd64`，`CGO_ENABLED=1`
- Git revision `9297398b95e1f7d5cefc8c6e8646ace20a6473e9`
- 构建时间 `2026-07-28T06:05:51Z`
- Gin 1.10.1
- GORM 1.30.0 + SQLite driver
- Bleve 2.5.7 全文检索
- fsnotify 与飞牛 file-monitor client
- 飞牛 appcenter、trim-rpc、OAuth、media server 等内部客户端
- libzmq 运行时依赖

能还原出的内部包边界：

```text
core/web/controller       HTTP controller
core/service              业务服务
core/storage/dao          数据访问
core/storage/entity       数据实体
core/storage/manager      聚合写入/关系维护
core/service/account      账号、token、权限
core/service/appstate     初始化和服务器设置
core/service/scanner      扫描、刮削、歌词任务
core/service/search       Bleve 索引与搜索
core/service/filestore    封面和歌词分片存储
core/pkg/appcenter        授权目录与读写权限
core/pkg/fsevent          文件变更事件
core/pkg/scraper/local    本地 tag/CUE/封面解析
core/pkg/scraper/cloud    官方云元数据和歌词
core/pkg/client/*         OAuth、媒体服务、云刮削 RPC
```

### 6.2 音乐入库链路

```text
应用中心授权目录
  -> 建立共享音乐库
  -> 全量扫描或文件监控增量事件
  -> 识别音频/CUE
  -> ffprobe、tag、WAV/ID3、本地封面和同目录歌词提取
  -> 可选官方云元数据/歌词刮削
  -> 写 SQLite 关系模型
  -> 封面/歌词写文件存储
  -> 更新 Bleve 搜索索引
```

扫描任务分为文件扫描、云刮削、歌词下载三类队列。文件创建、修改、移动、删除会通过飞牛文件监控服务增量同步；删除采用逻辑失效标记，后续可恢复或清理。

本地解析支持 CUE 整轨切分、内嵌 tag、内嵌/旁挂封面和歌词；云端可补全 track/album/artist/lyric 元数据。

### 6.3 数据库

两个 schema 共定义 22 张表：

```text
app_state
user, user_oauth, user_token
shared_library, shared_library_member
audio_file
artist, album, track
album_artist, track_artist
genre, track_genre
lyric
favorite_track, play_history
user_track_lyric_preference
playlist, playlist_track
user_track_lyric_offset
user_sorting_preference
```

`sqlite_v2.sql` 是升级迁移：增加共享库访问模式、歌单、歌词偏移、排序偏好，以及音频文件/曲目的删除状态。

### 6.4 HTTP API

统一基址：

```text
/music/api/v1
```

响应 envelope 为：

```json
{ "code": 0, "msg": "", "data": {} }
```

前端定义的主要 API：

| 域 | 接口 |
| --- | --- |
| 初始化/登录 | `/sys/config`, `/initialization/state`, `/initialization/prepare`, `/initialization/confirm`, `/user/auth-login`, `/user/password-login`, `/user/me`, `/user/logout` |
| 音乐 | `/track/list`, `/track/metadata`, `/album/list`, `/album/detail`, `/artist/list`, `/artist/detail`, `/genre/list`, `/genre/detail` |
| 关联详情 | `/track/album-detail/list`, `/track/artist-detail/list`, `/track/genre-detail/list`, `/track/playlist-detail/list`, `/album/artist-detail/list`, `/artist/list-all` |
| 播放 | `/track/stream`, `/track/transcode`, `/track/transcode/heartbeat`, `/track/transcode/quit`, `/track/hls/:guid/preset.m3u8` |
| 漫游播放 | `/track/roam-start`, `/track/roam-next`, `/track/roam-previous` |
| 搜索 | `/search/track`, `/search/album`, `/search/artist`, `/search/playlist`, `/search/suggest`, `/search/index/rebuild` |
| 个人数据 | `/favorite-track/*`, `/play-history/*`, `/playlist/*`, `/lyric/list`, `/event/report` |
| 管理 | `/shared-library/*`, `/app-center/authed-dir/*`, `/settings/server`, `/settings/user`, `/user/*`, `/task/*` |
| 静态资源 | `/static/cover`, `/static/cover/track`, `/static/cover/playlist` |

### 6.5 鉴权和请求签名

登录态使用 `music-token` Cookie：

```text
Path=/; SameSite=Strict
fetch credentials=include
```

每个 API 请求还带 `authx`。算法已经从前端 bundle 还原：

```text
nonce      = 6 位随机数
timestamp  = Date.now() 毫秒
payload    = GET 时为排序后的 query string；其他方法为 JSON body
payloadMD5 = MD5(decodeURIComponent(payload))  # GET
payloadMD5 = MD5(payload)                      # 非 GET

source = "NDzZTVxnRKP8Z0jXg1VAMonaG8akvh"
       + "_" + pathname
       + "_" + nonce
       + "_" + timestamp
       + "_" + payloadMD5
       + "_" + clientKey

authx = "nonce=<nonce>&timestamp=<timestamp>&sign=" + MD5(source)
```

`clientKey` 以 XOR 数组形式放在公开前端 bundle 中，可以在第二阶段兼容实现。签名是协议兼容机制，不是可以保密的服务端密钥。

### 6.6 播放链路

前端播放引擎会按格式和浏览器能力选择：

1. 浏览器原生 `<audio>` + Range 请求。
2. 后端媒体服务转码并通过 HLS 播放，带心跳和退出接口。
3. 浏览器端 FFmpeg WASM 流式/整文件解码。

原生格式表包含 MP3/AAC/WAV/M4A/OGG；FLAC/APE/WavPack 等可走 WASM 预取；DSF/DFF/TAK/TTA 等走完整下载解码。H5 优先使用原生和服务端 HLS，尤其要避开 iOS 上 SharedArrayBuffer/WASM 多线程限制。

## 7. 前端架构

### 7.1 技术组成

从构建产物可确认或高置信识别：

- React DOM 19.2.7
- Vite 动态 chunk 构建
- TanStack Router
- TanStack Query
- Semi Design 组件
- Tailwind 风格 utility CSS
- Framer Motion
- Tolgee i18n
- HLS.js
- music-metadata 浏览器解析
- Three.js/WebGL 登录背景
- FFmpeg WebAssembly

前端共 55 个 JS、3 个 CSS、102 个 PNG 和 31 个 SVG。主 JS 约 1.65 MB，最大 CSS 约 870 KB。

### 7.2 页面路由

```text
/                  首页
/albums            专辑
/artists           艺术家
/genres            风格
/library           音乐库/文件夹
/favorites         喜欢
/recent            最近播放
/playlists/:id     歌单
/search            搜索
/settings          设置和管理
/welcome           首次欢迎
/init              初始化
/login             登录
/oauth/result      NAS OAuth 回调
```

业务页按路由动态加载 chunk。全局播放器、播放队列、顶部栏和左侧栏常驻在主 bundle。

### 7.3 PWA 状态

包中有 `manifest.webmanifest`，声明 `start_url=/music/`、standalone 模式和 192/512/1024 图标；HTML 也包含 viewport、theme-color 和 Apple Web App meta。

没有找到实际 service worker 文件或应用级 `navigator.serviceWorker.register()`。因此目前更准确地说是“具备可安装 Web App 外壳”，不是带离线缓存策略的完整 PWA。

### 7.4 PC-only 的直接证据

主业务壳：

```jsx
className="relative flex h-dvh w-screen flex-col overflow-clip min-w-[1120px]"
```

侧栏配置：

```js
collapsedWidth: 64
expandedWidth: 229
autoCollapseWidth: 1100
```

因此手机视口仍会渲染至少 1120px 宽的桌面画布，仅把侧栏缩成 64px，并产生横向裁切/缩放。现有断点主要用于组件尺寸和登录页，并没有手机业务壳、底部导航或手机播放详情布局。

## 8. H5 改造建议

### 8.1 推荐边界

保留：

- FPK 外层生命周期和权限结构
- `trim-music` Go 后端
- SQLite schema 和持久目录
- `/music/api/v1` 协议
- 扫描、刮削、搜索、转码和封面服务

新建：

- 独立、可维护的 H5 前端源码工程
- 手机业务壳、底部导航、手机播放器和队列抽屉
- API client、`authx` signer、登录态 bootstrap
- 针对 iOS/Android 浏览器的播放策略和触摸交互

不要直接修改 1.65 MB 的压缩主 bundle。它没有 source map，任何字符串/AST patch 都容易在下一版失效。

### 8.2 保留桌面版的方式

建议保留现有所有 `/music/static/assets/*`，只替换 `static/index.html` 为一个很薄的启动器：

- 宽屏加载原官方桌面入口 JS/CSS。
- 手机视口加载新建的 `/music/static/h5/*`。
- 两者仍使用同一个 origin、Cookie、API 和后端。
- 允许通过 query 或 localStorage 强制选择桌面/H5，方便平板和调试。

这样无需移动原有 chunk；原 bundle 内大量 `/music/static/assets/*` 绝对路径仍然有效。

### 8.3 H5 第一版页面优先级

1. 登录/OAuth、初始化状态和用户 bootstrap。
2. 首页、音乐库、搜索、专辑/艺术家详情。
3. 底部迷你播放器、全屏播放、歌词、播放队列。
4. 喜欢、最近播放、歌单。
5. 用户侧设置。
6. 管理员的音乐库、用户和任务管理；复杂表格可暂时引导切桌面版。

### 8.4 播放策略

H5 第一版建议：

- MP3/AAC/M4A/OGG/WAV 优先浏览器原生播放。
- 不被浏览器稳定支持的格式优先请求后端 HLS 转码。
- FFmpeg WASM 作为后续增强，不作为手机首版的必选路径。
- 保留 Media Session、后台播放、锁屏元数据、耳机控制、Range seek。
- 针对 iPhone 刘海/底部手势区使用 safe-area，并在真机验证音频解锁和后台切换。

### 8.5 打包与测试风险

- 修改包不应继续宣称 `source="official"` 或保留官方维护者身份用于公开分发。
- 若沿用 `appname="trim.music"`，它是覆盖升级包，不能与官方版并存；测试前必须备份持久目录。
- 若更换 appname 做并行安装，后端硬编码的 `/music`、应用中心授权身份和 `/var/apps/trim.music` 路径可能冲突，需要额外验证，不能只改 manifest。
- 最稳的第一轮是在测试 NAS/虚拟机上以原 appname 覆盖验证，确认数据兼容后再决定是否做可并行的第三方 appname。
- 外层和 `app.tgz` 应由脚本可重复构建，并最终调用官方 `fnpack`。
- 回归矩阵至少覆盖桌面 Chrome/Edge、Android Chrome、iOS Safari、局域网 HTTP 和 FN Connect/HTTPS。

## 9. 第二阶段建议产物

```text
src-h5/                  # 新 H5 源码
packaging/               # 从官方包骨架生成派生 FPK
scripts/unpack-fpk.sh
scripts/build-h5.sh
scripts/build-fpk.sh
tests/                   # API contract + Playwright 响应式测试
dist/                    # 不提交的构建产物
```

开始第二阶段前，最有价值的外部条件是准备一台可回滚的 fnOS 测试环境和一个测试音乐库。静态分析已经足以开始搭 UI，但登录、授权目录、转码、Cookie 和 fnOS 路由必须在真实系统上完成最终验证。
