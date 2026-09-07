// src/components/relay-status-cards.tsx
// 中继站对接页的状态卡组件族（从 admin/relay/page.tsx 拆出，协议 v1.4）：
// 断线卫星 / 审核中 / 已获证 / 已对接（含每日配额）四态卡片 + 公告悬浮面板 + 元信息解析。
// 全部为纯展示组件：状态与回调由页面显式注入，不持有任何数据获取逻辑。
"use client";

import { Markdown } from "@/components/markdown";

// RelayMeta 解析后的元信息快照（含配额；旧缓存可能缺 quota/rules）。
export interface RelayMeta {
  name: string;
  rules_md: string;
  quota?: { daily_moments: number; daily_articles: number; media: { per_item_bytes: number; daily_items: number; daily_bytes: number } };
}

// parseMeta 从元信息 JSON 提取（容错：结构不全时返回空字段）。
export function parseMeta(metaJSON: string | null | undefined): RelayMeta {
  if (!metaJSON) {
    return { name: "", rules_md: "" };
  }
  try {
    const d = JSON.parse(metaJSON);
    return { name: d.name ?? "", rules_md: d.rules_md ?? "", quota: d.quota };
  } catch {
    return { name: "", rules_md: "" };
  }
}

// safeMetaName 从元信息 JSON 提取中继站名（容错）。
export function safeMetaName(metaJSON: string | null | undefined): string {
  return parseMeta(metaJSON).name;
}

// BrokenLink 断线卫星小场景（未连接态）：星球与卫星之间通讯中断。
export function BrokenLink() {
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
      <p className="mt-1 text-xs text-ink-3">卫星通讯未建立 —— 申请中继站许可后对接中继卫星</p>
      <style>{`@keyframes bl{0%,100%{opacity:1}50%{opacity:.25}}.blink{animation:bl 1.6s ease-in-out infinite}`}</style>
    </div>
  );
}

// ReviewingCard 审核中卡片（手动审核模式：等待运营方通过，页面轮询中）。
export function ReviewingCard(props: { relayName: string }) {
  return (
    <div className="rounded-xl border border-line bg-card p-4 text-center">
      <p className="text-2xl animate-pulse">📡</p>
      <p className="mt-1 text-sm font-medium text-ink">
        申请已提交 {props.relayName || "中继站"}，等待运营方审核
      </p>
      <p className="mt-1 text-xs text-ink-3">本页每 5 秒自动检测 · 审核通过后将自动领取许可，届时可通讯连接</p>
    </div>
  );
}

// LicensedCard 已获证卡片（key 已隐藏保管，待通讯连接）。
export function LicensedCard(props: { relayName: string }) {
  return (
    <div className="rounded-xl border border-line bg-card p-4 text-center">
      <p className="text-2xl">🔐</p>
      <p className="mt-1 text-sm font-medium text-ink">
        已获得 {props.relayName || "中继站"} 的对接许可
      </p>
      <p className="mt-1 text-xs text-ink-3">许可证已隐藏保管（key 不在页面显示）· 通讯连接后正式对接</p>
    </div>
  );
}

// ConnectedCard 已对接状态卡（订阅运行中 + 每日配额；公告/断开回调由页面注入）。
export function ConnectedCard(props: { relayName: string; meta: RelayMeta; onAnnouncement: () => void; onDisconnect: () => void }) {
  return (
    <div className="rounded-xl border border-line bg-card p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium text-ink">🛰️ 已对接 {props.relayName || "中继站"}</p>
          <p className="mt-1 text-xs text-ink-3">订阅运行中 · key 隐藏保管 · 断开后可随时重新通讯连接</p>
        </div>
        <div className="flex gap-2">
          {props.meta.rules_md && (
            <button
              type="button"
              onClick={props.onAnnouncement}
              className="rounded-lg bg-accent-soft px-4 py-1.5 text-xs font-medium text-glow transition-opacity hover:opacity-85"
            >
              📜 中继站公告
            </button>
          )}
          <button
            type="button"
            onClick={props.onDisconnect}
            className="rounded-lg border border-line px-4 py-1.5 text-xs text-ink-2 transition-colors hover:bg-muted"
          >
            断开对接
          </button>
        </div>
      </div>
      {/* 每日信息限制（握手缓存的配额，运营方调整后经 config.update/握手刷新） */}
      {props.meta.quota && (
        <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3 text-xs">
          <span className="rounded-full bg-muted px-3 py-1 text-ink-2">每日说说 {props.meta.quota.daily_moments} 条</span>
          <span className="rounded-full bg-muted px-3 py-1 text-ink-2">每日文章 {props.meta.quota.daily_articles} 篇</span>
          <span className="rounded-full bg-muted px-3 py-1 text-ink-2">
            媒体 ≤{Math.round(props.meta.quota.media.per_item_bytes / 1024)}KB · 每日 {props.meta.quota.media.daily_items} 张
          </span>
        </div>
      )}
    </div>
  );
}

// AnnouncementPanel 公告悬浮面板（中继站规则 Markdown 渲染；点遮罩或关闭按钮退出）。
export function AnnouncementPanel(props: { relayName: string; rulesMD: string; onClose: () => void }) {
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/50 p-4" onClick={props.onClose}>
      <div
        className="max-h-[76vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-line bg-elevated p-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 className="font-display text-lg font-semibold text-ink">📜 {props.relayName || "中继站"} · 公告规则</h3>
          <button
            type="button"
            onClick={props.onClose}
            className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:bg-muted"
          >
            关闭
          </button>
        </div>
        <Markdown content={props.rulesMD} />
      </div>
    </div>
  );
}
