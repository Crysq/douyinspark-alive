# 架构与关键设计

## 模块

```
server/
├── main.go                     装配：config → store → cache → signer → bus → sender → scheduler → router
└── internal/
    ├── config/                 环境变量 + .env
    ├── model/                  Admin/Account/Friend/Task/Template/Record/Setting/AuditLog/Notification
    ├── store/                  MySQL 连接（启动重试）、迁移、种子数据
    ├── cache/                  Redis 封装
    ├── signer/                 签名服务客户端 + 私信/会话接口
    ├── service/
    │   ├── spark.go            状态机：24h 窗口、四级分级、天数推进、模板渲染
    │   ├── sender.go           一次真实下发：签名 → 落库 → 事件 → 缓存失效
    │   ├── scheduler.go        20s 心跳、Redis 锁、窗口/间隔/静默/幂等/重试、断火检测
    │   └── events.go           EventBus：本地扇出 + Redis Pub/Sub
    ├── handler/                HTTP 层（auth/dashboard/account/friend/task/template/record/setting/system/event）
    ├── middleware/             JWT(+黑名单)、限流、CORS、慢请求日志
    └── router/                 路由 + SPA 静态托管
```

## Redis 键位

| 键 | 用途 |
| --- | --- |
| `doyinspark:overview` | 看板聚合缓存（20s，发送后失效） |
| `doyinspark:lock:scheduler` | 调度器分布式锁（45s TTL，Lua 校验释放） |
| `doyinspark:once:<friendID>:<yyyymmdd>` | 好友×日期幂等标记（40h TTL） |
| `doyinspark:break:<friendID>:<yyyymmdd>` | 断火通知去重（26h TTL） |
| `doyinspark:rl:<scope>:<ip>` | 登录/扫码/续风/任务执行的固定窗口限流 |
| `doyinspark:jwt:black:<jti>` | 登出即时失效 |
| `doyinspark:events` | 跨实例事件广播（SSE 订阅转发） |

## 私信发送链路

1. `Sender.SendForFriend`：校验账号在线 → 选模板 → 渲染占位符
2. `signer.SendIM`：构造 `msg_type=7` 表单 → POST `sign-service /fetch`
3. sign-service 在该账号的常驻 Chromium 页面里用 `byted_acrawler.frontierSign` 签名
   并以页面自身 Cookie 发起 `fetch`（`credentials: include`）
4. 返回 `status_code != 0` 视为失败；失败按设置重试，原因原文落库
5. 成功后推进连续天数、模板使用计数，广播 SSE 事件，失效看板缓存

## 火花状态机

```
评估(lastInteractAt):
  nil                     → urgent(待互动)  progress=1
  剩余 = 24h - elapsed
  剩余 <= 0               → broken
  剩余 < 3h               → urgent
  剩余 < 8h               → warn
  否则                    → safe

推进(streak):
  同日重复互动            → 不变
  elapsed <= 24h          → +1
  elapsed >  24h          → 归 1
```

## 多租户与会员（v2）

### 数据模型

```
users(id, username, password_hash, role[admin|user], status[active|pending|banned],
      plan, member_expire_at, last_login_at)
card_keys(id, code, plan, days, status[unused|used|void], batch, note,
          created_by, used_by, used_at, expire_at)
membership_logs(id, user_id, action[register|redeem|grant], plan, days, card_code,
                before_expire, after_expire, remark)
```

所有租户表（accounts / friends / tasks / templates / records / settings / notifications / audit_logs）
都带 `user_id`，查询统一在 SQL 层加条件；越权访问返回 404/403。

### 卡密生命周期

```
生成（管理员） → unused → 兑换/注册 → used（记录 used_by、used_at）
                    └→ 作废 → void
```

- 卡密格式 `DYSP-XXXX-XXXX-XXXX`，字符集去掉 I/O/0/1，Base32 随机 12 位
- 卡密自身有有效期 `expire_at`，与会员时长无关
- 兑换在同一个事务里：校验 → 更新用户到期 → 标记卡密 → 写流水
- 默认 6 种卡：day/week/month/quarter/year/forever（永久卡到期时间 = 100 年后）

### 登录与激活令牌

```
登录 → 密码正确 → 会员有效？
        ├─ 是 → 完整令牌（72h，scope=full）
        └─ 否 → 待激活令牌（2h，scope=activate）+ 403 needActivation
                  └─ 只能访问 POST /membership/redeem 与 GET /membership
                  └─ 兑换成功 → 换发完整令牌
```

### 旧数据迁移

启动时若存在无归属（`user_id` 不在 users 表）的数据，会自动认领到管理员：
旧版 `friends.user_id` 里存的是好友抖音 ID，先搬到 `friend_user_id` 再改租户列。
系统模板库固定 `user_id=0`，不参与认领。
