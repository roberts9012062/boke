// src/app/admin/plugin-market/page.tsx
// 后台插件商城（设计稿 D/冷月/插件商城 1400×1100）：
// 插件源仓库（GitHub 自定义，settings 持久化）→ 二级分类 Tab（一级：全部/免费/付费/已安装；
// 二级：SEO·增长/内容增强/安全/性能/分析/写作/运维）+ 搜索 + 插件卡片
// + 详情弹窗（插件 README 渲染 Markdown，M5）+ 安装弹层（免费/付费/成功，M3.9 支付）。
"use client";

import { useEffect, useState } from "react";

import { PluginCard } from "@/components/admin/plugin-market/plugin-card";
import { PluginDetailModal } from "@/components/admin/plugin-market/plugin-detail-modal";
import { PluginInstallModal } from "@/components/admin/plugin-market/plugin-install-modal";
import { ProxySettings } from "@/components/admin/plugin-market/proxy-settings";
import { CATEGORIES, CATEGORY_LABEL } from "@/components/admin/plugin-market/labels";
import {
  apiAdminSaveSettings,
  apiCreatePluginOrder,
  apiInstallPlugin,
  apiPayPluginOrder,
  apiPluginMarket,
  apiPluginOAuthAuthorize,
  apiPluginOAuthDisconnect,
  apiPluginOAuthStatus,
  ApiError,
  type MarketPlugin,
} from "@/lib/api";

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
  // 详情弹窗（M5）
  const [detailTarget, setDetailTarget] = useState<MarketPlugin | null>(null);
  // 安装弹层
  const [installTarget, setInstallTarget] = useState<MarketPlugin | null>(null);
  const [installing, setInstalling] = useState<boolean>(false);
  const [installed, setInstalled] = useState<boolean>(false);
  const [installError, setInstallError] = useState<string>("");
  // GitHub 连接（M3.5：OAuth App 凭证配置后启用）
  const [oauthEnabled, setOAuthEnabled] = useState<boolean>(false);
  const [oauthConnected, setOAuthConnected] = useState<boolean>(false);
  const [oauthUsername, setOAuthUsername] = useState<string>("");

  // 加载 OAuth 连接状态（凭证未配置时隐藏入口）
  useEffect(() => {
    apiPluginOAuthStatus()
      .then((r) => {
        setOAuthEnabled(r.status.enabled);
        setOAuthConnected(r.status.connected);
        setOAuthUsername(r.status.username ?? "");
      })
      .catch(() => {
        /* 接口不可用静默（匿名模式不受影响） */
      });
  }, []);

  // 发起 GitHub 连接（跳转授权页）
  const connectOAuth = async () => {
    try {
      const r = await apiPluginOAuthAuthorize();
      if (r.url) window.location.href = r.url;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "连接失败");
    }
  };

  // 断开 GitHub 连接
  const disconnectOAuth = async () => {
    try {
      await apiPluginOAuthDisconnect();
      setOAuthConnected(false);
      setOAuthUsername("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "断开失败");
    }
  };

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

  // 打开安装弹层（重置状态）
  const openInstall = (plugin: MarketPlugin) => {
    setInstallTarget(plugin);
    setInstalled(false);
    setInstallError("");
  };

  // 安装（免费直接安装；付费：安装 → 创建订单 → 支付签发激活——M3.9 支付渠道，
  // dev 模拟支付直接成功并由服务端签发许可证自动激活）
  const confirmInstall = async () => {
    if (!installTarget) return;
    setInstalling(true);
    setInstallError("");
    try {
      await apiInstallPlugin(installTarget.id);
      if (installTarget.price > 0) {
        // 付费插件：创建订单 → 支付（模拟）→ 服务端签发许可证并自动激活
        const fresh = await apiPluginMarket(source);
        const item = fresh.items.find((p) => p.id === installTarget.id);
        if (item?.instance_id) {
          const order = await apiCreatePluginOrder(item.instance_id, installTarget.price);
          await apiPayPluginOrder(order.order_id);
        }
      }
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

      {/* 插件源仓库（GitHub 自定义，M3.1 需求：用户可填写仓库拉取清单） */}
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
        {/* GitHub 连接（M3.5：OAuth App 凭证配置后启用；未连接时匿名模式拉取公开仓库） */}
        {oauthEnabled && (
          <span className="ml-2 inline-flex items-center gap-2 border-l border-line pl-3 text-xs">
            {oauthConnected ? (
              <>
                <span className="text-glow">✓ 已连接 {oauthUsername}</span>
                <button
                  type="button"
                  onClick={() => void disconnectOAuth()}
                  className="text-ink-3 hover:text-like"
                >
                  断开
                </button>
              </>
            ) : (
              <button
                type="button"
                onClick={() => void connectOAuth()}
                className="text-glow hover:underline"
              >
                连接 GitHub
              </button>
            )}
          </span>
        )}
      </div>

      {/* 加速代理（国内网络直连 GitHub 失败时选择；保存后重新拉取商城） */}
      <ProxySettings onApplied={() => load(source)} />

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
            <PluginCard
              key={plugin.id}
              plugin={plugin}
              onDetail={setDetailTarget}
              onInstall={openInstall}
            />
          ))}
        </div>
      )}
      {!loading && filtered.length === 0 && (
        <p className="mt-10 text-center text-sm text-ink-3">没有匹配的插件</p>
      )}

      {/* 详情弹窗（M5：插件 README 介绍，Markdown 渲染） */}
      <PluginDetailModal
        plugin={detailTarget}
        source={source}
        onClose={() => setDetailTarget(null)}
      />

      {/* 安装弹层（设计稿：免费=能力清单+确认安装；付费=永久授权+支付 ¥xx 并安装） */}
      <PluginInstallModal
        target={installTarget}
        installing={installing}
        installed={installed}
        error={installError}
        onClose={() => {
          if (!installing) setInstallTarget(null);
        }}
        onConfirm={() => void confirmInstall()}
      />
    </div>
  );
}
