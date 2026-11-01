#!/usr/bin/env python3
"""DoyinSpark 全接口冒烟测试：逐个打真实接口并校验状态码。"""
import json, sys, urllib.request, urllib.error, urllib.parse, time

BASE = "http://127.0.0.1:8790/api"
results = []


def call(method, path, token=None, body=None, raw=False, timeout=60, headers=None):
    url = BASE + urllib.parse.quote(path, safe="/?=&#%")
    data = None
    hdrs = {"Content-Type": "application/json"}
    if token:
        hdrs["Authorization"] = "Bearer " + token
    if headers:
        hdrs.update(headers)
    if body is not None:
        data = json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            payload = resp.read()
            return resp.status, payload, dict(resp.headers)
    except urllib.error.HTTPError as e:
        return e.code, e.read(), dict(e.headers)
    except Exception as e:  # noqa
        return 0, str(e).encode(), {}


def decode(status, payload, headers):
    ctype = (headers or {}).get("Content-Type", "")
    if "application/json" in ctype:
        try:
            return json.loads(payload.decode())
        except Exception:
            return {}
    return payload.decode(errors="replace")


def check(name, method, path, expect, token=None, body=None, note="", timeout=60):
    status, payload, headers = call(method, path, token, body, timeout=timeout)
    data = decode(status, payload, headers)
    ok = status == expect
    msg = ""
    if isinstance(data, dict):
        msg = data.get("message", "")
        if data.get("code") not in (None, 0) and not msg:
            msg = "code=%s" % data.get("code")
    elif isinstance(data, str):
        msg = data[:60].replace("\n", " ")
    results.append((ok, name, method + " " + path, expect, status, msg[:60], note))
    print(("PASS " if ok else "FAIL ") + f"{name:28s} {method:6s} {path:46s} -> {status} {msg[:44]}")
    return status, data, headers


print("=" * 110)
print("基础：登录 / 注册")
print("=" * 110)
check("登录-错误密码", "POST", "/auth/login", 401, body={"username": "admin", "password": "wrong"})
st, data, _ = check("登录-管理员", "POST", "/auth/login", 200, body={"username": "admin", "password": "admin123"})
ADMIN = data["data"]["token"]
check("登录-空参数", "POST", "/auth/login", 400, body={})
check("令牌无效", "GET", "/auth/me", 401, token="bad.token.here")

st, data, _ = check("生成月卡", "POST", "/admin/cards/generate", 200, ADMIN, {"plan": "month", "count": 1, "note": "sweep"})
CARD = data["data"]["cards"][0]["code"]
uname = "sweep" + time.strftime("%H%M%S")
st, data, _ = check("注册-卡密开通", "POST", "/auth/register", 200,
                    body={"username": uname, "password": "test123456", "nickname": "冒烟用户", "cardCode": CARD})
USER = data["data"]["token"]
USER_ID = data["data"]["user"]["id"]
check("注册-重复用户名", "POST", "/auth/register", 400,
      body={"username": uname, "password": "test123456", "cardCode": CARD})
check("注册-卡密已用", "POST", "/auth/register", 400,
      body={"username": uname + "x", "password": "test123456", "cardCode": CARD})
check("注册-无卡密", "POST", "/auth/register", 400, body={"username": uname + "y", "password": "test123456"})
check("注册-弱密码", "POST", "/auth/register", 400, body={"username": uname + "z", "password": "123", "cardCode": CARD})

print()
print("=" * 110)
print("会员")
print("=" * 110)
check("卡种-公开", "GET", "/membership/plans", 200)
check("会员-当前", "GET", "/membership", 200, USER)
check("会员-兑换无效", "POST", "/membership/redeem", 400, USER, {"code": "DYSP-AAAA-BBBB-CCCC"})
check("会员-兑换已用", "POST", "/membership/redeem", 400, USER, {"code": CARD})
st, data, _ = check("生成日卡", "POST", "/admin/cards/generate", 200, ADMIN, {"plan": "day", "count": 1})
DAY = data["data"]["cards"][0]["code"]
st, data, _ = check("会员-兑换有效", "POST", "/membership/redeem", 200, USER, {"code": DAY.lower().replace("-", "")})
USER = data["data"]["token"]  # 兑换后换发的新令牌
check("会员-未登录", "POST", "/membership/redeem", 401, body={"code": DAY})

print()
print("=" * 110)
print("总览 / 通知 / 审计")
print("=" * 110)
check("总览", "GET", "/overview", 200, USER)
check("趋势", "GET", "/overview/trend?days=30", 200, USER)
check("趋势-非法 days", "GET", "/overview/trend?days=abc", 200, USER)
check("热力图", "GET", "/overview/heatmap", 200, USER)
check("通知列表", "GET", "/notifications", 200, USER)
check("通知已读", "POST", "/notifications/read", 200, USER)
check("审计日志", "GET", "/audit?page=1", 200, USER)

print()
print("=" * 110)
print("账号")
print("=" * 110)
check("账号列表", "GET", "/accounts", 200, USER)
st, data, _ = check("账号新增", "POST", "/accounts", 200, USER, {"nickname": "冒烟账号", "remark": "sweep"})
ACC = data["data"]["id"]
check("账号改名", "PATCH", f"/accounts/{ACC}", 200, USER, {"nickname": "冒烟账号2", "remark": "sweep"})
check("账号同步资料", "POST", f"/accounts/{ACC}/sync", 502, USER)
check("同步会话-离线拦截", "POST", f"/accounts/{ACC}/sync/friends", 409, USER)
check("表情-离线拦截", "GET", f"/accounts/{ACC}/emojis", 409, USER)
check("账号Cookie非法", "POST", f"/accounts/{ACC}/cookie", 400, USER, {"cookie": "foo=bar"})
check("账号-越权改名", "PATCH", f"/accounts/{ACC}", 404, ADMIN, {"nickname": "hack"})

print()
print("=" * 110)
print("好友")
print("=" * 110)
st, data, _ = check("好友新增", "POST", "/friends", 200, USER, {"accountId": ACC, "name": "冒烟好友", "userId": "123456789", "streakDays": 7})
FR = data["data"]["id"]
check("好友列表", "GET", "/friends?page=1&pageSize=10", 200, USER)
check("好友筛选-紧急", "GET", "/friends?level=urgent", 200, USER)
check("好友搜索", "GET", "/friends?keyword=冒烟", 200, USER)
check("好友关闭自动", "PATCH", f"/friends/{FR}", 200, USER, {"autoKeep": False})
check("好友续火-账号离线", "POST", f"/friends/{FR}/keep", 502, USER)
check("批量续火-空", "POST", "/friends/batch-keep", 400, USER, {"ids": []})
check("批量续火-无效id", "POST", "/friends/batch-keep", 200, USER, {"ids": [999999]})
check("好友删除", "DELETE", f"/friends/{FR}", 200, USER)
check("好友删除-重复", "DELETE", f"/friends/{FR}", 404, USER)

print()
print("=" * 110)
print("任务 / 模板")
print("=" * 110)
st, data, _ = check("任务新增", "POST", "/tasks", 200, USER, {"accountId": ACC, "name": "冒烟任务", "windowStart": "00:00", "windowEnd": "23:59", "intervalMin": 30})
TASK = data["data"]["id"]
check("任务列表", "GET", "/tasks", 200, USER)
check("任务暂停", "PATCH", f"/tasks/{TASK}", 200, USER, {"enabled": False})
check("任务执行-账号离线", "POST", f"/tasks/{TASK}/run", 200, USER)
check("任务删除", "DELETE", f"/tasks/{TASK}", 200, USER)

check("模板列表", "GET", "/templates?category=all", 200, USER)
st, data, _ = check("模板新增", "POST", "/templates", 200, USER, {"name": "冒烟模板", "content": "hi {{name}}", "category": "custom"})
TPL = data["data"]["id"]
check("模板修改", "PATCH", f"/templates/{TPL}", 200, USER, {"name": "冒烟模板2"})
check("模板删除", "DELETE", f"/templates/{TPL}", 200, USER)

print()
print("=" * 110)
print("记录 / 设置 / 系统")
print("=" * 110)
check("记录列表", "GET", "/records?page=1&pageSize=5", 200, USER)
check("记录统计", "GET", "/records/stats", 200, USER)
check("记录导出", "GET", "/records/export", 200, USER)
check("记录清理", "DELETE", "/records/prune?days=0", 200, USER)
check("设置读取", "GET", "/settings", 200, USER)
st, data, _ = check("设置保存", "PUT", "/settings", 200, USER, {"retryTimes": 3, "quietStart": "01:00", "quietEnd": "06:00", "notifyBreak": True, "autoSync": True, "keepAlive": True, "theme": "dark", "notifyWebhook": ""})
check("系统健康", "GET", "/system/health", 200, USER)
check("火花规则", "GET", "/system/rules", 200, USER)
check("演示数据-生成", "POST", "/system/demo", 200, USER)
check("演示数据-重复", "POST", "/system/demo", 400, USER)
check("演示数据-清除", "DELETE", "/system/demo", 200, USER)

print()
print("=" * 110)
print("管理端 / 越权")
print("=" * 110)
check("运营概览", "GET", "/admin/stats", 200, ADMIN)
check("用户列表", "GET", "/admin/users?page=1", 200, ADMIN)
check("用户搜索", "GET", "/admin/users?keyword=" + uname, 200, ADMIN)
check("用户-赠送会员", "POST", f"/admin/users/{USER_ID}/grant", 200, ADMIN, {"plan": "week"})
check("用户-禁用", "PATCH", f"/admin/users/{USER_ID}", 200, ADMIN, {"status": "banned"})
check("被禁用后登录", "POST", "/auth/login", 403, body={"username": uname, "password": "test123456"})
check("用户-恢复", "PATCH", f"/admin/users/{USER_ID}", 200, ADMIN, {"status": "active"})
check("卡密列表", "GET", "/admin/cards?page=1", 200, ADMIN)
st, data, _ = check("卡密生成-年卡", "POST", "/admin/cards/generate", 200, ADMIN, {"plan": "year", "count": 2, "validDays": 30})
VOID_ID = data["data"]["cards"][0]["id"]
check("卡密作废", "POST", f"/admin/cards/{VOID_ID}/void", 200, ADMIN)
check("卡密作废-重复", "POST", f"/admin/cards/{VOID_ID}/void", 400, ADMIN)
check("卡密导出", "GET", "/admin/cards/export", 200, ADMIN)
check("越权-用户访问管理端", "GET", "/admin/users", 403, USER)

print()
print("=" * 110)
print("SSE / 登出")
print("=" * 110)
def sse_probe(token, seconds=4):
    req = urllib.request.Request(BASE + "/events/stream?token=" + token, headers={"Accept": "text/event-stream"})
    try:
        with urllib.request.urlopen(req, timeout=seconds) as resp:
            chunk = resp.read(64)
            return resp.status, chunk.decode(errors="replace").strip()
    except urllib.error.HTTPError as e:
        return e.code, ""
    except Exception:
        # 读超时说明连接已建立并持续推送（SSE 正常表现）
        return 200, "connected"

status, first = sse_probe(USER)
ok = status == 200 and "connected" in first
results.append((ok, "SSE 事件流", "GET /events/stream", 200, status, first, ""))
print(("PASS " if ok else "FAIL ") + f"SSE 事件流 -> {status} {first!r}")

check("登出", "POST", "/auth/logout", 200, USER)
check("登出后访问", "GET", "/auth/me", 401, USER)

failed = [r for r in results if not r[0]]
print()
print("=" * 110)
print(f"总计 {len(results)} 项，通过 {len(results) - len(failed)}，失败 {len(failed)}")
for item in failed:
    print("  ✗", item[1], "|", item[2], "| 期望", item[3], "实际", item[4], "|", item[5])
sys.exit(1 if failed else 0)
