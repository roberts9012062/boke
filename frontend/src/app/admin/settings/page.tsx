// src/app/admin/settings/page.tsx
// 后台站点设置（设计稿 D/冷月/后台站点 1400×800）：
// 「站点与用户」分区 → 站点名称/简介 + 开关（开放评论/维护模式）+ 敏感词（逗号分隔批量添加）
// + 默认主题（M1.7 需求补充）+ 保存设置。
"use client";

import { useEffect, useState } from "react";

import {
  apiAdminAddSensitiveWords,
  apiAdminSaveSettings,
  apiAdminSettings,
} from "@/lib/api";

// AdminSettings 站点设置。
export default function AdminSettings() {
  const [siteName, setSiteName] = useState<string>("");
  const [siteDesc, setSiteDesc] = useState<string>("");
  const [allowRegister, setAllowRegister] = useState<boolean>(true);
  const [commentOpen, setCommentOpen] = useState<boolean>(true);
  const [theme, setTheme] = useState<string>("cool-moon");
  const [maintenanceMode, setMaintenanceMode] = useState<boolean>(false); // 维护开关（M2）
  const [sensitiveWords, setSensitiveWords] = useState<string>(""); // 敏感词（逗号分隔，设计稿）
  const [loaded, setLoaded] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  const [savedText, setSavedText] = useState<string>(""); // 敏感词添加成功提示
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
        setMaintenanceMode(s.maintenance_mode === "on");
      })
      .catch(() => {
        // 读取失败保持默认
      })
      .finally(() => setLoaded(true));
  }, []);

  // 保存（站点设置 + 敏感词批量添加，设计稿「保存设置」按钮）
  const handleSave = async () => {
    setError("");
    setSaved(false);
    setSavedText("");
    try {
      await apiAdminSaveSettings({
        site_name: siteName,
        site_description: siteDesc,
        allow_register: String(allowRegister),
        comment_open: String(commentOpen),
        theme,
        maintenance_mode: maintenanceMode ? "on" : "off",
      });
      // 敏感词（逗号分隔，支持中英文逗号）：批量添加为 forbidden 级别（设计稿字段）
      const words = sensitiveWords
        .split(/[,，]/)
        .map((w) => w.trim())
        .filter(Boolean);
      if (words.length > 0) {
        const result = await apiAdminAddSensitiveWords(words);
        setSensitiveWords("");
        setSavedText(`已添加 ${result.added} 个敏感词`);
        return;
      }
      setSaved(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    }
  };

  return (
    <div className="max-w-[640px]">
      <h1 className="font-display text-xl font-semibold text-ink">站点设置</h1>
      {/* 分区标题（设计稿：站点与用户） */}
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

        {/* 简介（设计稿：简介，如「个人微博客 · 月色」） */}
        <div>
          <label htmlFor="site-desc" className="mb-1.5 block text-sm text-ink-2">
            简介
          </label>
          <textarea
            id="site-desc"
            value={siteDesc}
            onChange={(e) => setSiteDesc(e.target.value)}
            rows={2}
            className="w-full rounded-lg border border-line bg-muted px-4 py-3 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 开关（需求 4.6：开放注册；设计稿：开放评论） */}
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

        {/* 敏感词（设计稿：逗号分隔，批量添加 forbidden 级别） */}
        <div>
          <label htmlFor="sensitive-words" className="mb-1.5 block text-sm text-ink-2">
            敏感词
          </label>
          <input
            id="sensitive-words"
            type="text"
            value={sensitiveWords}
            onChange={(e) => setSensitiveWords(e.target.value)}
            placeholder="多个词用逗号分隔，如：广告,代购,赌博"
            className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <p className="mt-1 text-xs text-ink-3">保存后按拦截级别添加，已存在的自动跳过</p>
        </div>

        {/* 维护模式（M2 全站维护开关：开启后前台接口返回 503，仅后台/登录可用） */}
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-ink">维护模式</p>
            <p className="text-xs text-ink-3">开启后全站前台进入维护页（后台仍可访问）</p>
          </div>
          <button
            type="button"
            onClick={() => setMaintenanceMode((v) => !v)}
            className={`relative h-6 w-11 rounded-full transition-colors ${
              maintenanceMode ? "bg-accent" : "bg-muted"
            }`}
            aria-label="维护模式开关"
          >
            <span
              className={`absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all ${
                maintenanceMode ? "left-[22px]" : "left-0.5"
              }`}
            />
          </button>
        </div>

        {/* 维护模式提示（设计稿《维护中》页：月言正在休整） */}
        {maintenanceMode && (
          <p className="rounded-md bg-like/10 px-3 py-2 text-xs text-like" role="alert">
            维护模式已开启：前台访问将跳转「月言正在休整」维护页，保存后即时生效。
          </p>
        )}

        {/* 默认主题（需求 4.6：cool-moon / mist，M1.7 补充） */}
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
        {savedText && (
          <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
            {savedText}
          </p>
        )}

        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={!loaded}
            className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            保存设置
          </button>
        </div>
      </div>
    </div>
  );
}
