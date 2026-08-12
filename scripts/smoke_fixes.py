# -*- coding: utf-8 -*-
"""
scripts/smoke_fixes.py
修复批次回归冒烟（2026-08-13 后置修复 P0~P2）：
  1. 站点设置「关闭注册」后注册接口 403（1003）
  2. 站点设置「关闭评论」后评论接口 403（1003）
  3. 注销账号：注册 → 注销（code 0）→ 原账号无法登录
  4. 最后一名超级管理员保护：admin 封禁自己 3002
  5. 多超管时角色调整放行（dm_test 临时提升 superadmin 后降级还原）
注意：登录限流 5 次/分；脚本 login 2 次（admin + 注销用户），重跑间隔 1 分钟以上。
"""
import json
import random
import string
import urllib.request

BASE = "http://localhost:8080/api/v1"

passed = 0
failed = 0


def call(method, path, token="", body=None, expect=0):
    """通用请求：返回 {code,message,data}；expect 非 0 时校验错误码。"""
    global passed, failed
    req = urllib.request.Request(BASE + path, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    data = json.dumps(body).encode("utf-8") if body is not None else None
    try:
        with urllib.request.urlopen(req, data=data) as resp:
            result = json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        result = json.loads(err.read().decode("utf-8"))
    if expect:
        ok = result.get("code") == expect
    else:
        ok = result.get("code") == 0
    if ok:
        passed += 1
        print(f"[PASS] {method} {path} → code {result.get('code')}")
    else:
        failed += 1
        print(f"[FAIL] {method} {path} → code {result.get('code')}（期望 {expect}）message={result.get('message')}")
    return result


def login(account, password, expect=0):
    result = call("POST", "/auth/login", body={"account": account, "password": password}, expect=expect)
    return (result.get("data") or {}).get("access_token", "")


# ---------- 1. admin 登录 ----------
admin_token = login("admin@yueyan.site", "Yueyan2026")
assert admin_token, "admin 登录失败，冒烟中止"

# ---------- 2. 关闭注册 → 注册 403 → 还原 ----------
settings = call("GET", "/admin/settings", token=admin_token)["data"]
original_register = settings.get("allow_register", "true")
original_comment = settings.get("comment_open", "true")
call("PUT", "/admin/settings", token=admin_token, body={"allow_register": "false"})
call("POST", "/auth/register", body={
    "nickname": "冒烟临时用户",
    "email": f"smoke{random.randint(1000, 9999)}@yueyan.site",
    "password": "Smoke2026",
}, expect=1003)
call("PUT", "/admin/settings", token=admin_token, body={"allow_register": original_register})
print(f"[INFO] allow_register 已还原为 {original_register}")

# ---------- 3. 关闭评论 → 评论 403 → 还原 ----------
posts = call("GET", "/posts?page=1&page_size=5").get("data") or {}
post_id = (posts.get("items") or [{}])[0].get("id")
assert post_id, "时间线无帖子，评论开关用例跳过（不影响其他断言）"
call("PUT", "/admin/settings", token=admin_token, body={"comment_open": "false"})
call("POST", f"/posts/{post_id}/comments", token=admin_token, body={"content": "冒烟评论"}, expect=1003)
call("PUT", "/admin/settings", token=admin_token, body={"comment_open": original_comment})
print(f"[INFO] comment_open 已还原为 {original_comment}")

# ---------- 4. 注销账号：注册 → 注销 → 无法登录 ----------
suffix = "".join(random.choices(string.ascii_lowercase, k=6))
email = f"bye{suffix}@yueyan.site"
account = ""
reg = call("POST", "/auth/register", body={"nickname": "待注销", "email": email, "password": "Goodbye2026"})
user_token = (reg.get("data") or {}).get("access_token", "")
me = call("GET", "/me", token=user_token)
account = (me.get("data") or {}).get("username", "")
call("POST", "/me/deactivate", token=user_token)
login(email, "Goodbye2026", expect=1001)  # 邮箱登录：账号不存在 → 1001
if account:
    login(account, "Goodbye2026", expect=1001)  # 用户名登录：账号不存在 → 1001

# ---------- 5. 最后一名超级管理员保护：admin 封禁自己 3002 ----------
me_data = call("GET", "/me", token=admin_token)["data"]
admin_id = me_data["id"]
call("PUT", f"/admin/users/{admin_id}/status", token=admin_token,
     body={"status": "banned", "reason": "冒烟自封禁"}, expect=3002)

# ---------- 6. 多超管角色调整放行（dm_test 提升 → 降级还原 visitor） ----------
users = call("GET", "/admin/users?page=1&page_size=50", token=admin_token)["data"]["items"]
dm = next((u for u in users if str(u.get("username", "")).startswith("dm_test")), None)
if dm:
    call("PUT", f"/admin/users/{dm['id']}/role", token=admin_token, body={"role": "superadmin"})
    call("PUT", f"/admin/users/{dm['id']}/role", token=admin_token, body={"role": "visitor"})
    print("[PASS] 多超管角色调整放行（dm_test 已还原 visitor）")
    passed += 1
else:
    print("[SKIP] 未找到 dm_test，跳过角色调整用例")

print(f"\n===== 修复批次冒烟：{passed} 通过 / {failed} 失败 =====")
raise SystemExit(1 if failed else 0)
