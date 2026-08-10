// src/app/admin/sensitive-words/page.tsx
// 后台敏感词管理（设计稿《敏感词》画板）：
// 拦截、替换与审核规则 → 搜索敏感词… → 词库列表（词/级别/添加时间/删除）。
// 级别：forbidden=直接拦截（发帖/评论拒绝）/ review=进入审核（预留）。
"use client";

import { useEffect, useState } from "react";

import {
  apiAdminAddSensitiveWord,
  apiAdminDeleteSensitiveWord,
  apiAdminSensitiveStats,
  apiAdminSensitiveWords,
  ApiError,
  type SensitiveWordDTO,
} from "@/lib/api";

// 级别文案（设计稿：拦截/审核）
const LEVEL_LABEL: Record<string, string> = {
  forbidden: "拦截",
  review: "审核",
};

// SensitiveWordsPage 敏感词管理页。
export default function SensitiveWordsPage() {
  const [items, setItems] = useState<SensitiveWordDTO[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>("");
  const [q, setQ] = useState<string>("");
  const [word, setWord] = useState<string>("");
  const [level, setLevel] = useState<string>("forbidden");
  const [stats, setStats] = useState<{ total: number; forbidden: number; review: number } | null>(null);
  const [error, setError] = useState<string>("");
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载统计（设计稿统计条：全部/拦截/审核）
  useEffect(() => {
    apiAdminSensitiveStats()
      .then(setStats)
      .catch(() => setStats(null));
  }, []);

  // 加载词库
  useEffect(() => {
    let cancelled = false;
    setLoaded(false);
    apiAdminSensitiveWords({ q, page: 1 })
      .then((r) => {
        if (cancelled) return;
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => {
        if (!cancelled) setItems([]);
      })
      .finally(() => {
        if (!cancelled) setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [q]);

  // 添加敏感词
  const handleAdd = async () => {
    setError("");
    const trimmed = word.trim();
    if (!trimmed) {
      setError("请输入敏感词");
      return;
    }
    try {
      await apiAdminAddSensitiveWord(trimmed, level);
      setWord("");
      setQ(""); // 刷新列表
      setLoaded(false);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "添加失败");
    }
  };

  // 删除敏感词
  const handleDelete = async (item: SensitiveWordDTO) => {
    try {
      await apiAdminDeleteSensitiveWord(item.word);
      setItems((prev) => prev.filter((x) => x.id !== item.id));
      setTotal((t) => Math.max(t - 1, 0));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
    }
  };

  return (
    <div>
      {/* 标题（设计稿：敏感词管理 / 拦截、替换与审核规则） */}
      <div>
        <h1 className="font-display text-xl font-semibold text-ink">敏感词管理</h1>
        <p className="mt-0.5 text-xs text-ink-3">拦截、替换与审核规则</p>
      </div>

      {/* 统计条（设计稿：全部标签/热门/本周新建/未使用；MVP 展示词库级别汇总） */}
      <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          { label: "全部", value: stats?.total ?? 0 },
          { label: "拦截", value: stats?.forbidden ?? 0 },
          { label: "审核", value: stats?.review ?? 0 },
          { label: "当前页", value: total },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">{s.label}</p>
            <p className="mt-1 font-display text-2xl font-semibold text-ink">{s.value}</p>
          </div>
        ))}
      </div>

      {/* 添加区（词 + 级别 + 添加） */}
      <div className="mt-4 flex flex-wrap items-center gap-2 rounded-lg border border-line bg-elevated p-4">
        <input
          type="text"
          value={word}
          onChange={(e) => setWord(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              void handleAdd();
            }
          }}
          placeholder="输入敏感词…"
          className="h-9 w-48 rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        <select
          value={level}
          onChange={(e) => setLevel(e.target.value)}
          className="h-9 rounded-full border border-line bg-muted px-3 text-sm text-ink-2 focus:border-accent focus:outline-none"
          aria-label="敏感词级别"
        >
          <option value="forbidden">拦截（直接拒绝）</option>
          <option value="review">审核（放行标记）</option>
        </select>
        <button
          type="button"
          onClick={() => void handleAdd()}
          className="h-9 rounded-full bg-accent px-5 text-sm font-medium text-on-accent hover:opacity-90"
        >
          添加
        </button>
        {error && <span className="text-xs text-like">{error}</span>}
      </div>

      {/* 搜索（设计稿：搜索敏感词…） */}
      <input
        type="text"
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            setQ(keyword.trim());
          }
        }}
        placeholder="搜索敏感词…"
        className="mt-4 h-9 w-full max-w-xs rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
      />

      {/* 词库列表 */}
      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && items.length === 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无敏感词</p>
        </div>
      )}
      {loaded && items.length > 0 && (
        <div className="mt-4 overflow-hidden rounded-lg border border-line bg-elevated">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">敏感词</th>
                <th className="px-4 py-3 font-normal">级别</th>
                <th className="px-4 py-3 font-normal">添加时间</th>
                <th className="px-4 py-3 font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.map((item) => (
                <tr key={item.id} className="hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium text-ink">{item.word}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${
                        item.level === "forbidden" ? "bg-like/15 text-like" : "bg-accent-soft text-glow"
                      }`}
                    >
                      {LEVEL_LABEL[item.level] ?? item.level}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink-3">
                    {new Date(item.created_at).toLocaleString("zh-CN")}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      type="button"
                      onClick={() => void handleDelete(item)}
                      className="rounded-full border border-line px-3 py-1 text-xs text-ink-3 transition-colors hover:text-like"
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="border-t border-line px-4 py-2 text-xs text-ink-3">共 {total} 个敏感词</p>
        </div>
      )}
    </div>
  );
}
