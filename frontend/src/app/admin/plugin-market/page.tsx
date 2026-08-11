// src/app/admin/plugin-market/page.tsx
// 后台插件商城（设计稿 D/冷月/插件商城 1400×1100）：
// 插件源仓库（GitHub 自定义，settings 持久化）→ 二级分类 Tab（一级：全部/免费/付费/已安装；
// 二级：SEO·增长/内容增强/安全/性能/分析/写作/运维）+ 搜索 + 插件卡片（类别/安装量/官方/描述/价格）
// + 安装弹层（免费：能力清单+确认安装；付费：永久授权+支付 ¥xx 并安装+复制授权码模拟）→ Loading → 成功。
"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import {
  apiAdminSaveSettings,
  apiInstallPlugin,
  apiPluginMarket,
  ApiError,
  type MarketPlugin,
} from "@/lib/api";

// 类别标签（设计稿二级 Tab：SEO·增长/内容增强/安全/性能/分析/写作/运维）
const CATEGORY_LABEL: Record<string, string> = {
  seo: "SEO / 增长",
  enhancement: "内容增强",
  security: "安全",
  performance: "性能",
  analytics: "分析",
  writing: "写作",
  ops: "运维",
};

// 二级类别列表（设计稿顺序）
const CATEGORIES = ["seo", "enhancement", "security", "performance", "analytics", "writing", "ops"] as const;

// formatInstalls 安装量缩写（设计稿：12.4k）。
function formatInstalls(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}

// PluginMarketPage 插件商城。
export default function PluginMarketPage() {
  const [items, setItems] = useState<MarketPlugin[]>([]);
  const [marketName, setMarketName] = useState<string>("插件商城");
  const [source, setSource] = useState<string>(""); // 当前生效源
  const [sourceInput, setSourceInput] = useState<string>(""); // 输入框
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  // 筛选：一级 Tab（all/free/paid/installed）+ 二级类别（""=全部）
  const [tab, setTab] = useState<string>("all");
  const [category, setCategory] = useState<string>("");
  const [keyword, setKeyword] = useState<string>("");
  // 安装弹层
  const [installTarget, setInstallTarget] = useState<MarketPlugin | null>(null);
  const [installing, setInstalling] = useState<boolean>(false);
  const [installed, setInstalled] = useState<boolean>(false);
  const [installError, setInstallError] = useState<string>("");

  // 加载商城（源变化时）
  const load = (src: string) => {
    setLoading(true);
    setError("");
    apiPluginMarket(src)
      .then((r) => {
        setItems(r.items);
        setMarketName(r.name);
        setSource(r.source);
      })
      .catch((err) => {
        setItems([]);
        setError(err instanceof ApiError ? err.message : "拉取插件清单失败");
      })
      .finally(() => setLoading(false));
  };
  useEffect(() => {
    load("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 保存自定义插件源（settings.plugin_source → 重新拉取）
  const saveSource = async () => {
    const src = sourceInput.trim();
    if (!src) {
      setError("请输入插件源仓库（owner/repo）");
      return;
    }
    setError("");
    try {
      await apiAdminSaveSettings({ plugin_source: src });
      load(src);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存插件源失败");
    }
  };

  // 过滤（一级 Tab × 二级类别 × 搜索）
  const filtered = items.filter((p) => {
    if (tab === "free" && p.price > 0) return false;
    if (tab === "paid" && p.price === 0) return false;
    if (tab === "installed" && !p.installed) return false;
    if (category && p.category !== category) return false;
    if (keyword && !(p.name + p.description).includes(keyword)) return false;
    return true;
  });

  // 安装（免费直接安装；付费模拟支付——MVP 无真实支付，点击「支付并安装」即完成，差异记录）
  const confirmInstall = async () => {
    if (!installTarget) return;
    setInstalling(true);
    setInstallError("");
    try {
      await apiInstallPlugin(installTarget.id);
      setInstalled(true);
      // 刷新列表（已安装状态）
      const r = await apiPluginMarket(source);
      setItems(r.items);
    } catch (err) {
      setInstallError(err instanceof ApiError ? err.message : "安装失败");
    } finally {
      setInstalling(false);
    }
  };

  const installedCount = items.filter((p) => p.installed).length;

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">插件商城</h1>
      <p className="mt-0.5 text-xs text-ink-3">
        {installedCount > 0 ? `已安装 ${installedCount} 个插件 · 官方与社区扩展` : "扩展博客能力 · 官方与社区插件"}
      </p>

      {/* 插件源仓库（GitHub 自定义，设计稿未含——M3.1 需求：用户可填写仓库拉取清单） */}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <span className="text-xs text-ink-3">插件源：</span>
        <input
          type="text"
          value={sourceInput}
          onChange={(e) => setSourceInput(e.target.value)}
          placeholder="owner/repo（GitHub 仓库，默认 roberts9012062/yueyan-plugins）"
          className="h-9 w-72 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <button
          type="button"
          onClick={() => void saveSource()}
          className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
        >
          拉取
        </button>
        <span className="text-xs text-ink-3">
          当前源：{source || "roberts9012062/yueyan-plugins"} · {marketName}
        </span>
      </div>

      {/* 搜索 + 一级分类 Tab（设计稿：全部/免费/付费/已安装） */}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索插件…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <div className="flex gap-1 rounded-full border border-line p-0.5">
          {[
            { key: "all", label: "全部" },
            { key: "free", label: "免费" },
            { key: "paid", label: "付费" },
            { key: "installed", label: "已安装" },
          ].map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setTab(t.key)}
              className={`rounded-full px-4 py-1 text-sm transition-colors ${
                tab === t.key ? "bg-accent-soft font-medium text-glow" : "text-ink-2 hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* 二级分类 Tab（设计稿：SEO·增长/内容增强/安全/性能/分析/写作/运维） */}
      <div className="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => setCategory("")}
          className={`rounded-full px-4 py-1 text-xs transition-colors ${
            category === "" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
          }`}
        >
          全部类别
        </button>
        {CATEGORIES.map((c) => (
          <button
            key={c}
            type="button"
            onClick={() => setCategory(category === c ? "" : c)}
            className={`rounded-full px-4 py-1 text-xs transition-colors ${
              category === c ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
            }`}
          >
            {CATEGORY_LABEL[c]}
          </button>
        ))}
      </div>

      {error && (
        <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 插件卡片网格（设计稿：名称/类别/安装量/官方/描述/价格按钮） */}
      {loading ? (
        <div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-44 animate-pulse rounded-lg border border-line bg-muted" aria-hidden />
          ))}
        </div>
      ) : (
        <div className="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((plugin) => (
            <div key={plugin.id} className="rounded-lg border border-line bg-elevated p-5">
              <div className="flex items-start justify-between gap-2">
                <p className="font-display text-base font-semibold text-ink">{plugin.name}</p>
                <div className="flex shrink-0 gap-1">
                  {plugin.official && (
                    <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[10px] text-glow">官方</span>
                  )}
                  <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">
                    {CATEGORY_LABEL[plugin.category] ?? plugin.category}
                  </span>
                </div>
              </div>
              <p className="mt-1 text-xs text-ink-3">{formatInstalls(plugin.installs)} 安装 · v{plugin.version}</p>
              {/* 兼容性契约（M3.2：core_version / requires / conflicts） */}
              <p className="mt-1 text-[10px] text-ink-3">
                {plugin.core_version ? `兼容核心 ${plugin.core_version}` : "兼容任意核心版本"}
                {plugin.requires?.length ? ` · 依赖 ${plugin.requires.join(", ")}` : ""}
                {plugin.conflicts?.length ? ` · 冲突 ${plugin.conflicts.join(", ")}` : ""}
              </p>
              <p className="mt-2 line-clamp-2 text-sm text-ink-2">{plugin.description}</p>

              <div className="mt-4">
                {plugin.installed ? (
                  <div className="flex items-center justify-between gap-2 text-xs">
                    <span className="rounded-full bg-accent-soft px-2.5 py-1 text-glow">
                      {plugin.state === "disabled" ? "已禁用" : "本站已启用"}
                    </span>
                    <div className="flex items-center gap-2">
                      <span className="text-ink-3">已装</span>
                      {/* 打开设置（设计稿《已装SEO》：schema 驱动设置页，M3.2） */}
                      {(plugin.settings_schema?.length ?? 0) > 0 ? (
                        <Link
                          href={`/admin/plugins/${plugin.id}/settings`}
                          className="rounded-full border border-line px-3 py-1 text-ink-2 hover:text-ink"
                        >
                          打开设置
                        </Link>
                      ) : (
                        <span className="text-[10px] text-ink-3">无配置项</span>
                      )}
                    </div>
                  </div>
                ) : plugin.price > 0 ? (
                  <button
                    type="button"
                    onClick={() => {
                      setInstallTarget(plugin);
                      setInstalled(false);
                      setInstallError("");
                    }}
                    className="w-full rounded-full border border-accent px-4 py-2 text-sm font-medium text-glow hover:bg-accent-soft"
                  >
                    购买并安装 · ¥{plugin.price}
                  </button>
                ) : (
                  <div className="flex items-center gap-2">
                    {/* 免费标签（设计稿：免费 + 安装） */}
                    <span className="rounded-full bg-muted px-2.5 py-1 text-xs text-ink-3">免费</span>
                    <button
                      type="button"
                      onClick={() => {
                        setInstallTarget(plugin);
                        setInstalled(false);
                        setInstallError("");
                      }}
                      className="flex-1 rounded-full bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:opacity-90"
                    >
                      安装
                    </button>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      {!loading && filtered.length === 0 && (
        <p className="mt-10 text-center text-sm text-ink-3">没有匹配的插件</p>
      )}

      {/* 安装弹层（设计稿：免费=能力清单+确认安装；付费=永久授权+支付 ¥xx 并安装+复制授权码） */}
      {installTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="安装插件"
          onClick={() => {
            if (!installing) setInstallTarget(null);
          }}
        >
          <div
            className="w-full max-w-[420px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            {installed ? (
              // 安装成功（设计稿《插件安装·成功》）
              <div className="py-6 text-center">
                <p className="text-4xl" aria-hidden>
                  ✓
                </p>
                <h2 className="mt-3 font-display text-lg font-semibold text-ink">
                  「{installTarget.name}」安装成功
                </h2>
                <p className="mt-1 text-xs text-ink-3">已启用，可在「我的插件」中管理</p>
                <button
                  type="button"
                  onClick={() => setInstallTarget(null)}
                  className="mt-5 rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent hover:opacity-90"
                >
                  完成
                </button>
              </div>
            ) : (
              <>
                <h2 className="font-display text-lg font-semibold text-ink">
                  {installTarget.price > 0 ? "购买" : "安装"} {installTarget.name}
                </h2>
                <p className="mt-1 text-xs text-ink-3">
                  {CATEGORY_LABEL[installTarget.category] ?? installTarget.category} ·{" "}
                  {installTarget.price > 0 ? "付费" : "免费"} · v{installTarget.version}
                </p>

                {/* 能力清单（设计稿：站点地图/元信息/Open Graph/robots.txt） */}
                <ul className="mt-4 space-y-1.5">
                  {installTarget.capabilities.map((cap) => (
                    <li key={cap} className="flex items-center gap-2 text-sm text-ink-2">
                      <span className="text-glow" aria-hidden>
                        ✓
                      </span>
                      {cap}
                    </li>
                  ))}
                  {installTarget.price > 0 && (
                    <li className="flex items-center gap-2 text-sm text-ink-2">
                      <span className="text-glow" aria-hidden>
                        ✓
                      </span>
                      永久授权
                    </li>
                  )}
                </ul>

                {installError && (
                  <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
                    {installError}
                  </p>
                )}

                <div className="mt-5 flex items-center justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setInstallTarget(null)}
                    disabled={installing}
                    className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    onClick={() => void confirmInstall()}
                    disabled={installing}
                    className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
                  >
                    {installing
                      ? "安装中…"
                      : installTarget.price > 0
                        ? `支付 ¥${installTarget.price} 并安装`
                        : "确认安装"}
                  </button>
                </div>
                {installTarget.price > 0 && (
                  <p className="mt-3 text-center text-[10px] text-ink-3">
                    MVP 模拟支付（无真实支付渠道，差异记录）；授权码复制功能后置
                  </p>
                )}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
