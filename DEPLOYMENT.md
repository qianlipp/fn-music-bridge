# 独立 FPK 部署

## 安装

如果 NAS 当前安装的是之前生成的 `trim.music-api-0.9.20.fpk`，它已经在官方应用
进程内启动了 18090 和 14040。请先从应用商店重新安装未修改的官方飞牛音乐包并
确认音乐数据正常，再安装本独立 FPK。不要让两个 Yinqiao API 同时占用相同端口。

推荐迁移顺序：

1. 停止当前“音乐”应用；
2. 使用应用商店的官方飞牛音乐包覆盖安装，保留应用数据；
3. 启动官方“音乐”并确认 `/music` 登录、曲库和设置正常；
4. 安装并启动 `yinqiao-1.0.5.fpk`；
5. 检查 18090 和 14040，再重新登记协议凭据。

在 fnOS 应用中心手动安装：

```text
dist/yinqiao-1.0.5.fpk
```

应用名是 `yinqiao`，不会覆盖 `trim.music`。建议先启动官方“音乐”，再启动
“音桥”；反过来启动也不会损坏数据，Yinqiao API 会在官方服务可用后自动恢复。

## 桌面窗口与账号绑定

应用中心安装完成后，“音桥”卡片会显示“打开”。点击后将在飞牛桌面窗口中
打开管理页，展示 Yinqiao API、OpenSubsonic、Audio Station、Jellyfin Audio 和 Ampache
状态及播放器连接地址。

首次使用时：

1. 在官方飞牛音乐的账号管理中创建独立账号，并授予需要访问的曲库权限；
2. 在协议桥桌面窗口输入该飞牛音乐账号和密码；
3. 点击“验证并绑定”。验证成功后，凭据使用 AES-256-GCM 加密保存在本应用数据目录；可继续新增多个账号，并在账号列表中独立删除；
4. 在播放器中选择 Subsonic、Audio Station、Jellyfin 或 Ampache，填写 NAS 地址、
   端口 `14040` 和同一账号密码。Audio Station 与 Jellyfin 可直接登录；Subsonic
   Token 模式和 Ampache 摘要模式使用已登记的加密凭据。

也可以直接访问 `http://NAS_IP:14040/yinqiao/ui/`。管理窗口的“系统设置”
可以修改播放器客户端端口、覆盖自动识别的局域网主机，并分别开启或关闭四种协议。
`14040` 始终保留为管理端口，默认客户端端口也是 `14040`。管理窗口和原有
`/rest`、Ampache、Yinqiao API 路径相互独立，不改变客户端兼容性。

## 检查

```bash
curl http://NAS_IP:18090/yinqiao/v1/health
curl http://NAS_IP:18090/yinqiao/v1/ready
curl http://NAS_IP:18090/yinqiao/v1/capabilities
curl http://NAS_IP:14040/yinqiao/status
curl http://NAS_IP:14040/yinqiao/capabilities
```

`health` 只表示 Yinqiao API 进程存活；`ready` 表示官方飞牛音乐上游可以正常调用。

日志位于独立应用数据目录：

```text
/var/apps/yinqiao/var/log/yinqiao-api.log
/var/apps/yinqiao/var/log/yinqiao-service.log
```

运行中不要直接删除日志文件。Linux 进程会继续写入已删除的旧 inode，同名路径不会自动
重建；误删后请在应用中心停止并重新启动“音桥”。需要清空日志时使用：

```bash
sudo truncate -s 0 /var/apps/yinqiao/var/log/yinqiao-api.log
sudo truncate -s 0 /var/apps/yinqiao/var/log/yinqiao-service.log
```

持续查看日志建议使用 `tail -F`，它能在升级或重启创建新文件后自动重新跟随：

```bash
sudo tail -F /var/apps/yinqiao/var/log/yinqiao-service.log
```

## 通用上游透传

稳定接口之外，可用以下形式访问官方 API：

```text
GET|POST|PUT|PATCH|DELETE
/yinqiao/v1/upstream/<official-api-v1-relative-path>
```

例如官方路径 `/music/api/v1/sys/info` 对应：

```bash
curl http://NAS_IP:18090/yinqiao/v1/upstream/sys/info \
  -H 'Authorization: Bearer FNMUSIC_TOKEN'
```

透传接口保留官方响应格式，稳定接口则返回统一的 `{ok,data,error,meta}` 格式。

## 卸载

卸载本 FPK 不会停止、卸载或修改官方飞牛音乐。卸载向导只处理
`/var/apps/yinqiao/var` 下本应用自己的加密凭据、设置和日志。
