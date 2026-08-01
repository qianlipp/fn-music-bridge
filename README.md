# 音桥

`yinqiao` 是独立安装的 fnOS FPK。它不包含、不覆盖、也不修改官方
`trim.music` FPK 的任何文件，只把运行中的官方飞牛音乐作为上游服务。本项目是社区
第三方独立应用，不是飞牛官方产品或官方增强版。

## 两层架构

```text
音乐客户端
  |-- Yinqiao API ----------------------- 18090
  |-- OpenSubsonic / Subsonic ---\
  |-- Synology Audio Station -----+
	|-- Jellyfin Audio -------------+------------- CLIENT_PORT (默认 14040)
  `-- Ampache XML / JSON ---------+
                                 |
                        Yinqiao Service
                                 |
                         Local Yinqiao API
                          |             |
                 fnOS Web gateway   Unix Socket
                          \             /
                         official trim.music
```

底层 Yinqiao API 提供 85 个经过分类和响应归一化的稳定接口，并提供通用签名透传，
用于访问尚未静态命名或未来新增的官方 `/music/api/v1` 接口。它只调用官方服务，
不会直接读取或修改飞牛音乐数据库。

上层音桥服务基于 Yinqiao API 工作，共享 fnmusic 用户、角色、曲库权限、收藏、歌单、
歌词、封面、音频流和一份 AES-256-GCM 加密凭据库。

## 服务地址

| 服务 | 地址 | 默认状态 |
| --- | --- | --- |
| Yinqiao API | `http://NAS_IP:18090/yinqiao/v1` | 开启 |
| OpenSubsonic/Subsonic | `http://NAS_IP:CLIENT_PORT/rest` | 开启 |
| Synology Audio Station | `http://NAS_IP:CLIENT_PORT/webapi` | 开启 |
| Jellyfin Audio | `http://NAS_IP:CLIENT_PORT` | 开启 |
| Ampache XML API | `http://NAS_IP:CLIENT_PORT/server/xml.server.php` | 开启 |
| Ampache JSON API | `http://NAS_IP:CLIENT_PORT/server/json.server.php` | 开启 |
| 桌面管理窗口 | `http://NAS_IP:14040/yinqiao/ui/` | 开启 |
| 协议状态 | `http://NAS_IP:14040/yinqiao/status` | 开启 |
| 凭据登记 | `http://NAS_IP:14040/yinqiao/enroll` | 开启 |
| 设置管理 | `http://NAS_IP:14040/yinqiao/settings` | 开启 |

`14040` 固定保留为 fnOS 桌面管理入口。管理页可以为播放器另设一个
`1024-65535` 范围内的客户端端口，并可分别关闭或开启四种协议；首次安装默认端口
仍为 `14040`，四种协议默认全部开启。管理页还支持同时绑定和独立删除多个账号。

OpenSubsonic 是对 Subsonic API 的开放扩展规范，本服务返回协议版本 `1.16.1`，
并声明 `songLyrics` 与 `formPost` 扩展。规范见
[OpenSubsonic API Reference](https://opensubsonic.netlify.app/docs/api-reference/)。

Ampache 官方协议使用 XML/JSON、握手摘要和会话令牌。本服务实现兼容性最广的
API 6 核心浏览与播放动作。协议依据见 [Ampache API](https://ampache.org/api/)。

## 依赖和安全边界

- NAS 上必须已经安装官方 `trim.music`，但两个 FPK 可以独立安装、升级和卸载。
- Yinqiao API 不提供额外 API Key，官方 fnmusic Token 和角色权限仍然生效。
- 18090、14040 和自定义客户端端口均为明文局域网服务，不应直接暴露到不可信网络。
- 官方私有 API 或签名规则升级时，只需升级本 FPK，客户端协议保持稳定。
- “内部能力”仅限官方进程通过 HTTP/Unix Socket 暴露的能力；无法安全调用官方
  二进制内部没有接口入口的 Go 函数。
- 以独立的 `yinqiao:yinqiao` 普通应用账号运行；官方 Socket 为 `0666`，连接不需要
  root。不修改官方包、数据库或用户共享目录，也不访问 `docker.sock`。
- 账号凭据使用 AES-256-GCM 保存在本应用私有目录；没有广告、遥测或开发者云服务。
- 浏览器跨站页面不能修改账号和设置，但局域网内原生程序仍可访问管理接口，因此 LAN
  本身必须可信。

完整边界见 [SECURITY.md](./SECURITY.md)、[PRIVACY.md](./PRIVACY.md) 和
[DISCLAIMER.md](./DISCLAIMER.md)。发布材料索引见 [docs/release](./docs/release/README.md)。

## 构建

```bash
./scripts/build.sh
./scripts/verify-package.sh
```

输出：

```text
dist/yinqiao-1.0.5.fpk
```

详见 [PROTOCOLS.md](./PROTOCOLS.md) 和 [DEPLOYMENT.md](./DEPLOYMENT.md)。

## 开源许可

本项目原创源码与文档采用 [GNU AGPL v3.0](./LICENSE) 许可。第三方协议名称、商标、
产品名称和播放器图标不属于项目原创授权范围，其权利归各自权利人所有；具体边界见
[THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)。官方飞牛音乐安装包、解包文件、
真实联调响应、账号凭据和构建产物均不包含在源码仓库中。
