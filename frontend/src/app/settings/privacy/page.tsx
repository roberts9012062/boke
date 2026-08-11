// src/app/settings/privacy/page.tsx
// 隐私设置页（设计稿 D/冷月/隐私设置 1400×900）：
// 可见性（公开个人主页/允许搜索到我/显示在线状态）+ 互动（谁可以私信我/谁可以评论/被 @ 时通知）
// + 数据（个性化推荐/下载我的数据/管理黑名单）+「更改即时生效」。
// 说明（MVP）：偏好存 localStorage（本地生效）；服务端过滤规则（搜索可见/私信权限）后置（差异记录）。
"use client";

import { useEffect, useState } from "react";

import { SettingsLayout } from "@/components/settings-layout";

// localStorage 键（隐私偏好）。
const PRIVACY_KEY = "yueyan-privacy";

// 默认偏好（设计稿各开关默认开启）。
const DEFAULTS = {
  publicProfile: true, // 公开个人主页
  searchable: true, // 允许搜索到我
  showOnline: true, // 显示在线状态
  dmScope: "all", // 谁可以私信我：all=所有人 / followers=仅关注者
  commentScope: "all", // 谁可以评论：all=所有人 / friends=关注者与互关
  mentionNotify: true, // 被 @ 时通知
  personalized: true, // 个性化推荐
};

// 偏好类型（与 DEFAULTS 同构）。
type PrivacyPrefs = typeof DEFAULTS;

// readPrefs 读取本地偏好（解析失败回退默认）。
function readPrefs(): PrivacyPrefs {
  if (typeof window === "undefined") {
    return { ...DEFAULTS };
  }
  try {
    const raw = localStorage.getItem(PRIVACY_KEY);
    return raw ? { ...DEFAULTS, ...(JSON.parse(raw) as Partial<PrivacyPrefs>) } : { ...DEFAULTS };
  } catch {
    return { ...DEFAULTS };
  }
}

// PrivacyPage 隐私设置。
export default function PrivacyPage() {
  const [prefs, setPrefs] = useState<PrivacyPrefs>({ ...DEFAULTS });
  const [loaded, setLoaded] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);

  // 加载本地偏好
  useEffect(() => {
    setPrefs(readPrefs());
    setLoaded(true);
  }, []);

  // 更新偏好（更改即时生效 + 本地持久化）
  const update = (patch: Partial<PrivacyPrefs>) => {
    setPrefs((prev) => {
      const next = { ...prev, ...patch };
      localStorage.setItem(PRIVACY_KEY, JSON.stringify(next));
      return next;
    });
    setSaved(true);
  };

  // 开关行（设计稿：标题 + 说明 + 开关）
  const ToggleRow = ({ label, desc, value, onChange }: { label: string; desc: string; value: boolean; onChange: (v: boolean) => void }) => (
    <div className="flex items-center justify-between py-3">
      <div>
        <p className="text-sm text-ink">{label}</p>
        <p className="text-xs text-ink-3">{desc}</p>
      </div>
      <button
        type="button"
        onClick={() => onChange(!value)}
        className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${value ? "bg-accent" : "bg-muted"}`}
        aria-label={`${label}开关`}
      >
        <span
          className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${value ? "left-[22px]" : "left-0.5"}`}
        />
      </button>
    </div>
  );

  return (
    <SettingsLayout active="privacy">
      <h1 className="font-display text-xl font-semibold text-ink">隐私设置</h1>
      <p className="mt-0.5 text-xs text-ink-3">控制谁可以看到你的内容与资料</p>

      <div className="mt-5 rounded-lg border border-line bg-elevated p-6">
        {/* 可见性 */}
        <h2 className="text-sm font-semibold text-ink">可见性</h2>
        <div className="divide-y divide-line">
          <ToggleRow label="公开个人主页" desc="关闭后仅关注者可见你的主页" value={prefs.publicProfile} onChange={(v) => update({ publicProfile: v })} />
          <ToggleRow label="允许搜索到我" desc="通过用户名或邮箱被发现" value={prefs.searchable} onChange={(v) => update({ searchable: v })} />
          <ToggleRow label="显示在线状态" desc="在消息中展示在线" value={prefs.showOnline} onChange={(v) => update({ showOnline: v })} />
        </div>

        {/* 互动（设计稿：单选 + 开关） */}
        <h2 className="mt-6 text-sm font-semibold text-ink">互动</h2>
        <div className="divide-y divide-line">
          <div className="py-3">
            <p className="text-sm text-ink">谁可以私信我</p>
            <div className="mt-2 flex gap-2">
              {[
                { key: "all", label: "所有人" },
                { key: "followers", label: "仅关注者" },
              ].map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => update({ dmScope: opt.key })}
                  className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                    prefs.dmScope === opt.key
                      ? "border-accent bg-accent-soft text-glow"
                      : "border-line text-ink-2 hover:text-ink"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
          <div className="py-3">
            <p className="text-sm text-ink">谁可以评论</p>
            <div className="mt-2 flex gap-2">
              {[
                { key: "all", label: "所有人" },
                { key: "friends", label: "关注者与互关可评论" },
              ].map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => update({ commentScope: opt.key })}
                  className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                    prefs.commentScope === opt.key
                      ? "border-accent bg-accent-soft text-glow"
                      : "border-line text-ink-2 hover:text-ink"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
          <ToggleRow label="被 @ 时通知" desc="有人提及你时推送" value={prefs.mentionNotify} onChange={(v) => update({ mentionNotify: v })} />
        </div>

        {/* 数据 */}
        <h2 className="mt-6 text-sm font-semibold text-ink">数据</h2>
        <div className="divide-y divide-line">
          <ToggleRow label="个性化推荐" desc="基于阅读兴趣排序信息流" value={prefs.personalized} onChange={(v) => update({ personalized: v })} />
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="text-sm text-ink">下载我的数据</p>
              <p className="text-xs text-ink-3">导出帖子与账号信息</p>
            </div>
            <button
              type="button"
              onClick={() => setSaved(true)}
              className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
            >
              即将上线
            </button>
          </div>
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="text-sm text-ink">管理黑名单</p>
              <p className="text-xs text-ink-3">拉黑用户后对方无法评论与私信</p>
            </div>
            <button
              type="button"
              onClick={() => setSaved(true)}
              className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
            >
              即将上线
            </button>
          </div>
        </div>

        {saved && (
          <p className="mt-4 rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
            更改即时生效
          </p>
        )}
        {!loaded && <div className="h-40 animate-pulse rounded-lg bg-muted" aria-hidden />}
      </div>
    </SettingsLayout>
  );
}
