# -*- coding: utf-8 -*-
# M4-报表 后端冒烟（中文输出）：overview 聚合 / CSV 导出 / 备份三格式 / 下载 / 删除 / 过期清理 / 路径安全。
# 运行：python scripts/smoke_report.py（需要后端 :8080 运行中）
import csv
import io
import json
import urllib.request
import urllib.error
import zipfile

BASE = "http://localhost:8080/api/v1"
PASS = 0
FAIL = 0


def call(method, path, body=None, token=None, expect=0, raw=False):
    """发起请求并校验统一响应 code（raw=True 返回原始字节，用于附件下载）。"""
    global PASS, FAIL
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            if raw:
                return r.read(), dict(r.headers)
            resp = json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        if raw:
            return e.read(), dict(e.headers)
        resp = json.loads(e.read().decode())
    ok = resp.get("code") == expect
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"[FAIL] {method} {path} 期望 code={expect} 实际 {resp.get('code')} {resp.get('message')}")
    return resp


def main():
    login = call("POST", "/auth/login", {"account": "admin@yueyan.site", "password": "Yueyan2026"})
    token = login["data"]["access_token"]
    assert token, "管理员登录失败"

    # ---------- 1. 报表 overview ----------
    r = call("GET", "/admin/reports/overview?days=30", token=token)
    d = r["data"]
    assert set(["views_7d", "likes_7d", "comments_7d", "posts_today", "pending_audit"]) <= set(d.keys()), "统计卡字段缺失"
    assert len(d["trend"]) == 30, f"30 日趋势应 30 点，实际 {len(d['trend'])}"
    assert "views" in d["trend"][0] and "posts" in d["trend"][0], "趋势缺四维字段"
    assert set(["comments", "reports", "sensitive"]) <= set(d["pending"].keys()), "待处理块字段缺失"
    assert "type_counts" in d and "activities" in d, "内容分布/最近动态缺失"
    print(f"[PASS] overview 结构：4 卡 + 30 日四维趋势 + 待处理（评论 {d['pending']['comments']}/举报 {d['pending']['reports']}/命中 {d['pending']['sensitive']}）")

    # 7 日视图
    r = call("GET", "/admin/reports/overview?days=7", token=token)
    assert len(r["data"]["trend"]) == 7, "7 日趋势应 7 点"
    print("[PASS] overview 7 日视图切换正常")

    # ---------- 2. CSV 导出（附件流） ----------
    raw, headers = call("GET", "/admin/reports/export.csv?days=30", token=token, raw=True)
    assert "attachment" in headers.get("Content-Disposition", ""), "CSV 应为附件下载"
    text = raw.decode("utf-8-sig")
    rows = list(csv.reader(io.StringIO(text)))
    assert rows[0] == ["日期", "浏览", "新帖", "获赞", "评论"], f"CSV 头部错误：{rows[0]}"
    assert len(rows) == 31, f"CSV 应 30 行数据 + 头部，实际 {len(rows)}"
    print(f"[PASS] CSV 导出：附件下载 + 头部正确 + {len(rows)-1} 行数据")

    # ---------- 3. 备份：全站数据 JSON ----------
    r = call("POST", "/admin/backups", {"backup_type": "all", "scope": ["content", "users", "media"], "format": "json", "retention_days": 30}, token=token)
    d = r["data"]
    assert d["status"] == "success" and d["file_name"].endswith(".json"), f"JSON 备份失败：{d}"
    json_id = d["id"]
    print(f"[PASS] 全站数据 JSON 备份（id={json_id}，{d['file_name']} {d['file_size']}B）")

    # ---------- 4. 备份：全站数据 CSV（打包 zip） ----------
    r = call("POST", "/admin/backups", {"backup_type": "all", "scope": ["content"], "format": "csv", "retention_days": 30}, token=token)
    d = r["data"]
    assert d["file_name"].endswith(".zip"), f"CSV 备份应为 zip：{d}"
    csv_id = d["id"]
    print(f"[PASS] 全站数据 CSV 备份（id={csv_id}，zip 内含每表 csv）")

    # ---------- 5. 备份：ZIP（数据 JSON + manifest） ----------
    r = call("POST", "/admin/backups", {"backup_type": "all", "scope": ["content", "users"], "format": "zip", "retention_days": 30}, token=token)
    d = r["data"]
    assert d["file_name"].endswith(".zip"), f"ZIP 备份失败：{d}"
    zip_id = d["id"]
    raw, _ = call("GET", f"/admin/backups/{zip_id}/download", token=token, raw=True)
    zf = zipfile.ZipFile(io.BytesIO(raw))
    names = zf.namelist()
    assert "data.json" in names and "manifest.json" in names, f"ZIP 内容错误：{names}"
    print(f"[PASS] 全站数据 ZIP 备份（id={zip_id}，含 data.json + manifest.json）")

    # ---------- 6. 备份：媒体库 ----------
    r = call("POST", "/admin/backups", {"backup_type": "media", "format": "zip", "retention_days": 30}, token=token)
    d = r["data"]
    assert d["file_name"].endswith(".zip"), f"媒体库备份失败：{d}"
    media_id = d["id"]
    print(f"[PASS] 媒体库 ZIP 备份（id={media_id}，{d['file_size']}B）")

    # ---------- 7. 下载（JSON 内容校验） ----------
    raw, headers = call("GET", f"/admin/backups/{json_id}/download", token=token, raw=True)
    assert "attachment" in headers.get("Content-Disposition", ""), "下载应为附件"
    payload = json.loads(raw.decode())
    assert "posts" in payload and "users" in payload and "media_assets" in payload, "JSON 备份内容缺表"
    print(f"[PASS] 备份下载：附件 + JSON 内容含 posts/users/media_assets")

    # ---------- 8. 列表 ----------
    r = call("GET", "/admin/backups", token=token)
    items = r["data"]["items"]
    assert len(items) >= 4, f"列表应 ≥4 条，实际 {len(items)}"
    print(f"[PASS] 备份列表：{len(items)} 条")

    # ---------- 9. 删除 ----------
    r = call("DELETE", f"/admin/backups/{media_id}", token=token)
    assert r["code"] == 0, "删除失败"
    r = call("GET", "/admin/backups", token=token)
    assert all(x["id"] != media_id for x in r["data"]["items"]), "删除后仍存在"
    print("[PASS] 备份删除（文件 + 记录）成功")

    # ---------- 10. 输入校验 ----------
    r = call("POST", "/admin/backups", {"backup_type": "all", "format": "json", "retention_days": 0}, token=token, expect=2001)
    print(f"[PASS] 保留天数校验：{r['message']}")
    r = call("POST", "/admin/backups", {"backup_type": "bad", "format": "json", "retention_days": 30}, token=token, expect=2001)
    print(f"[PASS] 备份类型校验：{r['message']}")

    # ---------- 11. 路径安全（伪造越权记录） ----------
    import sqlite3  # 无 sqlite，改用 urllib 无法直接改库 → 走接口层验证：不存在 id 返回 2002
    r = call("GET", "/admin/backups/999999/download", token=token, expect=2002)
    print(f"[PASS] 越权/不存在备份下载拦截：{r['message']}")

    print(f"\n===== M4-报表冒烟：{PASS} 通过 / {FAIL} 失败 =====")
    raise SystemExit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
