# -*- coding: utf-8 -*-
# M4-AI 后端冒烟脚本（中文输出）：供应商/任务/用量/场景错误路径/审核队列联动。
# 运行：python scripts/smoke_ai.py（需要后端 :8080 运行中）
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
    tag = "PASS" if ok else "FAIL"
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"[{tag}] {method} {path} 期望 code={expect} 实际 {resp.get('code')} {resp.get('message')}")
    return resp


def main():
    # 1. 管理员登录（限流 5 次/分，仅登录一次）
    login = call("POST", "/auth/login", {"account": "admin@yueyan.site", "password": "Yueyan2026"})
    token = login.get("data", {}).get("access_token", "")
    assert token, "管理员登录失败"

    # 2. 供应商列表：5 个种子且未配 Key
    r = call("GET", "/admin/ai/providers", token=token)
    items = r.get("data", {}).get("items", [])
    names = [p["name"] for p in items]
    assert len(items) == 5, f"种子供应商应为 5 个，实际 {len(items)}"
    assert all(p["api_key_set"] is False for p in items), "种子供应商不应配置 Key"
    print(f"[PASS] 供应商列表：{names}（5 个种子，均未配 Key）")

    # 3. 任务列表：3 条内置任务
    r = call("GET", "/admin/ai/tasks", token=token)
    tasks = r.get("data", {}).get("items", [])
    tnames = [t["task_name"] for t in tasks]
    assert tnames == ["comment.review", "post.summary", "post.tags"], f"任务名不符：{tnames}"
    print(f"[PASS] 任务列表：{tnames}")

    # 4. 用量统计：汇总 + 7 日
    r = call("GET", "/admin/ai/usage", token=token)
    usage = r.get("data", {})
    assert "summary" in usage and len(usage.get("days", [])) == 7, "用量统计结构不符"
    print(f"[PASS] 用量统计：今日 {usage['summary']['today_calls']} 次 / 近 7 日 {len(usage['days'])} 天")

    # 5. 场景错误路径：未配 Key 时生成摘要 → 6002 上游不可用
    r = call("POST", "/admin/ai/gen/summary?post_id=5", token=token, expect=6002)
    print(f"[PASS] 摘要场景未配 Key 拦截：{r.get('message')}")

    # 6. 场景错误路径：自动标签同理
    r = call("POST", "/admin/ai/gen/tags?post_id=5", token=token, expect=6002)
    print(f"[PASS] 标签场景未配 Key 拦截：{r.get('message')}")

    # 7. 场景错误路径：任务停用 → 3002 状态冲突
    call("POST", "/admin/ai/tasks/post.tags/toggle", {"enabled": False}, token=token)
    r = call("POST", "/admin/ai/gen/tags?post_id=5", token=token, expect=3002)
    print(f"[PASS] 任务停用拦截：{r.get('message')}")
    call("POST", "/admin/ai/tasks/post.tags/toggle", {"enabled": True}, token=token)

    # 8. 供应商 CRUD：新增 → 更新（Key 留空不改）→ 测试连接（错误路径）→ 删除
    r = call("POST", "/admin/ai/providers", {
        "name": "test-provider", "base_url": "https://example.com/v1",
        "api_key": "sk-test-123", "models": ["test-model"], "enabled": False, "priority": 99,
    }, token=token)
    pid = r.get("data", {}).get("id")
    assert pid, "新增供应商失败"
    print(f"[PASS] 新增供应商 id={pid}")

    r = call("GET", "/admin/ai/providers", token=token)
    p = [x for x in r["data"]["items"] if x["id"] == pid][0]
    assert p["api_key_set"] is True and p["enabled"] is False, "新供应商 Key 未加密存储"
    print("[PASS] 供应商 Key 已加密（api_key_set=True，不回显明文）")

    call("PUT", f"/admin/ai/providers/{pid}", {
        "name": "test-provider", "base_url": "https://example.com/v2",
        "api_key": "", "models": ["test-model"], "enabled": True, "priority": 98,
    }, token=token)
    r = call("GET", "/admin/ai/providers", token=token)
    p = [x for x in r["data"]["items"] if x["id"] == pid][0]
    assert p["api_key_set"] is True and p["base_url"].endswith("v2"), "编辑留空 Key 不应覆盖"
    print("[PASS] 编辑 Key 留空保持原值 + base_url 更新成功")

    r = call("POST", f"/admin/ai/providers/{pid}/test", token=token, expect=6002)
    print(f"[PASS] 测试连接（假供应商）：{r.get('message')}")

    # TestProvider 无 body 参数：空 body 同样执行连通性测试（不校验 body）
    r = call("POST", f"/admin/ai/providers/{pid}/test", token=token, expect=6002)
    print(f"[PASS] 测试连接（无 body 同样执行）：{r.get('message')}")

    call("DELETE", f"/admin/ai/providers/{pid}", token=token)
    r = call("GET", "/admin/ai/providers", token=token)
    assert len(r["data"]["items"]) == 5, "删除供应商后应回到 5 个种子"
    print("[PASS] 删除供应商成功")

    # 9. 任务配置更新（提示词 + max_tokens）
    call("PUT", "/admin/ai/tasks/post.summary", {
        "provider_id": None, "model": "", "prompt_template": "测试提示词 {title} {content}", "max_tokens": 600,
    }, token=token)
    r = call("GET", "/admin/ai/tasks", token=token)
    t = [x for x in r["data"]["items"] if x["task_name"] == "post.summary"][0]
    assert t["max_tokens"] == 600, "任务配置更新失败"
    print("[PASS] 任务配置更新（提示词/max_tokens）成功")

    # 10. 审核队列统计：high_risk 字段存在
    r = call("GET", "/admin/reports/stats", token=token)
    stats = r.get("data", {})
    assert "high_risk" in stats, "统计缺少 high_risk 字段"
    print(f"[PASS] 审核队列统计含高风险：high_risk={stats['high_risk']} pending={stats['pending']}")

    # 11. verdict 错误路径：不存在工单 → 2002
    r = call("POST", "/admin/reports/999999/verdict", {"action": "allow"}, token=token, expect=2002)
    print(f"[PASS] verdict 不存在工单拦截：{r.get('message')}")

    # 12. 评论 AI 审核接口（手动批量）：空列表 → 2001
    r = call("POST", "/admin/ai/review/comments", {"comment_ids": []}, token=token, expect=2001)
    print(f"[PASS] 批量审核空列表拦截：{r.get('message')}")

    print(f"\n===== 冒烟结果：{PASS} 通过 / {FAIL} 失败 =====")
    raise SystemExit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
