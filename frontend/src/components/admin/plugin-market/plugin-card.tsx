// src/components/admin/plugin-market/plugin-card.tsx
// 商城插件卡片（设计稿：名称/类别/安装量/官方/描述/价格按钮）：
// 标题徽章 + 兼容性契约 + 描述 + 「详情」入口 + 安装/已装操作区。
"use client";

import Link from "next/link";

import type { MarketPlugin } from "@/lib/api";

import { categoryLabel, formatInstalls } from "./labels";

// PluginCard 插件卡片（纯展示组件，安装/详情动作回调给页面）。
export function PluginCard({
  plugin,
  onDetail,
  onInstall,
}: {
  plugin: MarketPlugin; // 商城插件（含已安装状态）
  onDetail: (plugin: MarketPlugin) => void; // 打开详情弹窗
  onInstall: (plugin: MarketPlugin) => void; // 打开安装弹层
}) {
  return (
    <div className="rounded-lg border border-line bg-elevated p-5">
      <div className="flex items-start justify-between gap-2">
        <p className="font-display text-base font-semibold text-ink">{plugin.name}</p>
        <div className="flex shrink-0 gap-1">
          {plugin.official && (
            <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[10px] text-glow">官方</span>
          )}
          <span className="rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">
            {categoryLabel(plugin.category)}
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

      {/* 详情入口（M5：查看插件 README 介绍，渲染 Markdown） */}
      <button
        type="button"
        onClick={() => onDetail(plugin)}
        className="mt-2 rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:border-accent hover:text-glow"
      >
        详情
      </button>

      <div className="mt-4">
        {plugin.installed ? (
          <div className="flex items-center justify-between gap-2 text-xs">
            <span className="rounded-full bg-accent-soft px-2.5 py-1 text-glow">
              {plugin.state === "disabled" ? "已禁用" : "本站已启用"}
            </span>
            <div className="flex items-center gap-2">
              <span className="text-ink-3">已装</span>
              {/* 打开设置（设计稿《已装SEO》：schema 驱动设置页，M3.2；详情接口按实例 ID 路由） */}
              {(plugin.settings_schema?.length ?? 0) > 0 ? (
                <Link
                  href={`/admin/plugins/${plugin.instance_id}/settings`}
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
            onClick={() => onInstall(plugin)}
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
              onClick={() => onInstall(plugin)}
              className="flex-1 rounded-full bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:opacity-90"
            >
              安装
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
