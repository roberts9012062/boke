// src/components/admin/plugin-market/plugin-install-modal.tsx
// 商城安装弹层（设计稿：免费=能力清单+确认安装；付费=永久授权+支付 ¥xx 并安装+复制授权码）。
// 从 plugin-market/page.tsx 抽出（M5 拆分，页面瘦身）。
"use client";

import { Modal } from "@/components/ui/modal";
import type { MarketPlugin } from "@/lib/api";

import { categoryLabel } from "./labels";

// PluginInstallModal 安装/购买弹层。
// 参数：target 目标插件（null=关闭）；installing 安装中；installed 成功态；
//      error 失败提示；onClose 关闭回调；onConfirm 确认安装/支付回调。
export function PluginInstallModal({
  target,
  installing,
  installed,
  error,
  onClose,
  onConfirm,
}: {
  target: MarketPlugin | null;
  installing: boolean;
  installed: boolean;
  error: string;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <Modal
      open={target !== null}
      title={target ? `${target.price > 0 ? "购买" : "安装"} ${target.name}` : ""}
      onClose={onClose}
      maxWidth="max-w-[420px]"
    >
      {target && (
        installed ? (
          // 安装成功（设计稿《插件安装·成功》）
          <div className="py-6 text-center">
            <p className="text-4xl" aria-hidden>
              ✓
            </p>
            <h2 className="mt-3 font-display text-lg font-semibold text-ink">「{target.name}」安装成功</h2>
            <p className="mt-1 text-xs text-ink-3">已启用，可在「我的插件」中管理</p>
            <button
              type="button"
              onClick={onClose}
              className="mt-5 rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent hover:opacity-90"
            >
              完成
            </button>
          </div>
        ) : (
          <>
            <p className="text-xs text-ink-3">
              {categoryLabel(target.category)} · {target.price > 0 ? "付费" : "免费"} · v{target.version}
            </p>

            {/* 能力清单（设计稿：站点地图/元信息/Open Graph/robots.txt） */}
            <ul className="mt-4 space-y-1.5">
              {target.capabilities.map((cap) => (
                <li key={cap} className="flex items-center gap-2 text-sm text-ink-2">
                  <span className="text-glow" aria-hidden>
                    ✓
                  </span>
                  {cap}
                </li>
              ))}
              {target.price > 0 && (
                <li className="flex items-center gap-2 text-sm text-ink-2">
                  <span className="text-glow" aria-hidden>
                    ✓
                  </span>
                  永久授权
                </li>
              )}
            </ul>

            {error && (
              <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
                {error}
              </p>
            )}

            <div className="mt-5 flex items-center justify-end gap-3">
              <button
                type="button"
                onClick={onClose}
                disabled={installing}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
              >
                取消
              </button>
              <button
                type="button"
                onClick={onConfirm}
                disabled={installing}
                className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
              >
                {installing ? "安装中…" : target.price > 0 ? `支付 ¥${target.price} 并安装` : "确认安装"}
              </button>
            </div>
            {target.price > 0 && (
              <p className="mt-3 text-center text-[10px] text-ink-3">
                开发环境模拟支付；支付成功由服务端签发许可证并自动激活 Pro 授权
              </p>
            )}
          </>
        )
      )}
    </Modal>
  );
}
