# 抖音 Web 接口现状（2026-11 实测）

> 结论先行：**读接口（会话好友、用户资料、表情）已经跑通；发消息不再是普通 HTTP 表单接口**，
> 已迁到 `imapi.douyin.com` 的 protobuf 协议，旧的 `/aweme/v1/web/im/send/msg/` 返回
> `404 Unsupported path(Janus)`。

## 参数基线

所有 `www.douyin.com/aweme/...` 接口必须带当前 Web 版本号，旧的 19.5.0 会被网关拒绝
（返回 `{"status_code":0,"status_msg":"url doesn't match"}`）：

```
device_platform=webapp&aid=6383&channel=channel_pc_web&pc_client_type=1
&version_code=170400&version_name=17.4.0&cookie_enabled=true&platform=PC
&update_version_code=170400
```

## 可用接口

| 用途 | 接口 | 说明 |
| --- | --- | --- |
| 用户资料 | `POST /aweme/v1/web/im/user/info/` | 表单 `sec_user_ids=["MS4w..."]`，返回 nickname / uid / unique_id / avatar_thumb |
| 表情面板 | `GET /aweme/v1/web/emoji/list?need_all=true` | 返回 371 个表情：`display_name` + `emoji_url.url_list` |
| 热门表情包 | `GET /aweme/v1/web/im/resources/emoticon/trending` | 动态贴纸（webp） |
| 自定义贴纸 | `GET /aweme/v1/web/im/resource/list/aggregation/` | `scenes=CUSTOM_STICKER_PAGE` |
| 会话在线态 | `POST /aweme/v1/web/im/user/active/status/` | body 里带全部会话的 `sec_user_ids` |

## 已失效

| 接口 | 现状 |
| --- | --- |
| `/aweme/v1/web/im/conversation/list/` | `404 Unsupported path(Janus)` |
| `/aweme/v1/web/im/friend/list/` | 同上 |
| `/aweme/v1/web/im/send/msg/` | 同上（发送已迁走） |
| `/v1/im/send/` | 路径存在但返回 504/500，body 为 protobuf，不是表单 |

## 好友列表怎么拿

聊天页的会话数据走 **WebSocket**，没有可复用的 HTTP 列表接口。当前方案：

1. 打开 `https://www.douyin.com/chat`（独立标签页，共用同一浏览器 profile）
2. 截获页面自己发出的 `im/user/active/status` 请求体，取出全部会话 `sec_user_ids`
3. 用 `im/user/info` 批量换成 `uid / nickname / unique_id / avatar`（每批 20 个）
4. 按 `friend_user_id`（= uid）或 `sec_uid` upsert 到好友表

实测：账号「王壹」一次拿到 **14 位会话好友**，uid / 抖音号 / 头像全部正确。

## 发送的现状与两条路线

**现状**：发送请求走 `imapi.douyin.com/v1/message/*`，`content-type: application/x-protobuf`，
抓到的最小请求体形如 `\bû\u000f\u0010N\u001a\u00050.1.8...`，需要 protobuf 编解码 + 设备头
（`session_aid / session_did / app_name / priority_region / user_agent ...`）。

**路线 A：复刻 protobuf（协议级）**
- 需要拿到 `message/send` 的 `.proto` 定义（可从 PC 端 bundle 提取）
- 实现 `SendMessage` 请求体构造 + 响应解析
- 优点：轻、快、可批量；缺点：协议随客户端升级而变化，维护成本高

**路线 B：页面内自动化（UI 级）**
- Playwright 在已登录的聊天页里选择会话 → 输入 → 回车
- 优点：不依赖协议，抖音改版影响小；缺点：慢（每次 2–4s）、需要稳定的选择器
- 适合「每天每人 1 条」的续火花场景

**当前代码状态**：`sender` 仍按 HTTP 表单发送，因此真实发送会失败并把原因原样落库
（`私信接口返回 404: Unsupported path(Janus)`），任务不会伪造成功。
选择路线后替换 `signer.SendIM` 即可，其余（调度、幂等、重试、记录）无需改动。
