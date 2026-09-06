// src/app/admin/relay/page.tsx
// 后台「中继站」页（B-1' + 申请制 + 对接仪式，协议 v1.3）：
// 三态状态机——未连接（断线卫星）/ 已获证（待点火对接）/ 已对接（运行中）。
// key 隐藏保管：申请由后端代理完成，明文 key 永不回到前端。
"use client";

import { useCallback, useEffect, useState } from "react";

import { RelayRitual } from "@/components/relay-ritual";
import { ApiError } from "@/lib/api";
import { apiRelayApply, apiRelayClaim, apiRelayConfig, apiRelaySave } from "@/lib/api-relay";

// PageState 表单状态。
interface PageState {
  url: string;
  mode: string;
  category: string;
  retentionDays: number;
}

// ConnStatus 连接状态（由配置派生）：未连接 → 审核中 → 已获证 → 已对接。
type ConnStatus = "idle" | "reviewing" | "licensed" | "connected";

// emptyState 空表单。
const emptyState: PageState = { url: "", mode: "public", category: "", retentionDays: 7 };

// BrokenLink 断线卫星小场景（未连接态）：星球与卫星之间通讯中断。
function BrokenLink() {
  return (
    <div className="flex flex-col items-center py-6">
      <svg viewBox="0 0 240 70" className="w-64">
        <circle cx="18" cy="35" r="12" fill="none" stroke="#c2410c" strokeWidth="1.5" />
        <ellipse cx="18" cy="35" rx="19" ry="5" fill="none" stroke="#9a3412" strokeWidth="1" transform="rotate(-14 18 35)" />
        <path d="M 32 35 H 92" stroke="#b45309" strokeWidth="1.4" strokeDasharray="5 6" className="blink" />
        <path d="M 148 35 H 208" stroke="#b45309" strokeWidth="1.4" strokeDasharray="5 6" className="blink" />
        <text x="120" y="24" textAnchor="middle" fill="#f59e0b" fontSize="11" className="blink">⚠ 通讯中断</text>
        <rect x="210" y="22" width="20" height="14" rx="3" fill="none" stroke="#94a3b8" strokeWidth="1.4" />
        <line x1="230" y1="29" x2="238" y2="29" stroke="#94a3b8" strokeWidth="1.2" />
        <circle cx="120" cy="42" r="1.6" fill="#f59e0b" className="blink" />
      </svg>
      <p className="mt-1 text-xs text-ink-3">卫星通讯未建立 —— 申请中继站许可后点火对接</p>
      <style>{`@keyframes bl{0%,100%{opacity:1}50%{opacity:.25}}.blink{animation:bl 1.6s ease-in-out infinite}`}</style>
    </div>
  );
}

// safeMetaName 从元信息 JSON 提取中继站名（容错）。
function safeMetaName(metaJSON: string | null | undefined): string {
  if (!metaJSON) {
    return "";
  }
  try {
    return JSON.parse(metaJSON).name ?? "";
  } catch {
    return "";
  }
}

export default function RelayAdminPage() {
  const [form, setForm] = useState<PageState>(emptyState);
  const [status, setStatus] = useState<ConnStatus>("idle");
  const [relayName, setRelayName] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState("");
  const [message, setMessage] = useState("");
  const [ritual, setRitual] = useState(false);

  // 拉取配置并派生状态（key 是否已隐藏保管由 has_key 表达）
  const reload = useCallback(() => {
    return apiRelayConfig()
      .then((cfg) => {
        setForm({
          url: cfg.url ?? "", mode: cfg.mode || "public", category: cfg.default_category ?? "",
          retentionDays: cfg.local_retention_days || 7,
        });
        setRelayName(safeMetaName(cfg.relay_meta_json));
        const pending = (cfg as { claim_pending?: boolean }).claim_pending;
        setStatus(cfg.enabled ? "connected" : cfg.has_key ? "licensed" : pending ? "reviewing" : "idle");
        setLoaded(true);
        return cfg;
      })
      .catch(() => setMessage("配置加载失败"));
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  // claimPoll 审核中轮询：中继站运营方通过后自动领取许可（每 5 秒）
  useEffect(() => {
    if (status !== "reviewing") {
      return;
    }
    const timer = setInterval(() => {
      apiRelayClaim()
        .then((d) => {
          if (d.status === "approved") {
            setStatus("licensed");
            setMessage(`申请已通过——${d.relay_name || relayName || "中继站"} 的许可已自动领取并隐藏保管，可点火对接`);
          } else if (d.status === "rejected") {
            setStatus("idle");
            setMessage("申请被中继站拒绝，可修改后重新申请");
          }
        })
        .catch(() => undefined);
    }, 5000);
    return () => clearInterval(timer);
  }, [status, relayName]);

  // doApply 自助申请：自动通过则直接获证；手动审核进入"审核中"（v1.4 审核制）
  const doApply = useCallback(() => {
    setBusy("apply");
    setMessage("");
    apiRelayApply({ url: form.url, mode: form.mode })
      .then((d) => {
        setRelayName(d.relay_name);
        if (!form.category && d.categories?.length) {
          setForm((f) => ({ ...f, category: d.categories[0] }));
        }
        if (d.status === "pending") {
          setStatus("reviewing");
          setMessage(`申请已提交 ${d.relay_name}，等待运营方审核…（本页自动检测通过结果）`);
        } else {
          setStatus("licensed");
          setMessage(`已获得 ${d.relay_name} 的对接许可，key 已隐藏保管`);
        }
      })
      .catch((err) => setMessage(err instanceof ApiError ? err.message : "申请失败"))
      .finally(() => setBusy(""));
  }, [form.url, form.mode, form.category]);

  // finishRitual 仪式完成：正式启用对接（key 由后端沿用隐藏保管值）
  const finishRitual = useCallback(() => {
    setRitual(false);
    setBusy("enable");
    apiRelaySave({
      enabled: true, url: form.url, site_key: "", mode: form.mode,
      default_category: form.category, local_retention_days: form.retentionDays,
    })
      .then(() => {
        setStatus("connected");
        setMessage("对接成功，订阅任务数秒内生效，首页「🌐 大世界」已开通");
      })
      .catch((err) => setMessage(err instanceof ApiError ? err.message : "启用失败"))
      .finally(() => setBusy(""));
  }, [form]);

  // doDisconnect 断开对接（保留许可，可随时重新点火）
  const doDisconnect = useCallback(() => {
    apiRelaySave({
      enabled: false, url: form.url, site_key: "", mode: form.mode,
      default_category: form.category, local_retention_days: form.retentionDays,
    })
      .then(() => {
        setStatus("licensed");
        setMessage("已断开（许可仍隐藏保管，可重新点火）");
      })
      .catch((err) => setMessage(err instanceof ApiError ? err.message : "操作失败"));
  }, [form]);

  if (!loaded) {
    return <div className="p-6 text-sm text-ink-2">加载中…</div>;
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <header>
        <h1 className="font-display text-2xl font-semibold text-ink">中继站 · 大世界</h1>
        <p className="mt-1 text-sm text-ink-2">
          申请许可 → 点火对接 → 你的博客加入星系：内容广播到每一颗星球，首页呈现跨站「大世界」。
        </p>
      </header>

      {/* 状态卡（三态） */}
      {status === "idle" && <BrokenLink />}
      {status === "reviewing" && (
        <div className="rounded-xl border border-line bg-card p-4 text-center">
          <p className="text-2xl animate-pulse">📡</p>
          <p className="mt-1 text-sm font-medium text-ink">
            申请已提交 {relayName || "中继站"}，等待运营方审核
          </p>
          <p className="mt-1 text-xs text-ink-3">本页每 5 秒自动检测 · 审核通过后将自动领取许可，届时可点火对接</p>
        </div>
      )}
      {status === "licensed" && (
        <div className="rounded-xl border border-line bg-card p-4 text-center">
          <p className="text-2xl">🔐</p>
          <p className="mt-1 text-sm font-medium text-ink">
            已获得 {relayName || "中继站"} 的对接许可
          </p>
          <p className="mt-1 text-xs text-ink-3">许可证已隐藏保管（key 不在页面显示）· 点火后正式对接</p>
        </div>
      )}
      {status === "connected" && (
        <div className="rounded-xl border border-line bg-card p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-ink">🛰️ 已对接 {relayName || "中继站"}</p>
              <p className="mt-1 text-xs text-ink-3">订阅运行中 · key 隐藏保管 · 断开后可随时重新点火</p>
            </div>
            <button
              type="button"
              onClick={doDisconnect}
              className="rounded-lg border border-line px-4 py-1.5 text-xs text-ink-2 transition-colors hover:bg-muted"
            >
              断开对接
            </button>
          </div>
        </div>
      )}

      {/* 参数区 */}
      <section className="space-y-4 rounded-xl border border-line bg-card p-4">
        <div>
          <label className="mb-1 block text-sm text-ink-2">中继站地址</label>
          <input
            value={form.url}
            onChange={(e) => setForm({ ...form, url: e.target.value })}
            placeholder="https://relay.example.com"
            className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-3">
          <div>
            <label className="mb-1 block text-sm text-ink-2">站点模式</label>
            <select
              value={form.mode}
              onChange={(e) => setForm({ ...form, mode: e.target.value })}
              className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
            >
              <option value="public">公网可达（摘要导流）</option>
              <option value="bridged">仅内网 · 需桥接（全文托管）</option>
            </select>
          </div>
          <div>
            <label className="mb-1 block text-sm text-ink-2">发布默认分类</label>
            <input
              value={form.category}
              onChange={(e) => setForm({ ...form, category: e.target.value })}
              className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
            />
          </div>
          <div>
            <label className="mb-1 block text-sm text-ink-2">本地保存天数（1~30）</label>
            <input
              type="number" min={1} max={30}
              value={form.retentionDays}
              onChange={(e) => setForm({ ...form, retentionDays: Number(e.target.value) || 7 })}
              className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
            />
          </div>
        </div>

        {/* 动作区（按状态切换） */}
        <div className="flex flex-wrap items-center gap-3">
          {status === "idle" && (
            <button
              type="button"
              onClick={doApply}
              disabled={busy === "apply" || !form.url}
              className="rounded-lg bg-glow px-5 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
            >
              {busy === "apply" ? "申请中…" : "📡 申请对接许可"}
            </button>
          )}
          {status === "licensed" && (
            <button
              type="button"
              onClick={() => { setMessage(""); setRitual(true); }}
              className="rounded-lg bg-glow px-6 py-2.5 text-sm font-medium text-white shadow-lg transition-opacity hover:opacity-90"
            >
              🚀 点火对接
            </button>
          )}
          {status === "connected" && (
            <button
              type="button"
              onClick={() => void reload().then(() => setMessage("已刷新订阅状态"))}
              className="rounded-lg border border-line px-4 py-2 text-sm text-ink transition-colors hover:bg-muted"
            >
              刷新状态
            </button>
          )}
        </div>
        {message && <p className="text-sm text-glow">{message}</p>}
      </section>

      {/* 对接仪式（全屏遮罩：倒计时 → 卫星对接星球 → 成功） */}
      {ritual && <RelayRitual relayName={relayName} onFinish={finishRitual} />}
    </div>
  );
}
