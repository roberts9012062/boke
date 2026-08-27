// frontend/src/components/admin/ai/search-tab.tsx
// AI 设置 · 联网搜索（SearXNG）配置 Tab：介绍 + 地址配置 + 实时测试。
//
// 定位：可选项——未配置时 AI 功能完全正常，仅无联网能力；
//       配置后 AI 对话可联网回答（ai.chat web_search）、开放接口可直查（浏览器插件）。

"use client";

import { useEffect, useState } from "react";

import { apiSearchConfig, apiSearchTest, apiSaveSearchConfig, ApiError, type WebSearchResult } from "@/lib/api";

// SearchTab 联网搜索配置 Tab。
export function SearchTab() {
  const [url, setUrl] = useState<string>("");
  const [savedUrl, setSavedUrl] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);
  const [message, setMessage] = useState<string>("");
  const [error, setError] = useState<string>("");

  // 测试搜索状态
  const [query, setQuery] = useState<string>("");
  const [testing, setTesting] = useState<boolean>(false);
  const [results, setResults] = useState<WebSearchResult[] | null>(null);

  // 加载已保存配置
  useEffect(() => {
    apiSearchConfig()
      .then((cfg) => {
        setUrl(cfg.url);
        setSavedUrl(cfg.url);
      })
      .catch(() => undefined);
  }, []);

  // handleSave 保存配置
  const handleSave = async (): Promise<void> => {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      await apiSaveSearchConfig(url.trim());
      setSavedUrl(url.trim());
      setMessage(url.trim() === "" ? "已停用联网搜索" : "配置已保存");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  // handleTest 实搜验证
  const handleTest = async (): Promise<void> => {
    if (!query.trim()) return;
    setTesting(true);
    setError("");
    setResults(null);
    try {
      const res = await apiSearchTest(query.trim());
      setResults(res.results);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "搜索失败");
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="max-w-2xl space-y-5">
      {/* 为什么配置（介绍文案） */}
      <section className="rounded-lg border border-line bg-muted/40 px-4 py-3">
        <p className="text-sm font-medium text-ink">什么是联网搜索（SearXNG）？</p>
        <p className="mt-1.5 text-xs leading-5 text-ink-2">
          SearXNG 是开源自托管元搜索引擎，一次检索聚合 Google、Bing、DuckDuckGo 等
          数十家结果——本站 Docker 编排已内置其实例，仅容器内网使用，不对外暴露。
        </p>
        <ul className="mt-2 space-y-1 text-xs leading-5 text-ink-2">
          <li>· 配置后：AI 对话可联网回答实时信息（开放接口 ai.chat 加 web_search 参数，回答附引用来源）；浏览器插件等外部应用可凭 API Key 直接调用聚合搜索（开放接口 ai.search）</li>
          <li>· 不配置：AI 全部功能照常使用，仅不具备联网检索能力——<span className="text-ink">完全可选项</span></li>
          <li>· 自托管意味着搜索记录不经第三方聚合服务留存，隐私可控</li>
        </ul>
      </section>

      {/* 地址配置 */}
      <section className="space-y-2">
        <label className="block">
          <span className="mb-1 block text-sm font-medium text-ink">SearXNG 地址</span>
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="http://searxng:8080（本站内置实例）"
            className="w-full rounded-lg border border-line bg-bg px-3 py-2 text-sm text-ink outline-none transition-colors placeholder:text-ink-3 focus:border-accent/50"
          />
        </label>
        <p className="text-xs text-ink-3">
          本站内置实例地址为 <code className="rounded bg-muted px-1">http://searxng:8080</code>（容器内网）；
          也可填任意外部 SearXNG 实例地址。清空保存即停用。
        </p>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={() => void handleSave()}
            disabled={saving || url.trim() === savedUrl}
            className="rounded-lg bg-ink px-4 py-2 text-sm text-white transition-colors hover:bg-ink/85 disabled:opacity-50"
          >
            {saving ? "保存中…" : "保存"}
          </button>
          {message && <span className="text-xs text-emerald-600">{message}</span>}
        </div>
      </section>

      {/* 实时测试 */}
      <section className="space-y-2 border-t border-line pt-4">
        <p className="text-sm font-medium text-ink">测试搜索</p>
        <div className="flex gap-2">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="输入关键词实测（如：今日科技新闻）"
            className="flex-1 rounded-lg border border-line bg-bg px-3 py-2 text-sm text-ink outline-none transition-colors placeholder:text-ink-3 focus:border-accent/50"
          />
          <button
            type="button"
            onClick={() => void handleTest()}
            disabled={testing || !query.trim()}
            className="shrink-0 rounded-lg border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:border-accent/40 hover:text-ink disabled:opacity-50"
          >
            {testing ? "搜索中…" : "测试"}
          </button>
        </div>

        {error && <p className="rounded-lg bg-red-500/10 px-3 py-2 text-xs text-red-600">{error}</p>}

        {results && (
          <ul className="space-y-2">
            {results.map((item, index) => (
              <li key={index} className="rounded-lg border border-line px-3 py-2">
                <a href={item.url} target="_blank" rel="noreferrer" className="text-sm font-medium text-ink hover:text-glow">
                  {item.title}
                </a>
                <p className="mt-0.5 line-clamp-2 text-xs text-ink-2">{item.snippet}</p>
                <p className="mt-0.5 truncate text-[11px] text-ink-3">{item.url}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
