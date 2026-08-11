// src/app/admin/tags/page.tsx
// 后台标签分类（设计稿 D/冷月/后台标签 1400×1000）：
// 统计条（全部标签/热门/本周新建/未使用）+ 搜索
// + 标签表格（标签/文章/热度/更新/操作：编辑·合并·删除）+ 重命名弹层 + 合并弹层（选目标）。
"use client";

import { useEffect, useState } from "react";

import {
  apiAdminDeleteTag,
  apiAdminMergeTag,
  apiAdminRenameTag,
  apiAdminTags,
  apiAdminTagStats,
  ApiError,
  type AdminTagItem,
} from "@/lib/api";
import { formatDateTime, timeAgo } from "@/lib/utils";

// 热度档（设计稿：高/中/闲置；口径：帖数 ≥50 高 / ≥10 中 / 0 闲置 / 其余低）。
function hotLevel(count: number): string {
  if (count >= 50) return "高";
  if (count >= 10) return "中";
  if (count === 0) return "闲置";
  return "低";
}

// AdminTags 标签分类管理。
export default function AdminTags() {
  const [items, setItems] = useState<AdminTagItem[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>("");
  const [stats, setStats] = useState<{ total: number; hot: number; week_new: number; unused: number }>({
    total: 0,
    hot: 0,
    week_new: 0,
    unused: 0,
  });
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");

  // 重命名弹层
  const [renameTarget, setRenameTarget] = useState<AdminTagItem | null>(null);
  const [renameName, setRenameName] = useState<string>("");
  const [renameCategory, setRenameCategory] = useState<string>("");
  // 合并弹层（选择目标标签）
  const [mergeTarget, setMergeTarget] = useState<AdminTagItem | null>(null);
  const [mergeDst, setMergeDst] = useState<string>("");

  // 加载统计条（设计稿：全部/热门/本周新建/未使用）
  useEffect(() => {
    apiAdminTagStats().then(setStats).catch(() => undefined);
  }, []);

  // 加载列表
  useEffect(() => {
    setLoading(true);
    apiAdminTags({ q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => setItems([]))
      .finally(() => setLoading(false));
  }, [keyword]);

  // 刷新统计与列表（操作后）
  const refresh = () => {
    apiAdminTagStats().then(setStats).catch(() => undefined);
    apiAdminTags({ q: keyword || undefined })
      .then((r) => {
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => undefined);
  };

  // 确认重命名
  const confirmRename = async () => {
    if (!renameTarget) {
      return;
    }
    setError("");
    const name = renameName.trim();
    if (!name) {
      setError("请输入新标签名");
      return;
    }
    try {
      await apiAdminRenameTag(renameTarget.id, name, name, renameCategory.trim());
      setRenameTarget(null);
      refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "重命名失败");
    }
  };

  // 确认合并（目标从列表选择）
  const confirmMerge = async () => {
    if (!mergeTarget) {
      return;
    }
    setError("");
    const dst = items.find((t) => t.id === Number(mergeDst));
    if (!dst) {
      setError("请选择目标标签");
      return;
    }
    if (dst.id === mergeTarget.id) {
      setError("不能合并到自身");
      return;
    }
    try {
      await apiAdminMergeTag(mergeTarget.id, dst.id);
      setMergeTarget(null);
      setMergeDst("");
      refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "合并失败");
    }
  };

  // 删除标签（二次确认）
  const handleDelete = async (tag: AdminTagItem) => {
    if (!window.confirm(`确定删除标签「#${tag.name}」？将解除 ${tag.post_count} 篇帖子的关联`)) {
      return;
    }
    setError("");
    try {
      await apiAdminDeleteTag(tag.id);
      refresh();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
    }
  };

  return (
    <div>
      <h1 className="font-display text-xl font-semibold text-ink">标签与分类</h1>
      <p className="mt-0.5 text-xs text-ink-3">管理话题标签与内容分类</p>

      {/* 统计条（设计稿：全部标签/热门/本周新建/未使用） */}
      <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {[
          { key: "全部标签", value: stats.total },
          { key: "热门", value: stats.hot },
          { key: "本周新建", value: stats.week_new },
          { key: "未使用", value: stats.unused },
        ].map((s) => (
          <div key={s.key} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-2xl font-semibold text-ink">{s.value}</p>
            <p className="mt-1 text-xs text-ink-3">{s.key}</p>
          </div>
        ))}
      </div>

      {/* 搜索 */}
      <div className="mt-4 flex items-center gap-3">
        <input
          type="text"
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          placeholder="搜索标签…"
          className="h-9 w-56 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <span className="text-xs text-ink-3">共 {total} 个标签</span>
      </div>

      {error && (
        <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* 标签表格（设计稿：标签/文章/热度/更新/操作） */}
      <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
        <table className="w-full min-w-[680px] text-left text-sm">
          <thead>
            <tr className="border-b border-line text-xs text-ink-3">
              <th className="px-4 py-3 font-normal">标签</th>
              <th className="px-4 py-3 font-normal">分类</th>
              <th className="px-4 py-3 font-normal">文章</th>
              <th className="px-4 py-3 font-normal">热度</th>
              <th className="px-4 py-3 font-normal">更新</th>
              <th className="px-4 py-3 font-normal">操作</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((tag) => (
              <tr key={tag.id} className="hover:bg-muted/40">
                <td className="px-4 py-3">
                  <p className="font-medium text-glow">{tag.name}</p>
                  <p className="text-xs text-ink-3">
                    #{tag.slug}
                    {tag.description ? ` · ${tag.description}` : ""}
                  </p>
                </td>
                {/* 分类（设计稿：情绪/栏目/体裁/临时） */}
                <td className="px-4 py-3 text-ink-2">{tag.category || "—"}</td>
                <td className="px-4 py-3 text-ink-2">{tag.post_count}</td>
                <td className="px-4 py-3">
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      hotLevel(tag.post_count) === "高"
                        ? "bg-like/15 text-like"
                        : hotLevel(tag.post_count) === "中"
                          ? "bg-accent-soft text-glow"
                          : "bg-muted text-ink-3"
                    }`}
                  >
                    {hotLevel(tag.post_count)}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-ink-3">
                  {timeAgo(tag.created_at)}
                  <span className="ml-1 hidden lg:inline">{formatDateTime(tag.created_at)}</span>
                </td>
                <td className="px-4 py-3">
                  <div className="flex gap-2 text-xs">
                    <button
                      type="button"
                      onClick={() => {
                        setRenameTarget(tag);
                        setRenameName(tag.name);
                        setRenameCategory(tag.category);
                        setError("");
                      }}
                      className="text-glow hover:underline"
                    >
                      编辑
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setMergeTarget(tag);
                        setMergeDst("");
                        setError("");
                      }}
                      className="text-glow hover:underline"
                    >
                      合并
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleDelete(tag)}
                      className="text-like hover:underline"
                    >
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!loading && items.length === 0 && (
          <p className="py-12 text-center text-sm text-ink-3">没有匹配的标签</p>
        )}
      </div>

      {/* 重命名弹层 */}
      {renameTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="编辑标签"
          onClick={() => setRenameTarget(null)}
        >
          <div
            className="w-full max-w-[380px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display text-lg font-semibold text-ink">编辑标签</h2>
            <label htmlFor="rename-name" className="mt-4 block text-sm text-ink-2">
              标签名
            </label>
            <input
              id="rename-name"
              type="text"
              value={renameName}
              onChange={(e) => setRenameName(e.target.value)}
              maxLength={20}
              className="mt-1.5 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
            />
            <label htmlFor="rename-category" className="mt-4 block text-sm text-ink-2">
              分类
            </label>
            <input
              id="rename-category"
              type="text"
              value={renameCategory}
              onChange={(e) => setRenameCategory(e.target.value)}
              maxLength={50}
              placeholder="如：情绪 / 栏目 / 体裁 / 临时"
              className="mt-1.5 h-10 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
            <p className="mt-2 text-xs text-ink-3">重命名后话题 URL 同步更新（帖子正文 # 文本不变）</p>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setRenameTarget(null)}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void confirmRename()}
                className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent hover:opacity-90"
              >
                保存
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 合并弹层（选择目标标签） */}
      {mergeTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="合并标签"
          onClick={() => setMergeTarget(null)}
        >
          <div
            className="w-full max-w-[380px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="font-display text-lg font-semibold text-ink">
              合并「#{mergeTarget.name}」
            </h2>
            <p className="mt-1 text-xs text-ink-3">
              {mergeTarget.post_count} 篇帖子的关联将转移到目标标签，本标签删除
            </p>
            <label htmlFor="merge-dst" className="mt-4 block text-sm text-ink-2">
              目标标签
            </label>
            <select
              id="merge-dst"
              value={mergeDst}
              onChange={(e) => setMergeDst(e.target.value)}
              className="mt-1.5 h-10 w-full rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
            >
              <option value="">选择目标标签…</option>
              {items
                .filter((t) => t.id !== mergeTarget.id)
                .map((t) => (
                  <option key={t.id} value={t.id}>
                    #{t.name}（{t.post_count} 篇）
                  </option>
                ))}
            </select>
            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setMergeTarget(null)}
                className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink"
              >
                取消
              </button>
              <button
                type="button"
                onClick={() => void confirmMerge()}
                className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent hover:opacity-90"
              >
                确认合并
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
