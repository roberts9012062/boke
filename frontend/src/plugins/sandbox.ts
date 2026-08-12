// frontend/src/plugins/sandbox.ts
// iframe 沙箱（M3.6）：复杂插件前端以静态站点随 .bpk 提供（frontend/index.html 等），
// 挂载到 /plugin-assets/{id}/frontend/。
// 通信协议（docs/plugin-dev-guide.md 8.2）：
//   主站 → iframe：{type:"init", user}（用户基础信息，不含密钥）
//   iframe → 主站：{type:"api", pluginId, requestId, method, path, body} → 主站代理转发
//   主站 → iframe：{type:"api-result", requestId, status, body}
// 差异记录：文档的短期 token（1 小时）签发接口后置——当前以 postMessage 代理为主
//（iframe 无直接网络权限，API 调用经主站转发并复用登录凭证）。
import type { PluginUser } from "./loader";

// mountSandbox 挂载 iframe 沙箱。
// 参数：pluginId 插件 ID；container 容器；entry 静态站点入口（如 index.html）；user 用户信息。
// 返回：清理函数（移除 iframe 与消息监听）。
export function mountSandbox(
  pluginId: string,
  container: HTMLElement,
  entry: string,
  user: PluginUser | null,
): () => void {
  const iframe = document.createElement("iframe");
  iframe.src = `/plugin-assets/${pluginId}/frontend/${entry}`;
  iframe.style.width = "100%";
  iframe.style.border = "0";
  iframe.setAttribute("data-plugin-sandbox", pluginId);
  container.appendChild(iframe);

  // postMessage 代理：插件 API 请求 → 主站转发 → 回传结果
  const onMessage = (event: MessageEvent) => {
    const msg = event.data as { type?: string; pluginId?: string; requestId?: string; method?: string; path?: string; body?: unknown };
    if (!msg || msg.type !== "api" || msg.pluginId !== pluginId) {
      return;
    }
    void proxyApi(pluginId, msg).then((result) => {
      iframe.contentWindow?.postMessage({ type: "api-result", requestId: msg.requestId, ...result }, "*");
    });
  };
  window.addEventListener("message", onMessage);

  // 加载完成后下发用户信息
  iframe.addEventListener("load", () => {
    iframe.contentWindow?.postMessage({ type: "init", user }, "*");
  });

  return () => {
    window.removeEventListener("message", onMessage);
    iframe.remove();
  };
}

// proxyApi 转发插件 API 调用（复用主站登录凭证，路径限定插件代理域）。
async function proxyApi(
  pluginId: string,
  msg: { method?: string; path?: string; body?: unknown },
): Promise<{ status: number; body: string }> {
  try {
    const res = await fetch(`/api/v1/plugins/${pluginId}${msg.path ?? "/"}`, {
      method: msg.method || "GET",
      headers: { "Content-Type": "application/json" },
      body: msg.body !== undefined ? JSON.stringify(msg.body) : undefined,
    });
    return { status: res.status, body: await res.text() };
  } catch (err) {
    return { status: 500, body: JSON.stringify({ error: String(err) }) };
  }
}
