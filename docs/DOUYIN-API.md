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

## 发送协议（2026-11 实测进展）

已确认的事实：

1. `imapi.douyin.com` **可以从 Node 侧带 Cookie 直连**（不受页面 CORS 限制）
2. 请求体是**明文 protobuf**（不是 LZ4）：抓到的 `get_message_by_init` 请求开头
   `08 fb0f 10 91 4e 1a 05 "0.1.8" 22 00 28 03 30 01 3a 18 "0d50935:feat/pc-im-group"...`
3. 把这条真实请求原样回放，返回 **HTTP 200 + 真实会话数据**（`"\x02OK"` + 消息体），通道可用
4. 页面内的 `fetch` 调 imapi 会被 CORS 拦（`Failed to fetch`），必须走 Node
5. 目前拿不到**发送请求的样本**：容器内聊天详情页打不开（右侧一直转圈），
   无法触发一次真实发送去截获 proto 结构

因此还差最后一块：`send` 的 proto 字段定义（conversation_id / to_user_id / msg_type /
content / client_msg_id 的字段号与嵌套关系）。

拿到样本后的实现路径（已就绪的部分）：

```
Node(签名服务) ──cookie──► POST https://imapi.douyin.com/<send path>
                            body = protobuf(明文)
```

`/imapi-probe` 已经能带 Cookie 发任意 protobuf 请求，Go 侧 `signer` 也已把 404 映射成
可读错误；只要补齐 proto 编码即可替换 `SendIM`，调度/幂等/重试/记录都不用改。

### 发送协议（已解出）

真实发送请求（用户浏览器抓包，已按字节还原）：

```
POST https://imapi.douyin.com/v1/message/send?msToken=…&a_bogus=…&verifyFp=…&fp=…
content-type: application/x-protobuf

信封:  field1=100 (IMCMD.SEND_MESSAGE)  field2=10237 (方法号)
       field3="0.1.8"  field5=3  field7="0d50935:feat/pc-im-group"
       field8 = { field100: 内层 }
内层:  f1 = "0:1:<对方uid>:<自己uid>"   // 会话 ID
       f2 = 1
       f3 = 纳秒时间戳 (varint)
       f4 = {"aweType":700,"type":0,"richTextInfos":[],"text":"…"}
       f5 = [ {s:mentioned_users: ""}, {s:client_message_id: <uuid>}, {s:stime: "<ms>.<us>"} ]
       尾部 = 40 字节不透明（含 msg_type=7 与一段 token）
```

必需请求头：`bd-ticket-guard-client-data`（设备级签名，由页面安全 SDK 在**发请求瞬间**生成）、
`bd-ticket-guard-ree-public-key`、`bd-ticket-guard-version`、`bd-ticket-guard-web-sign-type`、
`bd-ticket-guard-web-version`，以及 `a_bogus`/`msToken` 签名。

已打通：信封构造、内层字段、时间戳与 UUID 生成、`a_bogus`（frontierSign）、
`securitySDK.getBdTicketGuardHeader` 的 5 个常驻头。

未打通：`bd-ticket-guard-client-data` —— 页面 SDK 的 fetch 拦截器在请求发出前注入，
直接调用 SDK 的公开方法拿不到；从页面内 `fetch` 直连 imapi 会被 CORS 拦（`Failed to fetch`）。

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
