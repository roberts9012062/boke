// src/components/admin/nav/nav-item-modal.tsx
// 导航项创建/编辑面板（Modal）：
//   名称 + 链接类型二选一 ——「URL 自定义」（手输站内/外链）/「内部页面」（下拉选择已发布
//   自定义页面，自动填 /pages/{slug}）+ 外链新窗口开关；一级项 URL 可留空（纯分组）。
"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/modal";
import type { AdminPageItem } from "@/lib/api-pages";
import type { SiteNavLink } from "@/types/api";

// NavItemModalProps 面板参数。
interface NavItemModalProps {
  open: boolean; // 是否显示
  title: string; // 面板标题（创建一级/二级、编辑一级/二级）
  initial: SiteNavLink | null; // 编辑回显（创建为 null）
  requireURL: boolean; // true = 二级项（地址必填）；一级项可留空作纯分组
  publishedPages: AdminPageItem[]; // 已发布自定义页面（「内部页面」下拉数据源）
  onSubmit: (item: SiteNavLink) => void; // 提交（已通过前端校验）
  onClose: () => void; // 关闭
}

// 链接类型：「URL 自定义」手输 /「内部页面」下拉。
type LinkSource = "custom" | "internal";

// NavItemModal 导航项创建/编辑面板。
export function NavItemModal({
  open,
  title,
  initial,
  requireURL,
  publishedPages,
  onSubmit,
  onClose,
}: NavItemModalProps) {
  const [label, setLabel] = useState<string>("");
  const [source, setSource] = useState<LinkSource>("custom");
  const [url, setUrl] = useState<string>("");
  const [pageSlug, setPageSlug] = useState<string>("");
  const [newTab, setNewTab] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 打开时回显（编辑：URL 以 /pages/ 开头且能匹配已发布页面 → 内部类型）
  useEffect(() => {
    if (!open) {
      return;
    }
    setError("");
    setLabel(initial?.label ?? "");
    setNewTab(initial?.new_tab ?? false);
    const matched = publishedPages.find(
      (p) => initial?.url === `/pages/${p.slug}`,
    );
    if (initial && matched) {
      setSource("internal");
      setPageSlug(matched.slug);
      setUrl("");
    } else {
      setSource("custom");
      setPageSlug("");
      setUrl(initial?.url ?? "");
    }
  }, [open, initial, publishedPages]);

  // 提交（名称必填；二级地址必填；自定义 URL 走前端轻校验，后端仍有协议白名单）
  const handleSubmit = (): void => {
    if (!label.trim()) {
      setError("导航名称不能为空");
      return;
    }
    const finalURL =
      source === "internal" ? `/pages/${pageSlug}` : url.trim();
    if (requireURL && !finalURL) {
      setError("二级导航必须填写地址或选择内部页面");
      return;
    }
    if (
      source === "custom" &&
      finalURL &&
      !finalURL.startsWith("/") &&
      !finalURL.startsWith("http://") &&
      !finalURL.startsWith("https://")
    ) {
      setError("地址须以 / 开头（站内）或 http(s):// 开头（外链）");
      return;
    }
    onSubmit({
      label: label.trim(),
      url: finalURL,
      new_tab: newTab,
      children: initial?.children,
    });
  };

  return (
    <Modal open={open} title={title} onClose={onClose}>
      <div className="space-y-4">
        {/* 名称 */}
        <div>
          <label className="mb-1.5 block text-sm text-ink-2">导航名称</label>
          <input
            type="text"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            maxLength={30}
            placeholder="如：首页、关于本站"
            className="h-10 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
          />
        </div>

        {/* 链接类型（URL 自定义 / 内部页面） */}
        <div>
          <p className="mb-1.5 text-sm text-ink-2">链接</p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setSource("custom")}
              className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                source === "custom"
                  ? "border-accent bg-accent-soft text-glow"
                  : "border-line text-ink-2 hover:text-ink"
              }`}
            >
              URL 自定义
            </button>
            <button
              type="button"
              onClick={() => setSource("internal")}
              disabled={publishedPages.length === 0}
              className={`rounded-full border px-4 py-1.5 text-sm transition-colors disabled:opacity-40 ${
                source === "internal"
                  ? "border-accent bg-accent-soft text-glow"
                  : "border-line text-ink-2 hover:text-ink"
              }`}
            >
              内部页面
            </button>
          </div>

          {source === "custom" ? (
            <div className="mt-2">
              <input
                type="text"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                maxLength={500}
                placeholder={requireURL ? "/topics 或 https://…" : "留空则为纯分组（仅展开二级）"}
                className="h-10 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
              />
            </div>
          ) : (
            <div className="mt-2">
              <select
                value={pageSlug}
                onChange={(e) => setPageSlug(e.target.value)}
                className="h-10 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
              >
                <option value="">选择已发布页面…</option>
                {publishedPages.map((p) => (
                  <option key={p.id} value={p.slug}>
                    {p.title}（/pages/{p.slug}）
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>

        {/* 外链新窗口 */}
        <label className="flex items-center gap-2 text-sm text-ink-2">
          <input
            type="checkbox"
            checked={newTab}
            onChange={(e) => setNewTab(e.target.checked)}
            className="h-3.5 w-3.5"
          />
          外部链接在新窗口打开
        </label>

        {error && (
          <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            取消
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            确定
          </button>
        </div>
      </div>
    </Modal>
  );
}
