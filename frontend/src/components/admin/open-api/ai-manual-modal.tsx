// src/components/admin/open-api/ai-manual-modal.tsx
// AI 开发手册弹窗：按 Key 已授权的接口从目录生成 Markdown 手册，
// 供用户复制给 AI，AI 据此开发基于本站开放接口的浏览器插件应用。
"use client";

import { useMemo, useState } from "react";

import { Modal } from "@/components/ui/modal";
import type { CatalogEntry, OpenAPIKey } from "@/lib/api-openapi";

// formatDateTime 格式化「YYYY-MM-DD HH:mm」（过期/创建时间展示；纯函数）。
function formatDateTime(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// buildManual 生成 Markdown 手册（纯函数）。
// 内容：站点与认证说明 → 浏览器插件开发指引（MV3 host_permissions 跨域）→ 逐接口用法。
function buildManual(baseUrl: string, apiKey: OpenAPIKey, catalog: CatalogEntry[]): string {
  // 只保留该 Key 已授权的接口（按目录顺序）
  const authorized = catalog.filter((e) => apiKey.endpoints.includes(e.endpoint));
  const lines: string[] = [];

  lines.push("# 月言博客开放接口 · AI 开发手册");
  lines.push("");
  lines.push("你将基于以下开放接口，开发一个对接本博客的浏览器插件（Chrome Extension MV3）。");
  lines.push("");
  lines.push("## 一、基本信息");
  lines.push("");
  lines.push(`- 站点地址：${baseUrl}`);
  lines.push(`- 接口基础路径：${baseUrl}/api/v1/open`);
  lines.push(`- API Key：\`${apiKey.key}\``);
  lines.push(
    `- Key 有效期：${apiKey.expires_at ? `至 ${formatDateTime(apiKey.expires_at)} 到期` : "永久有效"}`,
  );
  lines.push("- 认证方式：所有请求必须携带请求头 `X-Api-Key`，缺失/无效/过期返回 401，未授权该接口返回 403");
  lines.push("- 响应格式：统一 JSON `{code, message, data, request_id}`，`code=0` 表示成功，业务数据在 `data` 字段");
  lines.push("");
  lines.push("## 二、浏览器插件开发指引（MV3）");
  lines.push("");
  lines.push("1. 跨域：在 `manifest.json` 的 `host_permissions` 中声明站点地址即可跨域请求：");
  lines.push("   ```json");
  lines.push('   "host_permissions": ["' + baseUrl + "/*\"]");
  lines.push("   ```");
  lines.push("2. 请求封装（content script 或 background service worker 中）：");
  lines.push("   ```js");
  lines.push(`   const API_BASE = "${baseUrl}/api/v1/open";`);
  lines.push(`   const API_KEY = "${apiKey.key}";`);
  lines.push("   async function callOpenApi(path, params) {");
  lines.push("     const query = params ? '?' + new URLSearchParams(params).toString() : '';");
  lines.push("     const res = await fetch(API_BASE + path + query, {");
  lines.push("       headers: { 'X-Api-Key': API_KEY },");
  lines.push("     });");
  lines.push("     const body = await res.json();");
  lines.push("     if (body.code !== 0) throw new Error(body.message);");
  lines.push("     return body.data;");
  lines.push("   }");
  lines.push("   ```");
  lines.push("3. 请勿把 Key 硬编码进分发的插件包以外的地方；Key 泄露可在后台删除后重新生成。");
  lines.push("");
  lines.push(`## 三、已授权接口（共 ${authorized.length} 个）`);
  lines.push("");

  for (const entry of authorized) {
    lines.push(`### ${entry.name}（${entry.endpoint}）`);
    lines.push("");
    lines.push(`- ${entry.description}`);
    lines.push(`- 请求：\`${entry.method} ${entry.path}\``); // 路径含 :id 等路由参数
    if (entry.params.length > 0) {
      lines.push("- 参数：");
      for (const p of entry.params) {
        lines.push(
          `  - \`${p.name}\`（${p.location}，${p.type}${p.required ? "，必填" : "，可选"}）：${p.description}`,
        );
      }
    }
    // 调用示例（query 参数展开为对象；路径参数用占位值）
    const pathParams = entry.params.filter((p) => p.location === "path");
    const queryParams = entry.params.filter((p) => p.location === "query");
    const callPath = pathParams.reduce((acc, p) => acc.replace(`:${p.name}`, `1`), entry.path).replace("/api/v1/open", "");
    const args = queryParams.length > 0
      ? `, { ${queryParams.map((p) => `${p.name}: ${p.type === "integer" ? "1" : `"值"`}`).join(", ")} }`
      : "";
    lines.push("- 调用示例：`callOpenApi(\"" + callPath + "\"" + args + ")`");
    lines.push("");
  }

  lines.push("## 四、开发建议");
  lines.push("");
  lines.push("- 先用「站点信息」接口确认连通性，再按需调用内容接口做列表/详情渲染。");
  lines.push("- 列表接口均支持 `page`/`page_size` 分页，`data.items` 为数据数组，`data.total` 为总数。");
  lines.push("- 帖子列表的 `data.items[].post_kind` 区分说说（moment）与文章（article）。");
  lines.push("- 如需本手册未包含的接口，请联系站点管理员在后台重新生成 Key 并授权。");
  return lines.join("\n");
}

// AiManualModal AI 开发手册弹窗。
// 参数：open 是否打开；onClose 关闭回调；apiKey 凭证；catalog 完整目录（按 Key 过滤）。
export function AiManualModal({
  open,
  onClose,
  apiKey,
  catalog,
}: {
  open: boolean;
  onClose: () => void;
  apiKey: OpenAPIKey;
  catalog: CatalogEntry[];
}) {
  const [copied, setCopied] = useState<boolean>(false);

  // 手册内容（打开时按 Key + 目录生成；站点地址取当前页面 origin）
  const manual = useMemo(() => {
    const baseUrl = typeof window !== "undefined" ? window.location.origin : "";
    return buildManual(baseUrl, apiKey, catalog);
  }, [apiKey, catalog]);

  // 复制手册到剪贴板（复制成功 2 秒内显示“已复制”）
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(manual);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // 剪贴板不可用（非安全上下文等）：保持按钮原样，用户可手动选中文本复制
      setCopied(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="AI 开发手册" maxWidth="max-w-3xl">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-ink-3">
          已按该 Key 授权的 {apiKey.endpoints.length} 个接口生成手册，复制后发给 AI 即可开发浏览器插件
        </p>
        <button
          type="button"
          onClick={() => void handleCopy()}
          className={`shrink-0 rounded-full px-5 py-2 text-sm font-medium transition-colors ${
            copied ? "bg-accent-soft text-glow" : "bg-accent text-on-accent hover:opacity-90"
          }`}
        >
          {copied ? "已复制 ✓" : "复制手册"}
        </button>
      </div>
      {/* 手册内容（等宽字体滚动区域，便于阅读与手动复制） */}
      <pre className="mt-3 max-h-[60vh] overflow-auto whitespace-pre-wrap break-words rounded-lg border border-line bg-muted p-4 font-mono text-xs leading-relaxed text-ink-2">
        {manual}
      </pre>
    </Modal>
  );
}
