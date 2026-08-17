#!/usr/bin/env bash
# scripts/smoke-custom-page.sh
# 自定义页面 + 头部导航自定义 冒烟测试（幂等：结束时清理测试数据并恢复 site_name）。
#
# 用法：
#   ./scripts/smoke-custom-page.sh
#
# 覆盖链路：登录 → 创建草稿（前台 404）→ 发布（前台 200）→ slug 冲突 3001
#   → 保存 nav_links → meta 返回 nav → sitemap 收录 → 危险协议拒绝 → 清理恢复。
# 编码注意：Windows Git Bash 会把请求体中的中文按 GBK 发送，故请求体一律用
#   JSON \uXXXX 转义（ASCII 传输，后端解码还原 UTF-8）；断言统一用 ASCII 子串。
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE="http://localhost:8080/api/v1"
SLUG="smoke-page"
PASS=0
FAIL=0

# check 断言辅助：名称 + 结果（0 = 通过）。
check() {
  if [ "$2" -eq 0 ]; then
    echo "[通过] $1"; PASS=$((PASS + 1))
  else
    echo "[失败] $1"; FAIL=$((FAIL + 1))
  fi
}

# ok 输出断言（grep 匹配即通过）。
ok() {
  if echo "$2" | grep -q "$3"; then check "$1" 0; else check "$1" 1; fi
}

echo "=== 自定义页面冒烟测试 ==="

# 1. 管理员登录
LOGIN=$(curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" \
  -d '{"account":"admin@yueyan.site","password":"Yueyan2026"}')
TOKEN=$(echo "$LOGIN" | python -c "import json,sys;print(json.load(sys.stdin)['data']['access_token'])" 2>/dev/null || echo "")
if [ -n "$TOKEN" ]; then check "管理员登录" 0; else check "管理员登录" 1; fi

AUTH="Authorization: Bearer $TOKEN"

# 2. 清理历史残留（幂等重跑保障）
PAGE_ID=$(curl -s "$BASE/admin/pages" -H "$AUTH" | python -c "
import json,sys
items=json.load(sys.stdin)['data']['items']
print(next((p['id'] for p in items if p['slug']=='$SLUG'), ''))" 2>/dev/null || echo "")
if [ -n "$PAGE_ID" ]; then
  curl -s -X DELETE "$BASE/admin/pages/$PAGE_ID" -H "$AUTH" > /dev/null
fi

# 3. 创建草稿页面（标题/正文用 ASCII 便于断言）
CREATE=$(curl -s -X POST "$BASE/admin/pages" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"slug\":\"$SLUG\",\"title\":\"Smoke Title\",\"content\":\"<p>smoke-content</p>\",\"content_format\":\"html\",\"description\":\"smoke desc\",\"status\":\"draft\"}")
PAGE_ID=$(echo "$CREATE" | python -c "import json,sys;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo "")
if [ -n "$PAGE_ID" ]; then check "创建草稿页面（id=$PAGE_ID）" 0; else check "创建草稿页面" 1; fi

# 4. 草稿前台不可见（HTTP 404）
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/pages/$SLUG")
if [ "$CODE" = "404" ]; then check "草稿前台 404 不可见" 0; else check "草稿前台 404 不可见（HTTP $CODE）" 1; fi

# 5. 更新为已发布
UPD=$(curl -s -X PUT "$BASE/admin/pages/$PAGE_ID" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"slug\":\"$SLUG\",\"title\":\"Smoke Title\",\"content\":\"<p>smoke-content-v2</p>\",\"content_format\":\"html\",\"description\":\"smoke desc\",\"status\":\"published\"}")
ok "更新为已发布" "$UPD" '"code":0'

# 6. 前台可见 + 内容正确
DETAIL=$(curl -s "$BASE/pages/$SLUG")
ok "前台读取已发布页面" "$DETAIL" 'smoke-content-v2'

# 7. slug 冲突（code 3001）
DUP=$(curl -s -X POST "$BASE/admin/pages" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"slug\":\"$SLUG\",\"title\":\"dup\",\"content\":\"x\",\"content_format\":\"html\",\"description\":\"\",\"status\":\"draft\"}")
ok "slug 重复返回 3001" "$DUP" '"code":3001'

# 8. 保存导航配置（站内 + 外链；label 用 ASCII「SmokeNav」断言）
NAV=$(curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"[{\"label\":\"SmokeNav\",\"url\":\"/pages/smoke-page\",\"new_tab\":false},{\"label\":\"ExtLink\",\"url\":\"https://example.com\",\"new_tab\":true}]"}')
ok "保存 nav_links 导航配置" "$NAV" '"code":0'

# 9. /meta 返回导航
META=$(curl -s "$BASE/meta")
ok "meta 接口返回自定义导航" "$META" 'SmokeNav'

# 10. sitemap 包含自定义页面
SITEMAP=$(curl -s "http://localhost:8080/sitemap.xml")
ok "sitemap 包含已发布页面" "$SITEMAP" "/pages/$SLUG"

# 11. 非法导航拒绝（javascript: 协议 → code 2003）
BADNAV=$(curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"[{\"label\":\"X\",\"url\":\"javascript:alert(1)\",\"new_tab\":false}]"}')
ok "危险协议导航被拒绝" "$BADNAV" '"code":2003'

# 12. 两级导航保存（一级纯分组 + 二级站内/外链）→ meta 返回 children
NAV2=$(curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"[{\"label\":\"Top\",\"url\":\"\",\"children\":[{\"label\":\"Sub\",\"url\":\"/pages/smoke-page\",\"new_tab\":false},{\"label\":\"Ext\",\"url\":\"https://example.com\",\"new_tab\":true}]},{\"label\":\"Home\",\"url\":\"/\"}]"}')
ok "两级导航保存" "$NAV2" '"code":0'
META2=$(curl -s "$BASE/meta")
ok "meta 返回二级菜单" "$META2" 'children'

# 13. 三级嵌套拒绝 + 二级空 URL 拒绝（2003）
NEST3=$(curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"[{\"label\":\"A\",\"children\":[{\"label\":\"B\",\"url\":\"/x\",\"children\":[{\"label\":\"C\",\"url\":\"/y\"}]}]}]"}')
ok "三级嵌套被拒绝" "$NEST3" '"code":2003'
EMPTYSUB=$(curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"[{\"label\":\"A\",\"children\":[{\"label\":\"B\",\"url\":\"\"}]}]"}')
ok "二级空地址被拒绝" "$EMPTYSUB" '"code":2003'

# 14. page 格式（AI 构建器整页文档）创建 → 前台读取含 DOCTYPE
PAGEFMT=$(curl -s -X POST "$BASE/admin/pages" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"slug":"smoke-page-ai","title":"AI Page","content":"<!DOCTYPE html><html><head><style>body{color:#333}</style></head><body><h1>smoke-ai-page</h1></body></html>","content_format":"page","description":"","status":"published"}')
ok "page 格式创建" "$PAGEFMT" '"code":0'
AID=$(echo "$PAGEFMT" | python -c "import json,sys;print(json.load(sys.stdin)['data']['id'])" 2>/dev/null || echo "")
AIDETAIL=$(curl -s "$BASE/pages/smoke-page-ai")
ok "前台读取 page 格式" "$AIDETAIL" 'DOCTYPE'

# 15. AI generate 向后兼容（旧字段 prompt/content；无该模型时业务错误 2001，验证字段链路未破坏）
AILEGACY=$(curl -s -X POST "$BASE/admin/ai/generate" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"model":"__no_such_model__","prompt":"p","content":"c"}')
ok "AI 旧字段调用兼容" "$AILEGACY" '"code":2001'
# 多轮 messages 字段透传（同样以模型不存在验证请求解析与新链路）
AIMSG=$(curl -s -X POST "$BASE/admin/ai/generate" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"model":"__no_such_model__","messages":[{"role":"system","content":"s"},{"role":"user","content":"u"}],"max_tokens":8192}')
ok "AI 多轮字段透传" "$AIMSG" '"code":2001'

# ---------- 清理（清空导航 + 恢复站点名「月言」+ 删除测试页面） ----------
# site_name 用 \u6708\u8a00 转义写入（终端编码无关，还原为 UTF-8「月言」）
curl -s -X PUT "$BASE/admin/settings" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"nav_links":"","site_name":"\u6708\u8a00"}' > /dev/null
curl -s -X DELETE "$BASE/admin/pages/$PAGE_ID" -H "$AUTH" > /dev/null
if [ -n "$AID" ]; then
  curl -s -X DELETE "$BASE/admin/pages/$AID" -H "$AUTH" > /dev/null
fi
CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/pages/$SLUG")
if [ "$CODE" = "404" ]; then check "清理：删除页面后前台 404" 0; else check "清理：删除页面后前台 404（HTTP $CODE）" 1; fi

echo "=== 结果：通过 $PASS / 失败 $FAIL ==="
[ "$FAIL" -eq 0 ]
