// src/app/admin/relay/page.tsx
// 后台「中继站」页（B-1' + 申请制 + 对接仪式，协议 v1.3）：
// 状态机——未连接（断线卫星）/ 审核中 / 已获证（待通讯连接）/ 已对接（运行中）。
// key 隐藏保管：申请由后端代理完成，明文 key 永不回到前端。
// 状态卡与公告面板的展示组件见 @/components/relay-status-cards（纯展示，回调注入）。
"use client";

import { useCallback, useEffect, useState } from "react";

import { RelayRitual } from "@/components/relay-ritual";
import { AnnouncementPanel, BrokenLink, ConnectedCard, LicensedCard, ReviewingCard, parseMeta, safeMetaName, type RelayMeta } from "@/components/relay-status-cards";
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

export default function RelayAdminPage() {
  const [form, setForm] = useState<PageState>(emptyState);
  const [status, setStatus] = useState<ConnStatus>("idle");
  const [relayName, setRelayName] = useState("");
  const [meta, setMeta] = useState<RelayMeta>({ name: "", rules_md: "" });
  const [announcement, setAnnouncement] = useState(false);
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
        setMeta(parseMeta(cfg.relay_meta_json));
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
            setMessage(`申请已通过——${d.relay_name || relayName || "中继站"} 的许可已自动领取并隐藏保管，可通讯连接`);
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

  // doDisconnect 断开对接（保留许可，可随时重新通讯连接）
  const doDisconnect = useCallback(() => {
    apiRelaySave({
      enabled: false, url: form.url, site_key: "", mode: form.mode,
      default_category: form.category, local_retention_days: form.retentionDays,
    })
      .then(() => {
        setStatus("licensed");
        setMessage("已断开（许可仍隐藏保管，可重新通讯连接）");
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
          申请许可 → 通讯连接 → 你的博客加入星系：内容广播到每一颗星球，首页呈现跨站「大世界」。
        </p>
      </header>

      {/* 状态卡（四态，展示组件） */}
      {status === "idle" && <BrokenLink />}
      {status === "reviewing" && <ReviewingCard relayName={relayName} />}
      {status === "licensed" && <LicensedCard relayName={relayName} />}
      {status === "connected" && (
        <ConnectedCard
          relayName={relayName}
          meta={meta}
          onAnnouncement={() => setAnnouncement(true)}
          onDisconnect={doDisconnect}
        />
      )}

      {/* 公告悬浮面板（中继站规则 Markdown 渲染） */}
      {announcement && (
        <AnnouncementPanel relayName={relayName} rulesMD={meta.rules_md} onClose={() => setAnnouncement(false)} />
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
              📡 通讯连接
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
