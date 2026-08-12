# -*- coding: utf-8 -*-
# M3.3+M3.4+M3.5 插件 后端冒烟（中文输出）：
#   .bpk 打包上传安装（M3.4）→ 启用（go-plugin 拉起）→ 钩子/API 验证（M3.3）
#   → 许可证链路（M3.5：demo 模式 → 签发 → 激活 → pro 功能 → 篡改/过期拒绝）
#   → 崩溃自愈/熔断 → 禁用/卸载。
# 前置：
#   1. 后端 :8080 运行中（./scripts/dev-server.sh --daemon）
#   2. demo 密钥对已生成（data/demo-keys/private.pem + public.pem，见验收报告）
# 注意：登录限流 5 次/分——脚本仅 1 次登录，正常不会触发。
import json
import os
import subprocess
import sys
import time
import uuid
import urllib.error
import urllib.parse
import urllib.request

BASE = "http://localhost:8080/api/v1"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PLUGIN_LOG = os.path.join(ROOT, "logs", "plugins", "demo-plugin.log")
BPK_FILE = os.path.join(ROOT, "dist", "demo-plugin-0.1.0-windows-amd64.bpk")
KEYS_DIR = os.path.join(ROOT, "data", "demo-keys")
LICENSE_JWT = os.path.join(KEYS_DIR, "license.jwt")
PASS = 0
FAIL = 0


def db_conn():
    """从 .env 读取数据库配置并连接（psycopg2）。"""
    env = {}
    with open(os.path.join(ROOT, ".env"), encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env[k.strip()] = v.strip().strip('"')
    return psycopg2.connect(
        host=env.get("POSTGRES_HOST", "localhost"),
        port=env.get("POSTGRES_PORT", "5432"),
        user=env.get("POSTGRES_USER", "postgres"),
        password=env.get("POSTGRES_PASSWORD", ""),
        dbname=env.get("POSTGRES_DB", "Blog"),
    )


def db_seed_demo():
    """直插 demo-plugin 实例（installed 状态；清理旧记录保证幂等）。"""
    conn = db_conn()
    try:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM plugin_instances WHERE plugin_id = 'demo-plugin'")
            cur.execute(
                "INSERT INTO plugin_instances (plugin_id, name, version, repo_url, state) "
                "VALUES ('demo-plugin', '演示插件', '0.1.0', 'https://github.com/roberts9012062/boke', 'installed')"
            )
        conn.commit()
    finally:
        conn.close()


def call(method, path, body=None, token=None, expect=0, raw=False):
    """发起请求；raw=True 返回原始文本（插件 API 非统一包装），否则校验统一 code。"""
    global PASS, FAIL
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            text = r.read().decode()
    except urllib.error.HTTPError as e:
        text = e.read().decode()
    if raw:
        return text
    resp = json.loads(text)
    ok = resp.get("code") == expect
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"[FAIL] {method} {path} 期望 code={expect} 实际 {resp.get('code')} {resp.get('message')}")
    return resp


def upload_bpk(token, bpk_path):
    """multipart 上传 .bpk（urllib 手写 multipart body）。"""
    with open(bpk_path, "rb") as f:
        content = f.read()
    boundary = "----bpk" + uuid.uuid4().hex
    head = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{os.path.basename(bpk_path)}"\r\n'
        "Content-Type: application/octet-stream\r\n\r\n"
    ).encode()
    tail = f"\r\n--{boundary}--\r\n".encode()
    req = urllib.request.Request(BASE + "/admin/plugins/upload", data=head + content + tail, method="POST")
    req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
    req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return json.loads(e.read().decode())


def login(account, password="Yueyan2026"):
    r = call("POST", "/auth/login", {"account": account, "password": password})
    token = (r.get("data") or {}).get("access_token", "")
    if not token:
        print("[FAIL] 登录失败")
    return token


def build_bpk():
    """cmd/bp 打包 demo 插件（scripts/build-demo-bpk.sh → dist/demo-plugin-0.1.0-windows-amd64.bpk）。"""
    r = subprocess.run(["bash", os.path.join(ROOT, "scripts", "build-demo-bpk.sh")],
                       capture_output=True, timeout=180)
    ok = r.returncode == 0 and os.path.exists(BPK_FILE)
    if ok:
        print(f"[PASS] cmd/bp 打包成功（{os.path.getsize(BPK_FILE)} 字节，含许可证公钥）")
    else:
        print(f"[FAIL] cmd/bp 打包失败：{r.stdout.decode('utf-8', errors='ignore')[-300:]}")
    return ok


def reset_license_db():
    """清理 demo-plugin 许可证记录（卸载保留授权语义——重装前重置为初始 demo 态）。"""
    import psycopg2
    env = {}
    with open(os.path.join(ROOT, ".env"), encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env[k.strip()] = v.strip().strip('"')
    try:
        conn = psycopg2.connect(host=env.get("POSTGRES_HOST", "localhost"), port=env.get("POSTGRES_PORT", "5432"),
                                user=env.get("POSTGRES_USER", "postgres"), password=env.get("POSTGRES_PASSWORD", ""),
                                dbname=env.get("POSTGRES_DB", "Blog"), connect_timeout=5)
        with conn.cursor() as cur:
            cur.execute("DELETE FROM plugin_licenses WHERE plugin_id = 'demo-plugin'")
        conn.commit()
        conn.close()
    except Exception:
        pass  # DB 清理失败静默（首次运行无记录）


def issue_license(exp_ts, out_path):
    """license-issue 签发许可证（go run 编译一次；exp_ts 为 Unix 秒）。"""
    r = subprocess.run(["go", "run", "./cmd/license-issue", "sign",
                        "-sub", "plugin:demo-plugin", "-licensee", "冒烟站点",
                        "-edition", "pro", "-features", "demo_pro",
                        "-exp", str(exp_ts),
                        "-key", os.path.join(KEYS_DIR, "private.pem"),
                        "-out", out_path],
                       cwd=ROOT, capture_output=True, timeout=180)
    return r.returncode == 0 and os.path.exists(out_path)


def plugin_pids():
    """查找 demo 插件进程 PID（进程名 plugin.exe + 命令行含 demo-plugin，排除 PowerShell 自身）。
    Windows PowerShell 输出 GBK 编码，用 bytes 手动解码避免 UnicodeDecodeError。"""
    cmd = ["powershell", "-NoProfile", "-Command",
           "Get-CimInstance Win32_Process | Where-Object {$_.Name -eq 'plugin.exe' -and $_.CommandLine -like '*demo-plugin*'} | Select-Object -ExpandProperty ProcessId"]
    try:
        out = subprocess.run(cmd, capture_output=True, timeout=20).stdout  # bytes
        text = out.decode("gbk", errors="ignore")
    except Exception:
        return []
    return [int(x) for x in text.split() if x.strip().isdigit()]


def kill_plugin():
    """强制结束 demo 插件进程（模拟崩溃；bytes 输出避免 GBK 解码异常）。"""
    for pid in plugin_pids():
        subprocess.run(["taskkill", "/F", "/PID", str(pid)], capture_output=True)


def plugin_log():
    """读取插件日志（无文件返回空串）。"""
    try:
        with open(PLUGIN_LOG, encoding="utf-8", errors="ignore") as f:
            return f.read()
    except OSError:
        return ""


def assert_true(cond, msg):
    global PASS, FAIL
    if cond:
        PASS += 1
        print(f"[PASS] {msg}")
    else:
        FAIL += 1
        print(f"[FAIL] {msg}")


def installed_item(token):
    """我的插件中 demo-plugin 实例（不存在返回 None）。"""
    r = call("GET", "/admin/plugins", token=token)
    for inst in (r.get("data") or {}).get("items", []):
        if inst["plugin_id"] == "demo-plugin":
            return inst
    return None


def wait_ping(token, timeout=20):
    """等待插件 API 可用（进程拉起/重启后轮询）。"""
    end = time.time() + timeout
    while time.time() < end:
        if "pong" in call("GET", "/plugins/demo-plugin/ping", token=token, raw=True):
            return True
        time.sleep(0.5)
    return False


def main():
    token = login("admin@yueyan.site")
    assert token, "超管登录失败"

    # ---------- 1. 前置清理（卸载残留 + 清许可证记录 + 杀进程 + 清理旧包） ----------
    inst = installed_item(token)
    if inst:
        call("DELETE", f"/admin/plugins/{inst['id']}", token=token)
    reset_license_db()
    kill_plugin()
    if os.path.exists(BPK_FILE):
        os.remove(BPK_FILE)
    time.sleep(1)

    # ---------- 2. cmd/bp 打包 → 本地上传安装（M3.4 完整链路） ----------
    assert_true(build_bpk(), "cmd/bp 打包 .bpk（checksums 生成）")
    r = upload_bpk(token, BPK_FILE)
    assert_true(r.get("code") == 0, f".bpk 上传安装（code={r.get('code')} {r.get('message')}）")
    assert_true(wait_ping(token), "安装即启用：插件进程拉起（自定义 API 可访问）")
    inst = installed_item(token)
    assert_true(inst and inst["state"] == "running", f"安装后状态 running（实际 {inst and inst['state']}）")

    # ---------- 2.5 许可证链路（M3.5：demo 模式 → 签发 → 激活 → pro → 篡改/过期拒绝） ----------
    inst = installed_item(token)
    # 2.5.1 demo 模式：付费插件未激活 → pro-status=false
    body = call("GET", "/plugins/demo-plugin/pro-status", token=token, raw=True)
    assert_true('"pro":false' in body and '"edition":"free"' in body, "付费插件 demo 模式（pro-status=false）")
    # 2.5.2 签发许可证（有效期 1 年）
    exp_future = int(time.time()) + 365 * 86400
    assert_true(issue_license(exp_future, LICENSE_JWT), "license-issue 签发许可证")
    with open(LICENSE_JWT, encoding="utf-8") as f:
        jwt_valid = f.read()
    # 2.5.3 激活 → 自动重启进程 → pro-status=true
    r = call("POST", f"/admin/plugins/{inst['id']}/license", {"license_jwt": jwt_valid}, token=token)
    assert_true(r.get("code") == 0, f"许可证激活（code={r.get('code')} {r.get('message')}）")
    time.sleep(2)
    body = call("GET", "/plugins/demo-plugin/pro-status", token=token, raw=True)
    assert_true('"pro":true' in body, "激活后 pro 功能放行（pro-status=true）")
    # 2.5.4 篡改许可证（edition 改为 free 再激活 → 验签拒绝 4004）
    tampered = json.loads(jwt_valid)
    tampered["edition"] = "free"
    r = call("POST", f"/admin/plugins/{inst['id']}/license", {"license_jwt": json.dumps(tampered)}, token=token, expect=4004)
    assert_true("签名" in (r.get("message") or ""), "篡改许可证验签拒绝（4004）")
    # 2.5.5 过期许可证（exp 早于 8 天前，超过 7 天宽限期）：激活成功但状态 degraded（功能锁定）
    exp_past = int(time.time()) - 8 * 86400
    expired_jwt = os.path.join(KEYS_DIR, "license-expired.jwt")
    assert_true(issue_license(exp_past, expired_jwt), "签发过期许可证（exp 过去）")
    with open(expired_jwt, encoding="utf-8") as f:
        jwt_expired = f.read()
    r = call("POST", f"/admin/plugins/{inst['id']}/license", {"license_jwt": jwt_expired}, token=token)
    assert_true(r.get("code") == 0, "过期许可证可激活（宽限期语义）")
    r = call("GET", f"/admin/plugins/{inst['id']}/license", token=token)
    lic = (r.get("data") or {}).get("license", {})
    assert_true(lic.get("degraded") is True, f"过期许可证状态 degraded（实际 {lic.get('degraded')}）")
    time.sleep(2)
    body = call("GET", "/plugins/demo-plugin/pro-status", token=token, raw=True)
    assert_true('"pro":false' in body and '"degraded":true' in body, "超宽限期功能锁定（pro=false, degraded=true）")
    # 2.5.6 重新激活有效许可证恢复 pro（后续钩子验证不受影响）
    r = call("POST", f"/admin/plugins/{inst['id']}/license", {"license_jwt": jwt_valid}, token=token)
    assert_true(r.get("code") == 0, "重新激活有效许可证恢复 Pro")
    time.sleep(2)

    # ---------- 3. 同步钩子拦截：标题含 [demo] 发帖 → 2003 校验拒绝 ----------
    r = call("POST", "/posts", {
        "content_type": "text", "title": "[demo] 拦截测试", "content": "进程外拦截验证",
        "visibility": "public", "status": "published",
    }, token=token, expect=2003)
    assert_true("演示插件拦截" in (r.get("message") or ""), "同步钩子 post.before_publish 拦截生效（[demo] 标题被拒）")

    # ---------- 4. 异步钩子：草稿 → 发布 → after_publish 插件日志 ----------
    r = call("POST", "/posts", {
        "content_type": "text", "title": "M3.3 冒烟验证贴", "content": "进程外插件异步钩子验证",
        "visibility": "public", "status": "draft",
    }, token=token)
    post_id = (r.get("data") or {}).get("id")
    call("POST", f"/posts/{post_id}/publish", token=token)
    time.sleep(1.5)
    assert_true("after_publish" in plugin_log(), "异步钩子 post.after_publish 触发（插件日志有记录）")

    # ---------- 5. 同步钩子：搜索触发 search.query ----------
    call("GET", "/search?q=" + urllib.parse.quote("冒烟"), token=token)
    time.sleep(0.5)
    assert_true("search.query" in plugin_log(), "同步钩子 search.query 触发（插件日志有记录）")

    # ---------- 6. 自定义 API 代理 ----------
    body = call("GET", "/plugins/demo-plugin/ping", token=token, raw=True)
    assert_true('"pong":true' in body, "自定义 API 代理（GET /plugins/demo-plugin/ping）")

    # ---------- 7. 崩溃自愈：kill 进程 → 1s 退避自动重启 ----------
    before = len(plugin_pids())
    assert_true(before >= 1, f"插件进程存在（数量 {before}）")
    kill_plugin()
    assert_true(wait_ping(token), "崩溃后自动重启（API 恢复可用）")

    # ---------- 8. 连续崩溃熔断：kill 5 次 → crashed + last_error ----------
    # 退避节奏：1s/2s/4s/8s 后重启，第 5 次熔断不再重启（累计约 30s）
    kill_plugin()
    time.sleep(4)
    kill_plugin()
    time.sleep(6)
    kill_plugin()
    time.sleep(9)
    kill_plugin()
    time.sleep(13)
    kill_plugin()
    time.sleep(2)
    inst = installed_item(token)
    assert_true(inst and inst["state"] == "crashed", f"连续崩溃后熔断 crashed（实际 {inst and inst['state']}）")
    assert_true(inst and "熔断" in (inst.get("last_error") or ""), "last_error 记录熔断原因")

    # ---------- 9. 熔断后手动恢复（disabled → running 重新拉起） ----------
    call("PUT", f"/admin/plugins/{inst['id']}/state", {"state": "disabled"}, token=token)
    inst = installed_item(token)
    assert_true(inst["state"] == "disabled", "熔断后可手动停用（disabled）")
    call("PUT", f"/admin/plugins/{inst['id']}/state", {"state": "running"}, token=token)
    assert_true(wait_ping(token), "手动重新启用成功（进程重新拉起）")
    inst = installed_item(token)
    assert_true(inst["state"] == "running", "恢复后状态 running")

    # ---------- 10. 禁用（进程退出）→ 卸载 ----------
    call("PUT", f"/admin/plugins/{inst['id']}/state", {"state": "disabled"}, token=token)
    time.sleep(1)
    assert_true(len(plugin_pids()) == 0, "禁用后插件进程退出")
    call("DELETE", f"/admin/plugins/{inst['id']}", token=token)
    inst = installed_item(token)
    assert_true(inst is None or inst["state"] == "uninstalled", "卸载完成（软删标记）")

    # ---------- 汇总 ----------
    print(f"\n========== M3.3 插件进程外化冒烟：{PASS} 通过 / {FAIL} 失败 ==========")
    sys.exit(1 if FAIL else 0)


if __name__ == "__main__":
    main()
