// src/app/settings/privacy/page.tsx
// 隐私设置页（设计稿 D/冷月/隐私设置 1400×900）：
// 可见性（公开个人主页/允许搜索到我/显示在线状态）+ 互动（谁可以私信我/谁可以评论/被 @ 时通知）
// + 数据（个性化推荐/下载我的数据/管理黑名单）+「更改即时生效」。
// 说明（MVP）：偏好存 localStorage（本地生效）；服务端过滤规则（搜索可见/私信权限）后置（差异记录）。
"use client";

import { useEffect, useState } from "react";

import { SettingsLayout } from "@/components/settings-layout";
import { apiMe, ApiError, get } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { PageResult, PostSummary, UserRelationDTO } from "@/types/api";

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

// fetchAllPages 分页循环拉取全量列表数据（每页 100 条；防御上限 100 页防死循环）。
// 参数：pathForPage 按页码构造接口路径的纯函数。
async function fetchAllPages<T>(pathForPage: (page: number) => string): Promise<T[]> {
  const all: T[] = [];
  for (let page = 1; page <= 100; page += 1) {
    const result = await get<PageResult<T>>(pathForPage(page));
    all.push(...result.items);
    // 拉满或返回空页即停止
    if (all.length >= result.total || result.items.length === 0) {
      break;
    }
  }
  return all;
}

// triggerJsonDownload 触发浏览器下载 JSON 文件（Blob → 临时 URL → a.download）。
function triggerJsonDownload(fileName: string, payload: unknown): void {
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.click();
  URL.revokeObjectURL(url);
}

// PrivacyPage 隐私设置。
export default function PrivacyPage() {
  const { user } = useAuth();
  const [prefs, setPrefs] = useState<PrivacyPrefs>({ ...DEFAULTS });
  const [loaded, setLoaded] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  // 数据导出状态（M 后置修复：「下载我的数据」此前为假按钮）
  const [exporting, setExporting] = useState<boolean>(false);
  const [exportError, setExportError] = useState<string>("");

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

  // 下载我的数据：聚合账号资料/全部帖子/收藏/关注与粉丝，导出为 JSON 文件
  const handleExport = async () => {
    if (!user || exporting) {
      return;
    }
    setExporting(true);
    setExportError("");
    try {
      const [profile, posts, favorites, following, followers] = await Promise.all([
        apiMe(),
        fetchAllPages<PostSummary>((p) => `/users/${user.id}/posts?page=${p}&page_size=100`),
        fetchAllPages<PostSummary>((p) => `/me/favorites?page=${p}&page_size=100`),
        fetchAllPages<UserRelationDTO>((p) => `/users/${user.id}/following?page=${p}&page_size=100`),
        fetchAllPages<UserRelationDTO>((p) => `/users/${user.id}/followers?page=${p}&page_size=100`),
      ]);
      triggerJsonDownload(`yueyan-data-${user.username}.json`, {
        exported_at: new Date().toISOString(),
        profile,
        posts,
        favorites,
        following,
        followers,
      });
    } catch (err) {
      setExportError(err instanceof ApiError ? err.message : "导出失败，请稍后再试");
    } finally {
      setExporting(false);
    }
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
              onClick={() => void handleExport()}
              disabled={exporting}
              className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
            >
              {exporting ? "导出中…" : "下载 JSON"}
            </button>
          </div>
          <div className="flex items-center justify-between py-3">
            <div>
              <p className="text-sm text-ink">管理黑名单</p>
              <p className="text-xs text-ink-3">拉黑用户后对方无法评论与私信</p>
            </div>
            {/* 诚实态：功能未上线时禁用并说明，不再以假按钮误导 */}
            <button
              type="button"
              disabled
              title="该功能即将上线"
              className="cursor-not-allowed rounded-full border border-line px-4 py-1.5 text-sm text-ink-3/60"
            >
              即将上线
            </button>
          </div>
        </div>

        {/* 导出失败提示 */}
        {exportError && (
          <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {exportError}
          </p>
        )}
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
