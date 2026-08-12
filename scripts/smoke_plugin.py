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
BPK_FILE = os.path.join(ROOT, "dist", "demo-plugin-0.2.0-windows-amd64.bpk")
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


def upload_bpk(token, bpk_path, upgrade=False):
    """multipart 上传 .bpk（urllib 手写 multipart body；?upgrade=1 本地升级通道）。"""
    with open(bpk_path, "rb") as f:
        content = f.read()
    boundary = "----bpk" + uuid.uuid4().hex
    head = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="file"; filename="{os.path.basename(bpk_path)}"\r\n'
        "Content-Type: application/octet-stream\r\n\r\n"
    ).encode()
    tail = f"\r\n--{boundary}--\r\n".encode()
    path = BASE + "/admin/plugins/upload" + ("?upgrade=1" if upgrade else "")
    req = urllib.request.Request(path, data=head + content + tail, method="POST")
    req.add_header("Content-Type", f"multipart/form-data; boundary={boundary}")
    req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read().decode())
    except urllib.error.HTTPError as e:
        return json.loads(e.read().decode())


def call_public(path):
    """公开端点请求（无凭证，返回原始文本；M3.6 assets 静态服务验证）。"""
    try:
        with urllib.request.urlopen("http://localhost:8080" + path, timeout=10) as r:
            return r.read().decode()
    except urllib.error.HTTPError as e:
        return e.read().decode()


def login(account, password="Yueyan2026"):
    r = call("POST", "/auth/login", {"account": account, "password": password})
    token = (r.get("data") or {}).get("access_token", "")
    if not token:
        print("[FAIL] 登录失败")
    return token


def build_bpk():
    """cmd/bp 打包 demo 插件（scripts/build-demo-bpk.sh → dist/demo-plugin-0.2.0-windows-amd64.bpk）。"""
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


def build_bpk_version(version, out_path):
    """cmd/bp 打包指定版本（升级链路验证用：0.2.0 新版本）。"""
    r = subprocess.run(["go", "run", "./cmd/bp", "pack",
                        "-plugin", "cmd/demo-plugin/yueyan-plugin.json",
                        "-bin", "dist/demo-plugin/plugin.bin",
                        "-pubkey", "data/demo-keys/public.pem",
                        "-frontend", "cmd/demo-plugin/frontend",
                        "-os", "windows", "-arch", "amd64",
                        "-version", version,
                        "-out", "dist"],
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


def installed_item(token, plugin_id="demo-plugin"):
    """我的插件中指定插件实例（不存在返回 None）。"""
    r = call("GET", "/admin/plugins", token=token)
    for inst in (r.get("data") or {}).get("items", []):
        if inst["plugin_id"] == plugin_id:
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

    # ---------- 2.3 前端扩展资产静态服务（M3.6：/plugin-assets） ----------
    body = call_public("/plugin-assets/demo-plugin/frontend/manifest.json")
    assert_true('"post.footer"' in body, "assets 静态服务：frontend/manifest.json 可访问（含 post.footer 声明）")
    body = call_public("/plugin-assets/demo-plugin/frontend/index.js")
    assert_true("export default function register" in body, "assets 静态服务：frontend/index.js 可访问（ESM register 契约）")
    body = call_public("/plugin-assets/demo-plugin/../../etc/passwd")
    assert_true("资源不存在" in body or "page not found" in body, "assets 路径穿越拒绝（/plugin-assets/../ 逃逸 404）")

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

    # ---------- 2.7 支付渠道（M3.9：配置签发私钥 → 订单 → 模拟支付 → 服务端签发 → 自动激活） ----------
    priv_pem = open(os.path.join(KEYS_DIR, "private.pem"), encoding="utf-8").read()
    r = call("PUT", "/admin/plugins/issuer-key", {"private_key_pem": priv_pem}, token=token)
    assert_true(r.get("code") == 0, "配置服务端签发私钥（AES 加密存储）")
    r = call("POST", f"/admin/plugins/{inst['id']}/orders", {"price": 99}, token=token)
    order_id = (r.get("data") or {}).get("order_id")
    assert_true(order_id and order_id > 0, "创建购买订单（pending）")
    r = call("POST", f"/admin/plugin-orders/{order_id}/pay", token=token)
    pay_data = r.get("data") or {}
    assert_true(r.get("code") == 0 and pay_data.get("state") == "paid" and pay_data.get("license_jwt"),
                "模拟支付成功：服务端签发许可证")
    time.sleep(2)
    body = call("GET", "/plugins/demo-plugin/pro-status", token=token, raw=True)
    assert_true('"pro":true' in body, "支付后自动激活：pro 功能放行（pro-status=true）")
    # 幂等：重复支付直接返回已签发
    r = call("POST", f"/admin/plugin-orders/{order_id}/pay", token=token)
    assert_true((r.get("data") or {}).get("state") == "paid", "支付幂等：重复支付直接返回已签发")

    # ---------- 2.8 SEO 插件链路（M4.1：seo-optimizer 发帖 SEO 面板通道——主进程落库/短链） ----------
    seo_bpk = os.path.join(ROOT, "dist", "seo-optimizer-1.2.0-windows-amd64.bpk")
    if os.path.exists(seo_bpk):
        # 安装 seo-optimizer（免费插件无公钥——覆盖 NULL pubkey 兼容；预置已装则先卸载）
        if installed_item(token, "seo-optimizer"):
            call("DELETE", f"/admin/plugins/{installed_item(token, 'seo-optimizer')['id']}", token=token)
        r = upload_bpk(token, seo_bpk)
        assert_true(r.get("code") == 0, f"SEO 插件上传安装（code={r.get('code')}）")
        seo_inst = installed_item(token, "seo-optimizer")
        assert_true(seo_inst and seo_inst["state"] == "running", "SEO 插件安装后 running（免费插件无公钥兼容）")
        # 发帖带 SEO 字段 → seo_meta 落库（标题/别名）
        # 发帖带 SEO 字段 → seo_meta 落库（标题/别名；alias 时间戳唯一化防跨轮冲突）
        seo_alias = "seo-smoke-" + str(int(time.time()))
        r = call("POST", "/posts", {
            "content_type": "text", "content": "SEO 插件链路冒烟验证",
            "visibility": "public", "status": "published",
            "seo": {"seo_title": "SEO 冒烟标题", "seo_description": "SEO 冒烟描述",
                    "url_alias": seo_alias, "robots": "index, follow"},
        }, token=token)
        seo_post_id = (r.get("data") or {}).get("id")
        assert_true(seo_post_id and seo_post_id > 0, "发帖带 SEO 字段成功")
        # 短链 /p/{alias} → 302 到 /posts/{id}（禁用重定向跟随——/posts 是前端路由，
        # 后端跟随会 404；NoRedirect 下 302 作为 HTTPError 抛出，从 e 取状态与 Location）
        import urllib.request as _ur

        class _NoRedirect(_ur.HTTPRedirectHandler):
            def redirect_request(self, *args, **kwargs):
                return None  # 不跟随：302 由 http_error_default 抛为 HTTPError

        _opener = _ur.build_opener(_NoRedirect)
        try:
            alias_resp = _opener.open(f"http://localhost:8080/p/{seo_alias}", timeout=10)
        except _ur.HTTPError as e:
            alias_resp = e  # 302 重定向在此抛出（未跟随）
        location = alias_resp.headers.get("Location", "")
        alias_ok = alias_resp.code == 302 and f"/posts/{seo_post_id}" in location
        alias_resp.close()
        assert_true(alias_ok, f"URL 别名短链 /p/{seo_alias} → 302 /posts/{seo_post_id}（实际 {alias_resp.code} {location}）")
        # 详情返回 SEO（标题/别名/收录）
        body = call("GET", f"/posts/{seo_post_id}", token=token, raw=True)
        detail = json.loads(body)
        seo_out = (detail.get("data") or {}).get("seo") or {}
        assert_true(seo_out.get("title") == "SEO 冒烟标题" and seo_out.get("url_alias") == seo_alias
                    and seo_out.get("robots") == "index, follow",
                    f"详情 SEO 输出（标题/别名/收录；实际 {seo_out}）")
        # AI 辅助接口（M4.1：模型列表 + 生成参数校验——经插件 API → 数据服务链路）
        body = call("GET", "/plugins/seo-optimizer/ai/models", token=token, raw=True)
        ai_models = json.loads(body)
        assert_true("configured" in ai_models and "models" in ai_models,
                    f"AI 模型接口可用（configured/models 字段；实际 {body[:100]}）")
        body = call("POST", "/plugins/seo-optimizer/ai/generate",
                    {"model": "", "prompt": "x", "content": "x"}, token=token, raw=True)
        assert_true("参数错误" in body, f"AI 生成参数校验（空模型拒绝；实际 {body[:80]}）")
        # 卸载还原（compose 面板由插件渲染——卸载后插件扩展消失；后端通道闲置）
        call("DELETE", f"/admin/plugins/{seo_inst['id']}", token=token)
        assert_true(installed_item(token, "seo-optimizer") is None, "SEO 插件卸载（软删标记）")
    else:
        print("[跳过] SEO 插件 bpk 不存在（未构建）")

    # ---------- 2.6 一键升级（插件后置：上传 0.2.0 包 ?upgrade=1 → 版本替换 + 进程重启） ----------
    bpk_020 = os.path.join(ROOT, "dist", "demo-plugin-0.2.0-windows-amd64.bpk")
    assert_true(build_bpk_version("0.2.0", bpk_020), "cmd/bp 打包 0.2.0 新版本")
    r = upload_bpk(token, bpk_020, upgrade=True)
    assert_true(r.get("code") == 0, f"本地上传升级（?upgrade=1，code={r.get('code')} {r.get('message')}）")
    assert_true(wait_ping(token), "升级后插件进程重启（API 可用）")
    inst = installed_item(token)
    assert_true(inst and inst["version"] == "0.2.0", f"升级后版本 0.2.0（实际 {inst and inst['version']}）")
    # 升级后许可证保持（plugin_licenses 未动）+ pro 功能仍可用
    body = call("GET", "/plugins/demo-plugin/pro-status", token=token, raw=True)
    assert_true('"pro":true' in body, "升级后 Pro 功能保持（许可证不受影响）")
    # 更新检查接口（demo 无 Release → 空列表；接口可用性验证）
    r = call("GET", "/admin/plugin-updates", token=token)
    assert_true(r.get("code") == 0 and isinstance((r.get("data") or {}).get("items"), list),
                "更新检查接口可用（items 列表）")

    # ---------- 2.7 插件沙箱短期令牌（插件后置：1 小时，直接调用代理 API） ----------
    r = call("POST", "/plugin-sandbox-token", token=token)
    st = (r.get("data") or {})
    assert_true(r.get("code") == 0 and st.get("token"), "短期令牌签发（1 小时）")
    body = call("GET", "/plugins/demo-plugin/ping", token=st.get("token", ""), raw=True)
    assert_true('"pong":true' in body, "短期令牌可直接调用插件代理 API（ping）")

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

    # ---------- 6.3 插件设置链路（M3.7：schema 聚合 → 保存过滤 → 即时生效 → 重启保持） ----------
    inst = installed_item(token)
    assert_true(inst is not None, "设置链路：获取插件实例")
    inst_id = inst["id"]
    # 详情：schema 来自进程 Info 上报（运行中优先），含 3 个设置项声明
    r = call("GET", f"/admin/plugins/{inst_id}", token=token)
    schema = r["data"]["plugin"]["settings_schema"] or []
    keys = sorted(f["key"] for f in schema)
    assert_true(keys == ["greeting", "show_badge", "theme"], f"详情接口：进程上报 schema 聚合（实际 {keys}）")
    # 保存配置：合法键保存 + 未声明键被过滤（防任意键注入）
    r = call("PUT", f"/admin/plugins/{inst_id}/config", token=token,
             body={"values": {"greeting": "你好，月言", "show_badge": "on", "theme": "dark", "evil_key": "注入"}})
    saved = r["data"]["config"]
    assert_true(saved.get("greeting") == "你好，月言" and "evil_key" not in saved,
                f"保存配置：schema 过滤（实际 {saved}）")
    # 即时生效：插件进程经 SetConfig 下发，/settings API 返回新值
    body = call("GET", "/plugins/demo-plugin/settings", token=token, raw=True)
    cfg = json.loads(body)
    assert_true(cfg.get("greeting") == "你好，月言" and cfg.get("theme") == "dark",
                f"保存后即时生效：插件 /settings 返回新配置（实际 {cfg}）")
    # 重启后保持：停用再启用（Start 激活后自动下发配置）
    call("PUT", f"/admin/plugins/{inst_id}/state", token=token, body={"state": "disabled"})
    call("PUT", f"/admin/plugins/{inst_id}/state", token=token, body={"state": "running"})
    assert_true(wait_ping(token), "设置链路：停用再启用后插件恢复")
    body = call("GET", "/plugins/demo-plugin/settings", token=token, raw=True)
    cfg = json.loads(body)
    assert_true(cfg.get("greeting") == "你好，月言", f"重启后配置保持（Start 激活下发，实际 {cfg}）")
    # 回读接口：DB config 一致
    r = call("GET", f"/admin/plugins/{inst_id}/config", token=token)
    assert_true(r["data"]["config"].get("greeting") == "你好，月言", "配置回读：DB config JSONB 一致")

    # ---------- 6.5 数据服务链路（M3.8：声明 data.read → broker 授权 → 脱敏数据） ----------
    body = call("GET", "/plugins/demo-plugin/data-demo", token=token, raw=True)
    data = json.loads(body)
    assert_true(data.get("authorized") is True and bool(data.get("user", {}).get("nickname")),
                f"数据服务：授权插件可查脱敏数据（实际 {body[:120]}）")
    assert_true(data.get("settings_keys", 0) > 0, "数据服务：站点公开设置白名单键下发")
    # 评论保存 → comment.after_save 异步钩子（M3.8 补全接线 + 双 dispatch 修复：应恰好 1 次）
    comment_before = plugin_log().count("comment.after_save")
    call("POST", f"/posts/{post_id}/comments", token=token, body={"content": "冒烟评论-数据服务验证"})
    time.sleep(1.5)
    comment_cnt = plugin_log().count("comment.after_save") - comment_before
    assert_true(comment_cnt == 1, f"comment.after_save 恰好触发 1 次（双 dispatch 修复；实际 {comment_cnt} 次）")

    # ---------- 6.7 钩子扩展（M3.9：content.render 改写 / api.middleware 拦截 / 流式通道） ----------
    # content.render：详情 API 正文被插件改写（渲染管道）
    body = call("GET", f"/posts/{post_id}", token=token, raw=True)
    detail = json.loads(body)
    detail_content = ((detail.get("data") or {}).get("content") or "")
    assert_true("由演示插件渲染" in detail_content, "content.render：插件改写帖子正文（渲染管道）")
    # api.middleware：DELETE 帖子被插件拦截（403 防误删演示）
    r = call("DELETE", f"/posts/{post_id}", token=token, expect=1003)
    assert_true("演示插件拦截" in (r.get("message") or ""), "api.middleware：DELETE 帖子被插件拦截（403）")
    # 流式通道（M3.9）：异步钩子仍触发（after_publish 走 stream 推送；进程存在说明通道建立成功）
    assert_true("after_publish" in plugin_log(), "流式钩子通道：异步事件仍触发（after_publish）")

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
