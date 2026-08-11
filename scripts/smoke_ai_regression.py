# -*- coding: utf-8 -*-
# M4-AI 回归冒烟（中文输出）：验证评论发布/列表、审核队列、帖子详情不受 AI 异步预审影响。
# 运行：python scripts/smoke_ai_regression.py（需要后端 :8080 运行中）
import json
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


def main():
    # 登录（限流 5 次/分，仅一次）
    login = call("POST", "/auth/login", {"account": "admin@yueyan.site", "password": "Yueyan2026"})
    token = login.get("data", {}).get("access_token", "")
    assert token, "管理员登录失败"

    # 1. 帖子详情（前台公开接口，不带 token 也应 200）
    r = call("GET", "/posts/5")
    assert r.get("data", {}).get("id") == 5, "帖子详情异常"
    print("[PASS] 帖子详情正常（id=5）")

    # 2. 时间线（AI 预审不影响发帖列表）
    r = call("GET", "/posts?page=1")
    assert "items" in r.get("data", {}), "时间线异常"
    print(f"[PASS] 时间线正常（{len(r['data']['items'])} 条）")

    # 3. 评论发布（登录用户）：AI 异步预审失败应静默，评论正常创建
    r = call("POST", "/posts/5/comments", {"content": "M4-AI 回归测试评论，验证预审注入不影响发布流程。"}, token=token)
    comment_id = r.get("data", {}).get("comment_id") or r.get("data", {}).get("id")
    assert comment_id, f"评论创建失败：{r}"
    print(f"[PASS] 评论发布正常（id={comment_id}，AI 预审异步静默）")

    # 4. 评论列表（新评论可见；data 为数组）
    r = call("GET", "/posts/5/comments")
    contents = [c.get("content", "") for c in r.get("data", [])]
    assert any("M4-AI 回归测试评论" in c for c in contents), "新评论未出现在列表"
    print("[PASS] 评论列表正常（新评论可见）")

    # 5. 评论删除（清理测试数据）
    r = call("DELETE", f"/comments/{comment_id}", token=token)
    assert r.get("code") == 0, f"评论删除失败：{r}"
    print("[PASS] 评论删除正常（测试数据已清理）")

    # 6. 审核队列列表（含 source 字段）
    r = call("GET", "/admin/reports?page=1", token=token)
    items = r.get("data", {}).get("items", [])
    for it in items:
        assert "source" in it, "工单缺少 source 字段"
    print(f"[PASS] 审核队列列表正常（{len(items)} 条，均含 source）")

    # 7. 评论管理列表（后台）
    r = call("GET", "/admin/comments?page=1", token=token)
    assert "items" in r.get("data", {}), "评论管理列表异常"
    print("[PASS] 后台评论管理列表正常")

    print(f"\n===== 回归结果：{PASS} 通过 / {FAIL} 失败 =====")
    raise SystemExit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
