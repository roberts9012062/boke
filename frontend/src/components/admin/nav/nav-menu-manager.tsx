// src/components/admin/nav/nav-menu-manager.tsx
// 头部导航树编辑器（自包含加载/保存）：一级列表可折叠展开二级，
// 支持排序/编辑/删除；「创建」弹出 NavItemModal（URL 自定义 / 内部页面下拉）。
// 保存到 settings.nav_links（两级 JSON），前台头部即时生效（见 desktop-nav.tsx）。
"use client";

import { useEffect, useState } from "react";

import { NavItemModal } from "@/components/admin/nav/nav-item-modal";
import { ApiError, apiAdminSaveSettings, apiAdminSettings } from "@/lib/api";
import { apiAdminPages } from "@/lib/api-pages";
import type { AdminPageItem } from "@/lib/api-pages";
import { useSitePluginNav } from "@/lib/site-plugin-nav";
import { invalidateSiteMeta } from "@/lib/site-meta";
import type { SiteNavLink } from "@/types/api";

// ModalTarget 当前面板目标（null = 关闭）。
// kind：create-top 新一级 / create-child 某一级下新增二级 / edit-top / edit-child。
interface ModalTarget {
  kind: "create-top" | "create-child" | "edit-top" | "edit-child";
  topIndex: number; // 所属一级索引（create-top/edit-top 为 -1）
  childIndex: number; // 二级索引（非 edit-child 为 -1）
}

// NavMenuManager 导航树编辑器。
export function NavMenuManager() {
  const [items, setItems] = useState<SiteNavLink[]>([]);
  const [pages, setPages] = useState<AdminPageItem[]>([]);
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [loaded, setLoaded] = useState<boolean>(false);
  const [saving, setSaving] = useState<boolean>(false);
  const [saved, setSaved] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [target, setTarget] = useState<ModalTarget | null>(null);
  // 插件注册的前台导航项（只读展示：由插件清单 siteNav 声明，不在此编辑）
  const pluginNav = useSitePluginNav();

  // 加载导航配置 + 已发布页面（「内部页面」下拉数据源）
  useEffect(() => {
    apiAdminSettings()
      .then((s) => {
        try {
          const parsed = s.nav_links ? (JSON.parse(s.nav_links) as SiteNavLink[]) : [];
          setItems(parsed);
        } catch {
          setItems([]); // 历史脏数据兜底：按未配置处理
        }
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoaded(true));
    apiAdminPages()
      .then((r) => setPages((r.items ?? []).filter((p) => p.status === "published")))
      .catch(() => undefined);
  }, []);

  // ---------- 树操作（全部纯函数式更新，不修改入参） ----------

  // move 移动数组元素（offset=-1 上移 / 1 下移；越界不动）。
  const move = (list: SiteNavLink[], index: number, offset: number): SiteNavLink[] => {
    const to = index + offset;
    if (index < 0 || to < 0 || to >= list.length) {
      return list;
    }
    const next = [...list];
    [next[index], next[to]] = [next[to], next[index]];
    return next;
  };

  const moveTop = (i: number, offset: number): void => {
    setItems((prev) => move(prev, i, offset));
  };

  const moveChild = (i: number, j: number, offset: number): void => {
    setItems((prev) =>
      prev.map((top, idx) =>
        idx === i ? { ...top, children: move(top.children ?? [], j, offset) } : top,
      ),
    );
  };

  const deleteTop = (i: number): void => {
    setItems((prev) => prev.filter((_, idx) => idx !== i));
  };

  const deleteChild = (i: number, j: number): void => {
    setItems((prev) =>
      prev.map((top, idx) =>
        idx === i ? { ...top, children: (top.children ?? []).filter((_, c) => c !== j) } : top,
      ),
    );
  };

  // toggleCollapse 折叠/展开一级项。
  const toggleCollapse = (i: number): void => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(i)) {
        next.delete(i);
      } else {
        next.add(i);
      }
      return next;
    });
  };

  // ---------- 面板提交（按目标写回树） ----------
  const handleModalSubmit = (item: SiteNavLink): void => {
    if (!target) {
      return;
    }
    setItems((prev) => {
      const next = prev.map((top) => ({ ...top, children: [...(top.children ?? [])] }));
      if (target.kind === "create-top") {
        next.push({ ...item, children: [] });
      } else if (target.kind === "create-child") {
        next[target.topIndex].children.push({ ...item });
      } else if (target.kind === "edit-top") {
        // 保留二级列表（item.children 为面板回显值；面板可能丢失时兜底空数组）
        next[target.topIndex] = { ...item, children: item.children ?? [] };
      } else {
        next[target.topIndex].children[target.childIndex] = item;
      }
      return next;
    });
    setTarget(null);
    setSaved(false);
  };

  // ---------- 保存（空列表 = 清空恢复默认） ----------
  const handleSave = async () => {
    setError("");
    setSaved(false);
    setSaving(true);
    try {
      await apiAdminSaveSettings({ nav_links: JSON.stringify(items) });
      invalidateSiteMeta(); // 前台导航缓存失效，下次读取拉新值
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  // 当前编辑目标的原值（面板回显）
  const editInitial: SiteNavLink | null = (() => {
    if (!target || target.kind === "create-top" || target.kind === "create-child") {
      return null;
    }
    if (target.kind === "edit-top") {
      return items[target.topIndex] ?? null;
    }
    return items[target.topIndex]?.children?.[target.childIndex] ?? null;
  })();

  return (
    <div className="mt-5">
      {/* 操作区：创建一级 + 保存 */}
      <div className="mb-4 flex items-center justify-between">
        <button
          type="button"
          onClick={() => setTarget({ kind: "create-top", topIndex: -1, childIndex: -1 })}
          className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
        >
          + 创建导航项
        </button>
        <button
          type="button"
          onClick={() => void handleSave()}
          disabled={!loaded || saving}
          className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
        >
          {saving ? "保存中…" : "保存导航"}
        </button>
      </div>

      {error && (
        <p className="mb-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}
      {saved && (
        <p className="mb-4 rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
          保存成功，前台头部导航已生效（一级项悬停可展开二级下拉）
        </p>
      )}

      {/* 两级树列表 */}
      <div className="rounded-lg border border-line bg-elevated">
        {loaded && items.length === 0 && (
          <p className="px-6 py-12 text-center text-sm text-ink-3">
            当前使用默认导航（首页/话题），点击「创建导航项」开始自定义
          </p>
        )}
        {!loaded && (
          <div className="px-6 py-10">
            <div className="h-5 w-2/3 animate-pulse rounded bg-muted" aria-hidden />
          </div>
        )}
        {items.map((top, i) => {
          const children = top.children ?? [];
          const isCollapsed = collapsed.has(i);
          return (
            <div key={`${top.label}-${i}`} className="border-b border-line last:border-b-0">
              {/* 一级行 */}
              <div className="flex items-center gap-2 px-4 py-3">
                <button
                  type="button"
                  onClick={() => toggleCollapse(i)}
                  className="w-4 text-xs text-ink-3 hover:text-ink"
                  aria-label={isCollapsed ? "展开二级" : "折叠二级"}
                >
                  {children.length > 0 ? (isCollapsed ? "▸" : "▾") : "·"}
                </button>
                <span className="min-w-[72px] font-medium text-ink">{top.label}</span>
                <span className="min-w-0 flex-1 truncate text-sm text-ink-3">
                  {top.url || "（纯分组）"} · {children.length} 个二级
                </span>
                <div className="flex items-center gap-2 text-xs">
                  <button type="button" onClick={() => moveTop(i, -1)} disabled={i === 0} className="text-ink-3 hover:text-ink disabled:opacity-30">上移</button>
                  <button type="button" onClick={() => moveTop(i, 1)} disabled={i === items.length - 1} className="text-ink-3 hover:text-ink disabled:opacity-30">下移</button>
                  <button
                    type="button"
                    onClick={() => setTarget({ kind: "create-child", topIndex: i, childIndex: -1 })}
                    className="text-glow hover:underline"
                  >
                    + 二级
                  </button>
                  <button
                    type="button"
                    onClick={() => setTarget({ kind: "edit-top", topIndex: i, childIndex: -1 })}
                    className="text-ink-2 hover:text-glow"
                  >
                    编辑
                  </button>
                  <button type="button" onClick={() => deleteTop(i)} className="text-ink-2 hover:text-like">删除</button>
                </div>
              </div>

              {/* 二级列表（折叠隐藏） */}
              {!isCollapsed && children.length > 0 && (
                <div className="ml-10 border-l border-line pl-3">
                  {children.map((child, j) => (
                    <div key={`${child.label}-${j}`} className="flex items-center gap-2 py-2 text-sm">
                      <span className="min-w-[72px] text-ink">{child.label}</span>
                      <span className="min-w-0 flex-1 truncate text-ink-3">{child.url}</span>
                      <div className="flex items-center gap-2 text-xs">
                        <button type="button" onClick={() => moveChild(i, j, -1)} disabled={j === 0} className="text-ink-3 hover:text-ink disabled:opacity-30">上移</button>
                        <button type="button" onClick={() => moveChild(i, j, 1)} disabled={j === children.length - 1} className="text-ink-3 hover:text-ink disabled:opacity-30">下移</button>
                        <button
                          type="button"
                          onClick={() => setTarget({ kind: "edit-child", topIndex: i, childIndex: j })}
                          className="text-ink-2 hover:text-glow"
                        >
                          编辑
                        </button>
                        <button type="button" onClick={() => deleteChild(i, j)} className="text-ink-2 hover:text-like">删除</button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* 插件注册的前台导航项（只读：由插件 frontend/manifest.json siteNav 声明，随插件启停自动生效） */}
      {pluginNav.sources.length > 0 && (
        <div className="mt-4 rounded-lg border border-dashed border-line bg-muted/30 p-4">
          <p className="text-sm text-ink-2">插件注册的导航项（只读，展示在上方配置项之后）</p>
          <div className="mt-2 space-y-1.5">
            {pluginNav.sources.map((ext) =>
              (ext.site_nav ?? []).map((nav) => (
                <p key={`${ext.plugin_id}-${nav.path}`} className="text-xs text-ink-3">
                  · {nav.label} → {nav.path}（{ext.name}）
                </p>
              )),
            )}
          </div>
        </div>
      )}

      {/* 创建/编辑面板 */}
      <NavItemModal
        open={target !== null}
        title={
          target?.kind === "create-top"
            ? "创建一级导航"
            : target?.kind === "create-child"
              ? `添加二级导航（${items[target.topIndex]?.label ?? ""}）`
              : target?.kind === "edit-top"
                ? "编辑一级导航"
                : "编辑二级导航"
        }
        initial={editInitial}
        requireURL={target?.kind === "create-child" || target?.kind === "edit-child"}
        publishedPages={pages}
        onSubmit={handleModalSubmit}
        onClose={() => setTarget(null)}
      />
    </div>
  );
}
