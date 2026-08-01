# Yinqiao API

这是独立 `yinqiao` FPK 的底层服务。它监听 NAS 的 `18090` 端口，并通过
官方 `/var/run/trim_music.socket` 转发到原飞牛音乐服务。密码登录会优先经本机
fnOS Web 网关转发，以保留官方浏览器登录链路中的网关请求环境。

```text
LAN client
  -> http://NAS_IP:18090/yinqiao/v1
  -> yinqiao-api
  -> fnOS localhost Web gateway (password login)
     or /var/run/trim_music.socket (all other calls)
  -> official trim-music
```

包内运行参数：

```text
YINQIAO_API_LISTEN=0.0.0.0:18090
YINQIAO_API_PREFIX=/yinqiao/v1
YINQIAO_API_ALLOWED_ORIGINS=*
YINQIAO_API_PROXY_MUSIC_ROOT=false
YINQIAO_UPSTREAM_ORIGIN=http://unix
YINQIAO_UPSTREAM_UNIX_SOCKET=/var/run/trim_music.socket
YINQIAO_UPSTREAM_API_BASE=/music/api/v1
YINQIAO_UPSTREAM_GATEWAYS=http://127.0.0.1:5666,http://127.0.0.1:8000
```

没有设置 `YINQIAO_API_KEY`，因此 API 入口没有额外鉴权。原音乐业务
Token 和角色权限仍由官方服务校验。

除分类后的稳定 API 外，`/yinqiao/v1/upstream/<path>` 可以签名并透传
任意官方 API v1 路径，用于尚未命名或未来新增的官方接口。透传仍受官方 Token、
用户角色和曲库权限约束。

本地验证：

```bash
go test -race ./...
go vet ./...
```

客户端协议见 [API.md](./API.md)。
