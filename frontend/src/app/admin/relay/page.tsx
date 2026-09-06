// src/app/admin/relay/page.tsx
// 后台「中继站」配置页（B-1'）：开关 / URL / key / 站点模式 / 连接测试 / 默认分类 / 本地保存天数。
// 连接测试实时调中继站 handshake，回显名称、规则、每日配额与分类列表。
"use client";

import { useCallback, useEffect, useState } from "react";

import {
  apiRelayConfig,
  apiRelaySave,
  apiRelayTest,
  type RelayConfig,
  type RelayHandshakeResp,
} from "@/lib/api-relay";
import { ApiError } from "@/lib/api";

// PageState 表单状态（全量显式字段）。
interface PageState {
  enabled: boolean;
  url: string;
  siteKey: string;
  mode: string;
  category: string;
  retentionDays: number;
}

// emptyState 未加载时的空表单。
const emptyState: PageState = {
  enabled: false, url: "", siteKey: "", mode: "public", category: "", retentionDays: 7,
};

export default function RelayAdminPage() {
  const [form, setForm] = useState<PageState>(emptyState);
  const [loaded, setLoaded] = useState(false);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [handshake, setHandshake] = useState<RelayHandshakeResp | null>(null);
  const [message, setMessage] = useState("");

  // 回填当前配置
  useEffect(() => {
    apiRelayConfig()
      .then((cfg: RelayConfig) => {
        setForm({
          enabled: cfg.enabled, url: cfg.url, siteKey: cfg.site_key, mode: cfg.mode || "public",
          category: cfg.default_category, retentionDays: cfg.local_retention_days || 7,
        });
        setLoaded(true);
      })
      .catch(() => setMessage("配置加载失败"));
  }, []);

  // doTest 连接测试：实时握手并回显
  const doTest = useCallback(() => {
    setTesting(true);
    setHandshake(null);
    setMessage("");
    apiRelayTest({ url: form.url, site_key: form.siteKey, mode: form.mode })
      .then((resp) => setHandshake(resp))
      .catch((err) => setMessage(err instanceof ApiError ? err.message : "连接失败"))
      .finally(() => setTesting(false));
  }, [form.url, form.siteKey, form.mode]);

  // doSave 保存配置（订阅任务自动重启，≤5s 生效）
  const doSave = useCallback(() => {
    setSaving(true);
    setMessage("");
    apiRelaySave({
      enabled: form.enabled, url: form.url, site_key: form.siteKey, mode: form.mode,
      default_category: form.category, local_retention_days: form.retentionDays,
    })
      .then(() => setMessage("已保存，订阅任务将在数秒内生效"))
      .catch((err) => setMessage(err instanceof ApiError ? err.message : "保存失败"))
      .finally(() => setSaving(false));
  }, [form]);

  if (!loaded) {
    return <div className="p-6 text-sm text-ink-2">加载中…</div>;
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <header>
        <h1 className="font-display text-2xl font-semibold text-ink">中继站 · 大世界</h1>
        <p className="mt-1 text-sm text-ink-2">
          接入跨站内容分发总线：本站内容推送到所有成员博客，首页呈现聚合流「大世界」。
        </p>
      </header>

      {/* 接入开关 */}
      <section className="rounded-xl border border-line bg-card p-4">
        <label className="flex items-center gap-3">
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
            className="h-4 w-4 accent-glow"
          />
          <span className="text-sm font-medium text-ink">开启「大世界」</span>
          <span className="text-xs text-ink-3">关闭后断开订阅，本地缓存自然过期</span>
        </label>
      </section>

      {/* 对接参数 */}
      <section className="space-y-4 rounded-xl border border-line bg-card p-4">
        <div>
          <label className="mb-1 block text-sm text-ink-2">中继站 URL</label>
          <input
            value={form.url}
            onChange={(e) => setForm({ ...form, url: e.target.value })}
            placeholder="https://relay.example.com"
            className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
          />
        </div>
        <div>
          <label className="mb-1 block text-sm text-ink-2">站点 key（运营方发放）</label>
          <input
            type="password"
            value={form.siteKey}
            onChange={(e) => setForm({ ...form, siteKey: e.target.value })}
            placeholder="rs_…"
            className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
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
            <label className="mb-1 block text-sm text-ink-2">本地保存天数（1~30）</label>
            <input
              type="number"
              min={1}
              max={30}
              value={form.retentionDays}
              onChange={(e) => setForm({ ...form, retentionDays: Number(e.target.value) || 7 })}
              className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
            />
          </div>
        </div>
        <div>
          <label className="mb-1 block text-sm text-ink-2">发布默认分类（连接测试后可选）</label>
          <input
            value={form.category}
            onChange={(e) => setForm({ ...form, category: e.target.value })}
            list="relay-categories"
            placeholder="生活 / 技术 / 随笔 …"
            className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink"
          />
          <datalist id="relay-categories">
            {(handshake?.meta.categories ?? []).map((c) => (
              <option key={c} value={c} />
            ))}
          </datalist>
        </div>
        <div className="flex gap-3">
          <button
            type="button"
            onClick={doTest}
            disabled={testing || !form.url || !form.siteKey}
            className="rounded-lg border border-line px-4 py-2 text-sm text-ink transition-colors hover:bg-muted disabled:opacity-50"
          >
            {testing ? "测试中…" : "连接测试"}
          </button>
          <button
            type="button"
            onClick={doSave}
            disabled={saving}
            className="rounded-lg bg-glow px-4 py-2 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {saving ? "保存中…" : "保存"}
          </button>
        </div>
        {message && <p className="text-sm text-glow">{message}</p>}
      </section>

      {/* 连接测试回显：中继站元信息与配额 */}
      {handshake && (
        <section className="space-y-2 rounded-xl border border-line bg-card p-4">
          <h2 className="font-medium text-ink">{handshake.meta.name}</h2>
          <p className="text-xs text-ink-3">
            成员 {handshake.meta.site_count}/{handshake.meta.max_sites} · 中继站保留 {handshake.meta.retention_days} 天
          </p>
          <div className="flex flex-wrap gap-2 text-xs">
            <span className="rounded-full bg-accent-soft px-3 py-1 text-glow">每日说说 {handshake.quota.daily_moments}</span>
            <span className="rounded-full bg-accent-soft px-3 py-1 text-glow">每日文章 {handshake.quota.daily_articles}</span>
            <span className="rounded-full bg-accent-soft px-3 py-1 text-glow">
              媒体 ≤{Math.round(handshake.quota.media.per_item_bytes / 1024)}KB · {handshake.quota.media.daily_items} 张/日
            </span>
          </div>
          {handshake.meta.rules_md && (
            <details className="text-sm text-ink-2">
              <summary className="cursor-pointer">中继站规则</summary>
              <pre className="mt-2 whitespace-pre-wrap rounded-lg bg-muted p-3 text-xs">{handshake.meta.rules_md}</pre>
            </details>
          )}
        </section>
      )}
    </div>
  );
}
