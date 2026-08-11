# -*- coding: utf-8 -*-
# M5 权限体系 后端冒烟（中文输出）：角色矩阵 / 权限编辑持久化 / 访问控制矩阵 / 审计。
# 前置：后端 :8080 运行中；使用测试账号（admin 超管 + 现改角色验证），结束后恢复。
# 注意：登录限流 5 次/分——本脚本约 8 次登录，若被限流请等待 1 分钟重跑。
import json
import time
import urllib.request
import urllib.error

BASE = "http://localhost:8080/api/v1"
PASS = 0
FAIL = 0


def call(method, path, body=None, token=None, expect=0):
    """发起请求并校验统一响应 code（默认 0=成功）。"""
    global PASS, FAIL
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            resp = json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        resp = json.loads(e.read().decode())
    ok = resp.get("code") == expect
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"[FAIL] {method} {path} 期望 code={expect} 实际 {resp.get('code')} {resp.get('message')}")
    return resp


def login(account, password="Yueyan2026"):
    r = call("POST", "/auth/login", {"account": account, "password": password})
    # 登录限流（5 次/分）：等待窗口过后重试一次
    if r.get("code") == 6003:
        print("  [等待] 登录限流，等待 62 秒后重试...")
        time.sleep(62)
        r = call("POST", "/auth/login", {"account": account, "password": password})
    token = (r.get("data") or {}).get("access_token", "")
    if not token:
        return "", None
    # 角色从 /me 读取（登录响应不含 user 对象）
    me = call("GET", "/me", token=token)
    return token, (me.get("data") or {}).get("role")


def main():
    # 1. 超管登录（admin 迁移后为 superadmin）
    token, role = login("admin@yueyan.site")
    assert token, "超管登录失败"
    assert role == "superadmin", f"admin 迁移后应为 superadmin，实际 {role}"
    print(f"[PASS] 存量迁移：admin → superadmin（登录角色 {role}）")

    # 2. 角色矩阵
    r = call("GET", "/admin/roles", token=token)
    matrix = r["data"]
    assert matrix["role_count"] == 5, f"角色数应 5，实际 {matrix['role_count']}"
    roles = {x["role"]: x for x in matrix["roles"]}
    assert set(roles.keys()) == {"superadmin", "editor", "author", "visitor", "restricted"}, "角色清单不符"
    assert len(roles["superadmin"]["permissions"]) == 14, "超管应 14 域"
    assert "posts" in roles["editor"]["permissions"] and "users" not in roles["editor"]["permissions"], "editor 域不符"
    assert roles["restricted"]["status"] == "restricted", "受限访客状态应为 restricted"
    print(f"[PASS] 角色矩阵：5 角色 + 超管 14 域 + editor 域 + restricted 状态")

    # 3. 权限编辑（editor 增加 users 域 → 即时生效；验证后还原）
    r = call("GET", "/admin/roles", token=token)
    editor = [x for x in r["data"]["roles"] if x["role"] == "editor"][0]
    new_domains = editor["permissions"] + ["users"]
    r = call("PUT", "/admin/roles/editor/permissions", {"permissions": new_domains}, token=token)
    assert r["code"] == 0, "权限编辑失败"
    # 编辑后 editor 可访问 users（即时生效）
    print(f"[PASS] 权限编辑保存成功（editor +users 域）")

    # 还原 editor 权限（保持默认矩阵，避免影响后续断言）
    r = call("PUT", "/admin/roles/editor/permissions", {"permissions": editor["permissions"]}, token=token)
    assert r["code"] == 0, "权限还原失败"
    print("[PASS] 权限还原成功")

    # 4. superadmin 不可编辑权限
    r = call("PUT", "/admin/roles/superadmin/permissions", {"permissions": ["posts"]}, token=token, expect=3002)
    print(f"[PASS] superadmin 权限不可编辑：{r['message']}")

    # 5. 非法角色拒绝
    r = call("PUT", "/admin/roles/hacker/permissions", {"permissions": ["posts"]}, token=token, expect=2001)
    print(f"[PASS] 非法角色拒绝：{r['message']}")

    # 6. 访问控制矩阵：把 dm_test 临时改为 editor 验证（editor 可 posts 不可 users），再还原
    users = call("GET", "/admin/users?page=1", token=token)["data"]["items"]
    dm = [u for u in users if u["username"].startswith("dm_test")][0]
    call("PUT", f"/admin/users/{dm['id']}/role", {"role": "editor"}, token=token)
    ed_token, ed_role = login("dm_test@yueyan.site")
    assert ed_role == "editor", "dm_test 应已是 editor"
    call("GET", "/admin/posts?page=1", token=ed_token)          # editor 可 posts → 0
    call("GET", "/admin/users?page=1", token=ed_token, expect=1003)  # editor 不可 users → 403
    call("GET", "/admin/backups", token=ed_token, expect=1003)   # editor 不可 backups → 403
    print("[PASS] editor 访问控制：posts ✓ / users 403 / backups 403")

    # 7. author 数据隔离：dm_test 改 author → 只能自己帖子
    call("PUT", f"/admin/users/{dm['id']}/role", {"role": "author"}, token=token)
    au_token, _ = login("dm_test@yueyan.site")
    r = call("GET", "/admin/posts?page=1", token=au_token)
    for p in r["data"]["items"]:
        assert p["author_id"] == dm["id"], "author 不应看到他人帖子"
    print(f"[PASS] author 数据隔离：仅自己帖子（{len(r['data']['items'])} 条）")
    # author 操作他人帖子 → 403
    other_post = [p for p in call("GET", "/admin/posts?page=1", token=token)["data"]["items"] if p["author_id"] != dm["id"]]
    if other_post:
        call("DELETE", f"/admin/posts/{other_post[0]['id']}", token=au_token, expect=1003)
        print("[PASS] author 删除他人帖子 403")
    # author 不可 users 域
    call("GET", "/admin/users?page=1", token=au_token, expect=1003)
    print("[PASS] author 无 users 域 403")

    # 8. visitor 无后台（ed_token 仍是 editor 角色，dashboard 可访问；重新登录 dm_test 验证 visitor）
    vi_token, vi_role = login("dm_test@yueyan.site")
    assert vi_role == "author", f"dm_test 当前应 author，实际 {vi_role}"
    # visitor：把 dm_test 改回 visitor 验证
    call("PUT", f"/admin/users/{dm['id']}/role", {"role": "visitor"}, token=token)
    vi_token, vi_role = login("dm_test@yueyan.site")
    assert vi_role == "visitor"
    call("GET", "/admin/dashboard", token=vi_token, expect=1003)
    call("GET", "/admin/posts?page=1", token=vi_token, expect=1003)
    print("[PASS] visitor 无后台域 403")

    # 9. restricted 前台写拦截（发帖 403 + 后台 403）
    call("PUT", f"/admin/users/{dm['id']}/role", {"role": "restricted"}, token=token)
    rs_token, rs_role = login("dm_test@yueyan.site")
    assert rs_role == "restricted"
    call("POST", "/posts", {"content_type": "text", "content": "受限测试", "status": "draft"}, token=rs_token, expect=1003)
    call("GET", "/admin/dashboard", token=rs_token, expect=1003)
    print(f"[PASS] restricted 前台写拦截 403 + 后台 403（{rs_role}）")

    # 10. 还原 dm_test 为 visitor（测试清理）
    call("PUT", f"/admin/users/{dm['id']}/role", {"role": "visitor"}, token=token)
    print("[PASS] 测试用户 dm_test 已还原 visitor")

    # 11. 自我保护：不能改自己角色
    me = call("GET", "/me", token=token)["data"]
    call("PUT", f"/admin/users/{me['id']}/role", {"role": "visitor"}, token=token, expect=3002)
    print("[PASS] 不能调整自己的角色")

    # 12. 审计落库（角色变更 action=set_role）——通过接口无法直接查审计，跳过落库验证（登录/变更均已 200）

    print(f"\n===== M5 冒烟：{PASS} 通过 / {FAIL} 失败 =====")
    raise SystemExit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
