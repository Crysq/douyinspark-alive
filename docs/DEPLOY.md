# 部署说明（OrbStack + Tailscale + Cloudflare Tunnel）

## 拓扑

```
                 ┌─────────────── Cloudflare (lkke.me) ──────────────┐
公网用户 ──HTTPS──► spark.lkke.me / douyinspark.lkke.me               │
                 │      ↓ 专属隧道 doyinspark (07a346e4-…)             │
                 └──────┼─────────────────────────────────────────────┘
                        ▼ cloudflared（launchd 常驻，127.0.0.1:8790）
                 ┌──────┴──────┐
tailnet 用户 ────► 127.0.0.1:8790 ◄── Tailscale Serve :9443（仅 tailnet）
                 │  OrbStack   │
                 │  ├ app      │ 8787（容器内）
                 │  ├ mysql    │
                 │  ├ redis    │
                 │  └ sign     │ 9100（Chromium 签名）
                 └─────────────┘
```

## 地址

| 入口 | 地址 | 说明 |
| --- | --- | --- |
| 内网（推荐日常用） | https://mac.tail17f525.ts.net:9443 | Tailscale Serve，仅 tailnet 可见，~20ms |
| 公网 | https://spark.lkke.me | Cloudflare 隧道 |
| 公网别名 | https://douyinspark.lkke.me | 同一隧道 |
| 本机 | http://127.0.0.1:8790 | 仅回环绑定，局域网不可达 |

> 端口 8790 只绑 `127.0.0.1`：公网走隧道，内网走 Tailscale，避免局域网直连。
> Tailscale 用 9443 而不是 8790，是因为 `tailscale serve` 会占用 tailnet IP 的该端口，
> 与 Docker 的 `0.0.0.0:8790` 冲突导致容器起不来。

## 组件

| 组件 | 位置 | 管理方式 |
| --- | --- | --- |
| 应用栈 | `docker compose`（项目 `doyinspark-keeper`） | `restart: unless-stopped`，OrbStack 开机自启 |
| Cloudflare 隧道 | `~/.cloudflared/config-doyinspark.yml` | launchd `com.doyinspark.cloudflared` |
| iseat 老隧道 | `~/.cloudflared/config.yml`（远程托管） | launchd `com.iseat.cloudflared`，未改动 |
| Tailscale Serve | `tailscale serve` 配置 | `tailscale serve status` |

## 从零复现

```bash
# 1. 应用
cd doyinspark-keeper
cp .env.example .env   # 或手写，见下方变量
docker compose up -d --build

# 2. Tailscale（仅内网）
tailscale serve --bg --https=9443 http://127.0.0.1:8790

# 3. Cloudflare 隧道
cloudflared tunnel login                       # 已有 cert.pem 可跳过
cloudflared tunnel create doyinspark
# 写 ~/.cloudflared/config-doyinspark.yml（ingress: 域名 → http://127.0.0.1:8790）
cloudflared tunnel route dns --overwrite-dns doyinspark spark.lkke.me
cloudflared tunnel route dns --overwrite-dns doyinspark douyinspark.lkke.me
# launchd 常驻（参考 ~/Library/LaunchAgents/com.doyinspark.cloudflared.plist）
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.doyinspark.cloudflared.plist
```

## 环境变量（.env，已 gitignore）

```ini
MYSQL_PASSWORD=doyinspark
JWT_SECRET=<64 位随机>
APP_PORT=8790
ALLOW_ORIGIN=https://spark.lkke.me,https://douyinspark.lkke.me,https://mac.tail17f525.ts.net:9443
TRUSTED_PROXIES=127.0.0.1,::1,192.168.156.0/24
```

`TRUSTED_PROXIES` 很关键：cloudflared 落在 127.0.0.1、再经 Docker 网关转发，
应用需要信任这两层才能读到 `CF-Connecting-IP`（cloudflared 不会补 X-Forwarded-For），
否则所有公网用户共用一个限流桶。`192.168.156.0/24` 用
`docker network inspect doyinspark-keeper_default --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}'` 查。

## 日常运维

```bash
docker compose ps
docker compose logs -f app
docker compose up -d --build app        # 改完代码重新发布
tailscale serve status
launchctl kickstart -k gui/$(id -u)/com.doyinspark.cloudflared   # 改隧道配置后重启
tail -f ~/.cloudflared/doyinspark.err.log
```

## 安全

- 首次登录后立刻改 `admin` 密码（默认 `admin123` 在 README 里公开）
- 8790 只绑回环，公网只经 CF 隧道 + Tailscale
- 登录/注册/兑换/续火均有 Redis 限流，按真实客户端 IP 计数
- 需要更强的公网防护，可在 Cloudflare 控制台给 `spark.lkke.me` 加 Access（邮箱/OTP）或 WAF 规则

## 已知问题

| 现象 | 原因 | 处理 |
| --- | --- | --- |
| 公网首字节 2–20s、偶发超时 | 本机到 CF 边缘 RTT ~250ms（LAX），TLS 握手就要 2.5s；`lkke.me` 老站同样 | 换更近的出口/关掉 Clash 的分流规则，或日常改用 Tailscale |
| 隧道日志报 DNS precheck 失败 | MagicDNS 把解析指向 100.100.100.100 | 不影响运行；如需自动更新可临时关 Tailscale DNS |
| 改了隧道配置不生效 | iseat 那条隧道是「远程托管」，本地 ingress 会被覆盖 | 本项目用独立隧道 `doyinspark`，不走远程配置 |
