// internal/pluginshared/shared.js
// 插件前端共享 SDK（E2 去重）：宿主经 /plugin-sdk/shared.js 同源分发，
// 插件页面/槽位模块以绝对路径 import（如 `import { escapeHtml } from "/plugin-sdk/shared.js"`）。
// 内容：HTML 转义、音频试播控制器、后台页骨架生成——此前在 qq/netease 两套前端各复制 3 份。
// 约束：本文件为手写 ESM（无构建链）；仅依赖浏览器标准 API；接口保持稳定（插件随包分发后不强制同步升级）。

// escapeHtml HTML 文本转义（防插件页面 XSS；所有 API 返回值渲染前必须经过它）。
export function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

// createAudioPreview 音频试播控制器（歌单列表“▶ 试听”按钮共用逻辑）。
// 用法：const preview = createAudioPreview()；preview.toggle(btn, msgEl, () => ctx.api.post("/song-url", { id }))。
// 参数（toggle）：btn 触发按钮；msgEl 可选错误提示元素；fetchUrl 本次取播放地址（返回 {url} 或 {error}）。
// 参数（构造）：labels 可选按钮文案 { idle, loading, playing }（缺省 ▶ / … / ⏸）。
// 返回：{ toggle(btn, msgEl, fetchUrl) 点击切换播放/停止；stop() 立即停止 }。
// 说明：同一控制器单实例单音频（新播放自动停上一首）；按钮态由控制器维护。
export function createAudioPreview(labels) {
  const idle = (labels && labels.idle) || "▶";
  const loading = (labels && labels.loading) || "…";
  const playing = (labels && labels.playing) || "⏸";
  let audio = null;
  const stop = () => {
    if (audio) {
      audio.pause();
      audio = null;
    }
  };
  const toggle = async (btn, msgEl, fetchUrl) => {
    if (audio && !audio.paused) {
      stop();
      btn.textContent = idle;
      return;
    }
    stop();
    btn.textContent = loading;
    btn.disabled = true;
    try {
      const r = await fetchUrl();
      btn.disabled = false;
      if (!r || r.error) {
        btn.textContent = idle;
        if (msgEl && r && r.error) msgEl.textContent = r.error;
        return;
      }
      audio = new Audio(r.url);
      audio.play().catch(() => {});
      audio.onended = () => {
        audio = null;
        btn.textContent = idle;
      };
      btn.textContent = playing;
    } catch (e) {
      btn.disabled = false;
      btn.textContent = idle;
      if (msgEl) msgEl.textContent = "播放失败：" + String(e);
    }
  };
  return { toggle, stop };
}

// pageChrome 后台插件页骨架（圆形品牌图标 + 标题 + 副标题 + Tab 导航 + 面板容器）。
// 参数：opts { color 主色；icon 图标字符；title 标题；subtitle 副标题；tabs [{key,label}] }。
// 返回：{ html 骨架 HTML（调用方 innerHTML）；rootQuery 面板选择器工厂 [data-panel-{key}]；
//        tabQuery Tab 按钮选择器工厂 [data-tab={key}]；bindTabs Tab 切换绑定（传容器元素） }。
export function pageChrome(opts) {
  const tabs = opts.tabs
    .map(
      (t, i) =>
        '<button type="button" data-tab="' + t.key + '" style="height:36px;padding:0 16px;font-size:13px;font-weight:600;color:' +
        (i === 0 ? opts.color : "var(--yy-text-2,#9aa6bc)") +
        ";background:transparent;border:none;border-bottom:2px solid " +
        (i === 0 ? opts.color : "transparent") +
        ';cursor:pointer">' + escapeHtml(t.label) + "</button>",
    )
    .join("");
  const panels = opts.tabs
    .map((t, i) => '<div style="margin-top:16px' + (i === 0 ? "" : ";display:none") + '" data-panel-' + t.key + "></div>")
    .join("");
  const html =
    '<div style="display:flex;align-items:center;gap:12px">' +
    '<span style="display:inline-flex;align-items:center;justify-content:center;width:38px;height:38px;border-radius:50%;background:' + opts.color + ';color:#fff;font-size:19px;font-weight:700">' + escapeHtml(opts.icon) + "</span>" +
    '<div><h1 style="font-size:18px;font-weight:700;color:var(--yy-text,#e8ecf4);line-height:1.3">' + escapeHtml(opts.title) + "</h1>" +
    '<p style="font-size:12px;color:var(--yy-text-2,#9aa6bc)">' + escapeHtml(opts.subtitle) + "</p></div></div>" +
    '<div style="margin-top:16px;display:flex;gap:8px;border-bottom:1px solid var(--yy-border,#2a3348)">' + tabs + "</div>" + panels;
  return {
    html,
    panel: (key) => "[data-panel-" + key + "]",
    bindTabs: (container) => {
      container.querySelectorAll("[data-tab]").forEach((b) => {
        b.addEventListener("click", () => {
          const name = b.dataset.tab;
          for (const t of opts.tabs) {
            const panel = container.querySelector("[data-panel-" + t.key + "]");
            if (panel) panel.style.display = t.key === name ? "block" : "none";
          }
          container.querySelectorAll("[data-tab]").forEach((x) => {
            const active = x.dataset.tab === name;
            x.style.color = active ? opts.color : "var(--yy-text-2,#9aa6bc)";
            x.style.borderBottomColor = active ? opts.color : "transparent";
          });
        });
      });
    },
  };
}

// cardStyle / hintStyle 常用内联样式（与宿主后台 --yy-* 设计变量对齐）。
export const cardStyle = "border-radius:12px;border:1px solid var(--yy-border,#2a3348);background:var(--yy-elevated,#fff)";
export const hintStyle = "font-size:12px;color:var(--yy-text-2,#9aa6bc)";

// createSandboxApi iframe 沙箱页面的受限 API 客户端（E1 接线；仅沙箱 HTML 页面使用）。
// 协议（与宿主 sandbox.ts 对应）：
//   宿主 → 页面：{type:"init", user, token}（加载完成即发）；页面 → 宿主：
//   {type:"api", pluginId, requestId, method, path, body} → 宿主代理转发 →
//   {type:"api-result", requestId, status, body}。
// 返回：Promise<SandboxApi>（收到 init 握手后 resolve；10s 未握手 reject）。
// 说明：沙箱页面与宿主同源（/plugin-assets 静态服务）但独立文档——无宿主 DOM/localStorage
//      访问权，凭证由宿主 postMessage 下发（token 为 1 小时短期令牌，过期提示管理员刷新页面）。
export function createSandboxApi() {
  return new Promise((resolve, reject) => {
    const pluginId = new URL(window.location.pathname, window.location.origin)
      .pathname.split("/")[2]; // /plugin-assets/{pluginId}/frontend/xxx.html
    const pending = new Map();
    let seq = 0;
    let initUser = null;
    let initToken = null;

    const onMessage = (event) => {
      if (event.origin !== window.location.origin) {
        return; // 来源校验：仅接受宿主同源消息
      }
      const msg = event.data;
      if (!msg || typeof msg !== "object") {
        return;
      }
      if (msg.type === "init") {
        initUser = msg.user || null;
        initToken = msg.token || null;
        resolve(makeApi());
        return;
      }
      if (msg.type === "api-result" && pending.has(msg.requestId)) {
        const { resolve: done } = pending.get(msg.requestId);
        pending.delete(msg.requestId);
        done({ status: msg.status, body: msg.body });
      }
    };
    window.addEventListener("message", onMessage);

    // 握手超时（宿主未在 10s 内下发 init——例如非沙箱环境误用）
    setTimeout(() => reject(new Error("沙箱握手超时（未收到宿主 init 消息）")), 10000);

    // makeApi 构造受限客户端（get/post；自动带短期 token 头）。
    function makeApi() {
      const call = (method, path, body) =>
        new Promise((done) => {
          const requestId = "sbx-" + ++seq + "-" + Date.now();
          pending.set(requestId, { resolve: done });
          window.parent.postMessage({ type: "api", pluginId, requestId, method, path, body }, window.location.origin);
        });
      return {
        user: initUser,
        token: initToken,
        async get(path) {
          return call("GET", path, undefined);
        },
        async post(path, body) {
          return call("POST", path, body);
        },
      };
    }
  });
}
