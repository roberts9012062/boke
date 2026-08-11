// src/app/admin/seo/page.tsx
// 后台 SEO 设置（设计稿 D/冷月/SEO设置 1400×1100）：
// 全局（站点标题后缀/默认描述/默认关键词）+ 站点地图（自动生成 sitemap + 说明）
// + 社交分享（Open Graph 预览）+ 索引策略（robots.txt 规则）+ 保存。
"use client";

import { useEffect, useState } from "react";

import { Switch } from "@/components/ui/switch";
import { apiSaveSeoSettings, apiSeoSettings, ApiError, type SeoSettings } from "@/lib/api";

// AdminSeo SEO 设置页。
export default function AdminSeo() {
  const [settings, setSettings] = useState<SeoSettings>({
    site_name: "月言",
    site_description: "",
    title_suffix: "· 月言",
    keywords: "",
    og_title: "",
    robots_txt: "",
    sitemap_enabled: true,
  });
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [saved, setSaved] = useState<boolean>(false);
  const [search, setSearch] = useState<string>(""); // 搜索设置项（设计稿）

  // 加载设置
  useEffect(() => {
    apiSeoSettings()
      .then(setSettings)
      .catch(() => undefined)
      .finally(() => setLoaded(true));
  }, []);

  // 保存
  const handleSave = async () => {
    setError("");
    setSaved(false);
    try {
      await apiSaveSeoSettings(settings);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    }
  };

  // 更新字段（浅合并）
  const update = (patch: Partial<SeoSettings>) => setSettings((prev) => ({ ...prev, ...patch }));

  return (
    <div className="max-w-[720px]">
      <h1 className="font-display text-xl font-semibold text-ink">SEO 设置</h1>
      {/* 设计稿：SEO 优化 · v1.2.0 · 已启用（对应插件商城「SEO 优化」插件状态） */}
      <p className="mt-0.5 text-xs text-ink-3">SEO 优化 · v1.2.0 · 已启用（插件商城安装）</p>

      {/* 搜索设置项（设计稿：搜索设置项…） */}
      <input
        type="text"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        placeholder="搜索设置项…"
        className="mt-4 h-9 w-64 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
      />

      {/* 设置区（搜索过滤分组） */}
      <div className="mt-5 space-y-5 rounded-lg border border-line bg-elevated p-6">
        {/* 搜索匹配：过滤显示（空搜索显示全部） */}
        {search && (
          <p className="text-xs text-ink-3">
            匹配项：{["站点标题后缀", "默认描述", "默认关键词", "自动生成 sitemap", "社交分享", "robots.txt 规则"].filter((s) => s.includes(search)).join("、") || "无匹配"}
          </p>
        )}
        {/* 全局（设计稿：站点标题后缀/默认描述/默认关键词） */}
        <section>
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-ink">全局</h2>
            <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[10px] text-glow">已启用</span>
          </div>
          <div className="mt-4 space-y-4">
            <div>
              <label htmlFor="seo-suffix" className="mb-1.5 block text-sm text-ink-2">
                站点标题后缀
                <span className="ml-2 text-xs text-ink-3">拼在文章标题后</span>
              </label>
              <input
                id="seo-suffix"
                type="text"
                value={settings.title_suffix}
                onChange={(e) => update({ title_suffix: e.target.value })}
                className="h-10 w-full max-w-sm rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
              />
              <p className="mt-1 text-xs text-ink-3">示例：夜读手记 {settings.title_suffix}</p>
            </div>
            <div>
              <label htmlFor="seo-desc" className="mb-1.5 block text-sm text-ink-2">
                默认描述
                <span className="ml-2 text-xs text-ink-3">未单独设置时使用 · 120 字内</span>
              </label>
              <textarea
                id="seo-desc"
                value={settings.site_description}
                onChange={(e) => update({ site_description: e.target.value })}
                rows={2}
                maxLength={120}
                className="w-full rounded-lg border border-line bg-muted px-4 py-3 text-sm text-ink focus:border-accent focus:outline-none"
              />
            </div>
            <div>
              <label htmlFor="seo-keywords" className="mb-1.5 block text-sm text-ink-2">
                默认关键词
                <span className="ml-2 text-xs text-ink-3">逗号分隔</span>
              </label>
              <input
                id="seo-keywords"
                type="text"
                value={settings.keywords}
                onChange={(e) => update({ keywords: e.target.value })}
                className="h-10 w-full max-w-sm rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
              />
              <p className="mt-1 text-xs text-ink-3">
                {settings.keywords ? `${settings.keywords.split(/[,，]/).filter(Boolean).length} 个关键词` : "未设置默认关键词"}
              </p>
            </div>
          </div>
        </section>

        {/* 站点地图（设计稿：自动生成 sitemap /sitemap.xml） */}
        <section className="border-t border-line pt-5">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold text-ink">自动生成 sitemap</h2>
              <p className="mt-0.5 text-xs text-ink-3">
                /sitemap.xml · 包含公开帖子与图片。更新后可主动推送搜索引擎。
              </p>
            </div>
            <Switch
              checked={settings.sitemap_enabled}
              onChange={(v) => update({ sitemap_enabled: v })}
              label="自动生成 sitemap"
            />
          </div>
        </section>

        {/* 社交分享（设计稿：Open Graph 预览） */}
        <section className="border-t border-line pt-5">
          <h2 className="text-sm font-semibold text-ink">社交分享</h2>
          <div className="mt-3 rounded-lg border border-line bg-muted/40 p-4">
            <p className="text-xs text-ink-3">Open Graph 预览 · 分享卡片</p>
            <p className="mt-1 text-sm text-ink">
              {settings.site_name} · 月色微博客 — {settings.site_description || "月光下慢慢写，记录日常与灵感。"}
            </p>
            <span className="mt-2 inline-block rounded-full bg-accent-soft px-2 py-0.5 text-[10px] text-glow">
              已配置
            </span>
          </div>
        </section>

        {/* 索引策略（设计稿：robots.txt 规则） */}
        <section className="border-t border-line pt-5">
          <h2 className="text-sm font-semibold text-ink">robots.txt 规则</h2>
          <textarea
            value={settings.robots_txt}
            onChange={(e) => update({ robots_txt: e.target.value })}
            rows={4}
            placeholder={"User-agent: *\nAllow: /\nDisallow: /admin/\n私密帖 noindex · 搜索页 noindex"}
            className="mt-3 w-full rounded-lg border border-line bg-muted px-4 py-3 font-mono text-xs text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <p className="mt-1 text-xs text-ink-3">留空使用默认规则（Allow: / · Disallow: /admin/）</p>
        </section>

        {error && (
          <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}
        {saved && (
          <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
            保存成功，设置已生效
          </p>
        )}

        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={!loaded}
            className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
          >
            保存
          </button>
        </div>
      </div>
    </div>
  );
}
