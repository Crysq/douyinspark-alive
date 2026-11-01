#!/usr/bin/env python3
"""卡密安全性验证：随机爆破、错误信息一致性、限流、XFF 伪造绕过测试。"""
import json, random, string, subprocess, urllib.request, urllib.error, time

BASE = "http://127.0.0.1:8790/api"
ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"


def call(method, path, token=None, body=None, xff=None, timeout=20):
    hdrs = {"Content-Type": "application/json"}
    if token:
        hdrs["Authorization"] = "Bearer " + token
    if xff:
        hdrs["X-Forwarded-For"] = xff
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode())
        except Exception:
            return e.code, {}
    except Exception as e:
        return 0, {"message": str(e)}


def flush(prefix):
    out = subprocess.run(["docker", "exec", "doyinspark-redis", "redis-cli", "--scan", "--pattern", prefix],
                         capture_output=True, text=True).stdout.split()
    if out:
        subprocess.run(["docker", "exec", "doyinspark-redis", "redis-cli", "del"] + out, capture_output=True)


def login(username, password):
    st, d = call("POST", "/auth/login", body={"username": username, "password": password})
    return d["data"]["token"] if st == 200 else None


print("== 准备：取一个普通用户令牌")
flush("doyinspark:rl:*")
admin = login("admin", "admin123")
st, d = call("POST", "/admin/cards/generate", admin, {"plan": "month", "count": 1})
code = d["data"]["cards"][0]["code"]
uname = "sec" + time.strftime("%H%M%S")
st, d = call("POST", "/auth/register", body={"username": uname, "password": "test123456", "cardCode": code})
user = d["data"]["token"]
print("   用户:", uname, "| 卡密样本:", code)

print("\n== 1) 随机卡密爆破（12 次，观察限流）")
random.seed(20261101)
codes, blocked = [], 0
for i in range(12):
    guess = "DYSP-" + "-".join("".join(random.choice(ALPHABET) for _ in range(4)) for _ in range(3))
    st, d = call("POST", "/membership/redeem", user, {"code": guess})
    mark = "429 限流" if st == 429 else f"{st}"
    if st == 429:
        blocked += 1
    codes.append((guess, st, d.get("message", "")))
    print(f"   #{i+1:02d} {guess} -> {mark} {d.get('message','')[:28]}")
print(f"   结果：{12 - blocked} 次进入校验、{blocked} 次被限流拦截")

print("\n== 2) 错误信息一致性（不存在 / 已使用 / 已作废 应完全一致）")
flush("doyinspark:rl:*")
used = code
st, d = call("POST", "/admin/cards/generate", admin, {"plan": "day", "count": 1})
void_code = d["data"]["cards"][0]["code"]
void_id = d["data"]["cards"][0]["id"]
call("POST", f"/admin/cards/{void_id}/void", admin)
msgs = []
for label, c in (("不存在", "DYSP-AAAA-BBBB-CCCC"), ("已使用", used), ("已作废", void_code)):
    st, d = call("POST", "/membership/redeem", user, {"code": c})
    msgs.append((label, st, d.get("message", "")))
    print(f"   {label}: HTTP {st} · {d.get('message','')}")
same = len({m[2] for m in msgs}) == 1
print("   文案是否一致:", "是（无法用作探测" if same else "否（存在信息泄露）")

print("\n== 3) 伪造 X-Forwarded-For 绕过限流（应仍被拦）")
flush("doyinspark:rl:register:*")
statuses = []
for i in range(12):
    st, d = call("POST", "/auth/register", body={
        "username": f"xff{int(time.time())}{i}", "password": "test123456", "cardCode": "DYSP-AAAA-BBBB-CCCC"},
        xff=f"203.0.113.{i + 1}")
    statuses.append(st)
print("   12 次注册（每次换一个伪造 IP）结果:", statuses)
print("   限流是否仍生效:", "是" if 429 in statuses else "否（可被伪造头绕过）")

print("\n== 4) 卡密空间估算")
space = 32 ** 12
print(f"   字母表 32 个字符 × 12 位（去掉 I/O/0/1） = {space:,} 种 ≈ 2^{round(__import__('math').log2(space))}")
per_5min, per_day = 8, 8 * 288
years = space / (per_day * 365)
print(f"   单 IP 限流 8 次/5 分钟 → 每天 {per_day} 次，穷举需要 {years:,.0f} 年")
