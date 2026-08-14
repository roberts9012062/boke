// src/components/admin/plugin-market/plugin-detail-modal.tsx
// 商城插件详情弹窗（M5）：打开时拉取插件 README 原文（后端代理公开插件仓库），
// 用 Markdown 组件渲染展示（表格/列表/代码块），内容区可滚动。
"use client";

import { useEffect, useState } from "react";

import { Markdown } from "@/components/markdown";
import { Modal } from "@/components/ui/modal";
import { ApiError, apiPluginMarketReadme, type MarketPlugin } from "@/lib/api";

import { categoryLabel, formatInstalls } from "./labels";

// PluginDetailModal 插件详情弹窗。
// 参数：plugin 目标插件（null=关闭）；source 当前生效插件源；onClose 关闭回调。
export function PluginDetailModal({
  plugin,
  source,
  onClose,
}: {
  plugin: MarketPlugin | null;
  source: string;
  onClose: () => void;
}) {
  const [readme, setReadme] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 打开（或切换插件）时拉取 README；卸载/切换时丢弃过期响应
  useEffect(() => {
    if (!plugin) {
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError("");
    setReadme("");
    apiPluginMarketReadme(plugin.id, source)
      .then((r) => {
        if (!cancelled) {
          setReadme(r.readme);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof ApiError ? err.message : "拉取插件介绍失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [plugin, source]);

  return (
    <Modal
      open={plugin !== null}
      title={plugin ? `插件详情 · ${plugin.name}` : "插件详情"}
      onClose={onClose}
      maxWidth="max-w-[640px]"
    >
      {plugin && (
        <>
          {/* 插件头信息（类别/版本/安装量/价格/官方） */}
          <div className="flex flex-wrap items-center gap-2 text-xs text-ink-3">
            <span className="rounded-full bg-muted px-2 py-0.5 text-[10px]">{categoryLabel(plugin.category)}</span>
            <span>v{plugin.version}</span>
            <span>{formatInstalls(plugin.installs)} 安装</span>
            <span>{plugin.price > 0 ? `付费 ¥${plugin.price}` : "免费"}</span>
            {plugin.official && (
              <span className="rounded-full bg-accent-soft px-2 py-0.5 text-[10px] text-glow">官方</span>
            )}
          </div>

          {/* 内容区：加载骨架 / 错误 / 渲染后的 Markdown */}
          <div className="mt-3 max-h-[65vh] overflow-y-auto border-t border-line pt-3">
            {loading ? (
              <div className="space-y-2 py-2">
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className="h-4 animate-pulse rounded bg-muted" aria-hidden />
                ))}
              </div>
            ) : error ? (
              <p className="py-4 text-center text-sm text-ink-3">{error}</p>
            ) : readme ? (
              <Markdown content={readme} />
            ) : (
              <p className="py-4 text-center text-sm text-ink-3">暂无介绍</p>
            )}
          </div>

          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={onClose}
              className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
            >
              关闭
            </button>
          </div>
        </>
      )}
    </Modal>
  );
}
