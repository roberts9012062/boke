# scripts/publish-release-helper.py
# publish-plugin-release.sh 的辅助步骤：查找/创建 Release + 清理同名旧资产 + 上传资产。
# 环境变量：GITHUB_TOKEN（.env 由外层脚本加载后导出）。
# 参数：repo tag asset_name asset_path；输出：下载直链（stdout）；
# 失败输出错误到 stderr 并以非零码退出（Windows 原生 curl 不识别 MSYS 路径，统一用 urllib）。
import json
import os
import sys
import urllib.request

API = "https://api.github.com"
UPLOADS = "https://uploads.github.com"


def github(method: str, url: str, body: dict | None = None, data: bytes | None = None) -> tuple[int, object]:
    """调用 GitHub API（返回状态码与解析后的 JSON；404 返回 (404, {})）。"""
    req = urllib.request.Request(url, method=method)
    req.add_header("Authorization", "Bearer " + os.environ["GITHUB_TOKEN"])
    req.add_header("Accept", "application/vnd.github+json")
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        req.add_header("Content-Type", "application/json")
    if data is not None:
        req.add_header("Content-Type", "application/octet-stream")
    try:
        with urllib.request.urlopen(req, data) as resp:
            payload = json.loads(resp.read().decode("utf-8"))
            return resp.status, payload
    except urllib.error.HTTPError as e:
        payload = {}
        try:
            payload = json.loads(e.read().decode("utf-8"))
        except Exception:
            pass
        return e.code, payload


def main() -> int:
    repo, tag, asset_name, asset_path = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
    # 查找已有 Release（by tag；404=不存在）
    status, release = github("GET", f"{API}/repos/{repo}/releases/tags/{tag}")
    if status == 404:
        status, release = github("POST", f"{API}/repos/{repo}/releases", {
            "tag_name": tag,
            "name": f"Plugins {tag}",
            "body": "插件安装包（.bpk，已签名）。",
        })
        if status != 201:
            print(f"创建 Release 失败：HTTP {status} {release}", file=sys.stderr)
            return 1
    elif status != 200:
        print(f"查询 Release 失败：HTTP {status} {release}", file=sys.stderr)
        return 1
    release_id = release["id"]
    # 清理同名旧资产（幂等替换）
    status, assets = github("GET", f"{API}/repos/{repo}/releases/{release_id}/assets")
    if status == 200:
        for asset in assets:
            if asset["name"] == asset_name:
                del_status, _ = github("DELETE", f"{API}/repos/{repo}/releases/assets/{asset['id']}")
                if del_status != 204:
                    print(f"旧资产删除失败：HTTP {del_status}", file=sys.stderr)
                    return 1
    # 上传资产（二进制流）
    with open(asset_path, "rb") as f:
        data = f.read()
    status, uploaded = github(
        "POST",
        f"{UPLOADS}/repos/{repo}/releases/{release_id}/assets?name={asset_name}",
        data=data,
    )
    if status != 201:
        print(f"上传失败：HTTP {status} {uploaded}", file=sys.stderr)
        return 1
    print(uploaded["browser_download_url"])
    return 0


if __name__ == "__main__":
    sys.exit(main())

