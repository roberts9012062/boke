// src/app/admin/settings/page.tsx
// 后台站点设置（需求 4.6 + 设计稿 D/冷月/后台站点）：
// 站点与用户 → 站点名称/站点描述 + 开关（开放注册/开放评论）+ 默认主题 + 保存。
"use client";

import { useEffect, useState } from "react";

import { apiAdminSaveSettings, apiAdminSettings } from "@/lib/api";

// AdminSettings 站点设置。
export default function AdminSettings() {
  const [siteName, setSiteName] = useState<string>("");
  const [siteDesc, setSiteDesc] = useState<string>("");
  const [allowRegister, setAllowRegister] = useState<boolean>(true);
  const [commentOpen, setCommentOpen] = useState<boolean>(true);
  const [theme, setTheme] = useState<string>("cool-moon");
  const [loaded, setLoaded] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 加载设置
  useEffect(() => {
    apiAdminSettings()
      .then((s) => {
        setSiteName(s.site_name ?? "月言");
        setSiteDesc(s.site_description ?? "");
        setAllowRegister(s.allow_register !== "false");
        setCommentOpen(s.comment_open !== "false");
        setTheme(s.theme ?? "cool-moon");
      })
      .catch(() => {
        // 读取失败保持默认
      })
      .finally(() => setLoaded(true));
  }, []);

  // 保存
  const handleSave = async () => {
    setError("");
    setSaved(false);
    try {
      await apiAdminSaveSettings({
        site_name: siteName,
        site_description: siteDesc,
        allow_register: String(allowRegister),
        comment_open: String(commentOpen),
        theme,
      });
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  };

  return (
    <div className="max-w-[640px]">
      <h1 className="font-display text-xl font-semibold text-ink">站点设置</h1>
      <p className="mt-0.5 text-xs text-ink-3">站点与用户 · 保存后即时生效</p>

      <div className="mt-5 space-y-5 rounded-lg border border-line bg-elevated p-6">
        {/* 站点名称（设计稿：月言） */}
        <div>
          <label htmlFor="site-name" className="mb-1.5 block text-sm text-ink-2">
            站点名称
          </label>
          <input
            id="site-name"
            type="text"
            value={siteName}
            onChange={(e) => setSiteName(e.target.value)}
            className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 站点描述 */}
        <div>
          <label htmlFor="site-desc" className="mb-1.5 block text-sm text-ink-2">
            站点描述
          </label>
          <textarea
            id="site-desc"
            value={siteDesc}
            onChange={(e) => setSiteDesc(e.target.value)}
            rows={2}
            className="w-full rounded-lg border border-line bg-muted px-4 py-3 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 开关（需求 4.6：开放注册/开放评论） */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-ink">开放注册</p>
            <p className="text-xs text-ink-3">允许新用户注册</p>
          </div>
          <button
            type="button"
            onClick={() => setAllowRegister((v) => !v)}
            className={`relative h-6 w-11 rounded-full transition-colors ${
              allowRegister ? "bg-accent" : "bg-muted"
            }`}
            aria-label="开放注册开关"
          >
            <span
              className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
                allowRegister ? "left-[22px]" : "left-0.5"
              }`}
            />
          </button>
        </div>

        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-ink">开放评论</p>
            <p className="text-xs text-ink-3">允许访客评论（含匿名）</p>
          </div>
          <button
            type="button"
            onClick={() => setCommentOpen((v) => !v)}
            className={`relative h-6 w-11 rounded-full transition-colors ${
              commentOpen ? "bg-accent" : "bg-muted"
            }`}
            aria-label="开放评论开关"
          >
            <span
              className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
                commentOpen ? "left-[22px]" : "left-0.5"
              }`}
            />
          </button>
        </div>

        {/* 默认主题（需求 4.6：cool-moon / mist） */}
        <div>
          <p className="mb-1.5 text-sm text-ink-2">默认主题</p>
          <div className="flex gap-3">
            {[
              { key: "cool-moon", label: "冷月" },
              { key: "mist", label: "薄雾" },
            ].map((t) => (
              <button
                key={t.key}
                type="button"
                onClick={() => setTheme(t.key)}
                className={`rounded-full border px-5 py-2 text-sm transition-colors ${
                  theme === t.key
                    ? "border-accent bg-accent-soft text-glow"
                    : "border-line text-ink-2 hover:text-ink"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>
        </div>

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
            className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            保存
          </button>
        </div>
      </div>
    </div>
  );
}
