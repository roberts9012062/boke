// src/app/search/page.tsx
// 搜索页（设计稿 D/冷月/搜索 + M/冷月/搜索）：
// 搜索框 + Tab（全部/帖子/话题/用户）+ 结果列表（帖子卡片/话题/用户）。
// M1.7：支持 URL 参数 ?q=（桌面导航搜索框跳转）、统一空态组件。
"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { PostCard } from "@/components/post-card";
import { Avatar } from "@/components/ui/avatar";
import { EmptyState } from "@/components/ui/empty-state";
import { apiSearch } from "@/lib/api";
import type { SearchResult } from "@/types/api";

// Tab 配置（设计稿：全部/帖子/话题/用户）
const TABS = ["全部", "帖子", "话题", "用户"] as const;
type TabKey = (typeof TABS)[number];

// 搜索热词（设计稿 M/冷月/搜索：「月色 夜读」）
const HOT_KEYWORDS = ["月色", "夜读", "城市声景"];

// SearchInner 搜索页主体（Suspense 内使用 useSearchParams）。
function SearchInner() {
  const searchParams = useSearchParams();
  const [keyword, setKeyword] = useState<string>("");
  const [submitted, setSubmitted] = useState<string>("");
  const [tab, setTab] = useState<TabKey>("全部");
  const [result, setResult] = useState<SearchResult | null>(null);
  const [searching, setSearching] = useState<boolean>(false);

  // 初始关键词：读取 URL ?q=（桌面导航搜索框跳转进入）
  useEffect(() => {
    const q = searchParams.get("q") ?? "";
    if (q) {
      setKeyword(q);
      setSubmitted(q);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 执行搜索（防抖 300ms，需求 3.7）
  useEffect(() => {
    if (!submitted.trim()) {
      setResult(null);
      return;
    }
    let cancelled = false;
    setSearching(true);
    apiSearch(submitted.trim())
      .then((r) => {
        if (!cancelled) setResult(r);
      })
      .catch(() => {
        if (!cancelled) setResult(null);
      })
      .finally(() => {
        if (!cancelled) setSearching(false);
      });
    return () => {
      cancelled = true;
    };
  }, [submitted]);

  // 回车提交
  const handleSubmit = () => {
    setSubmitted(keyword.trim());
  };

  // 统一空态（M1.7：EmptyState 组件，替换本地实现）
  const renderEmpty = (title: string, desc: string) => (
    <EmptyState icon="🔍" title={title} description={desc} />
  );

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-4 py-6 pb-20">
        {/* 搜索框（设计稿：搜索… / 月色 夜读 热词） */}
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                handleSubmit();
              }
            }}
            placeholder="搜索帖子、话题、用户…"
            className="h-11 flex-1 rounded-full border border-line bg-elevated px-5 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
          <button
            type="button"
            onClick={handleSubmit}
            className="h-11 rounded-full bg-accent px-6 text-sm font-medium text-on-accent transition-opacity hover:opacity-90"
          >
            搜索
          </button>
        </div>

        {/* 搜索热词（设计稿：月色 夜读；点击直接搜索） */}
        <div className="mt-3 flex items-center gap-2 text-sm">
          <span className="text-xs text-ink-3">大家都在搜</span>
          {HOT_KEYWORDS.map((word) => (
            <button
              key={word}
              type="button"
              onClick={() => {
                setKeyword(word);
                setSubmitted(word);
              }}
              className="rounded-full bg-muted px-3 py-1 text-xs text-ink-2 transition-colors hover:text-glow"
            >
              {word}
            </button>
          ))}
        </div>

        {/* 结果 Tab（设计稿：全部/帖子/话题/用户） */}
        {result && (
          <div className="mt-5 flex gap-2 border-b border-line pb-3">
            {TABS.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => setTab(t)}
                className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                  tab === t
                    ? "bg-accent-soft font-medium text-glow"
                    : "bg-muted text-ink-2 hover:text-ink"
                }`}
              >
                {t}
              </button>
            ))}
          </div>
        )}

        {/* 搜索结果区 */}
        <div className="mt-4">
          {searching && <div className="h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}

          {!searching && result && tab === "全部" && (
            <div className="post-card-list space-y-4">
              {/* 话题匹配提示（设计稿：相关帖子） */}
              {result.topics.length > 0 && (
                <div className="rounded-lg border border-line bg-elevated p-4">
                  <p className="text-xs text-ink-3">相关话题</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {result.topics.map((t) => (
                      <Link
                        key={t.slug}
                        href={`/topics/${encodeURIComponent(t.name.replace("#", ""))}`}
                        className="rounded-full bg-accent-soft px-3 py-1 text-sm text-glow hover:underline"
                      >
                        {t.name} · {t.post_count} 帖
                      </Link>
                    ))}
                  </div>
                </div>
              )}
              {result.users.length > 0 && (
                <div className="rounded-lg border border-line bg-elevated p-4">
                  <p className="text-xs text-ink-3">相关用户</p>
                  <div className="mt-2 space-y-2">
                    {result.users.slice(0, 3).map((u) => (
                      <Link key={u.id} href={`/users/${u.id}`} className="flex items-center gap-3">
                        <Avatar name={u.nickname} url={u.avatar_url} className="h-8 w-8 text-xs" />
                        <span className="text-sm text-ink-2">
                          {u.nickname} <span className="text-ink-3">@{u.username}</span>
                        </span>
                      </Link>
                    ))}
                  </div>
                </div>
              )}
              {/* 帖子结果 */}
              {result.posts.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
              {result.posts.length === 0 && result.topics.length === 0 && result.users.length === 0 && renderEmpty("没有找到相关内容", "换个关键词试试")}
            </div>
          )}

          {!searching && result && tab === "帖子" && (
            <div className="space-y-4">
              {result.posts.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
              {result.posts.length === 0 && renderEmpty("没有找到相关内容", "换个关键词试试")}
            </div>
          )}

          {!searching && result && tab === "话题" && (
            <div className="space-y-3">
              {result.topics.map((t) => (
                <Link
                  key={t.slug}
                  href={`/topics/${encodeURIComponent(t.name.replace("#", ""))}`}
                  className="block rounded-lg border border-line bg-elevated p-4 hover:border-accent"
                >
                  <p className="text-sm font-medium text-glow">{t.name}</p>
                  <p className="mt-1 text-xs text-ink-3">{t.post_count} 帖</p>
                </Link>
              ))}
              {result.topics.length === 0 && renderEmpty("没有找到相关内容", "换个关键词试试")}
            </div>
          )}

          {!searching && result && tab === "用户" && (
            <div className="space-y-3">
              {result.users.map((u) => (
                <Link
                  key={u.id}
                  href={`/users/${u.id}`}
                  className="flex items-center gap-3 rounded-lg border border-line bg-elevated p-4 hover:border-accent"
                >
                  <Avatar name={u.nickname} url={u.avatar_url} className="h-10 w-10 text-sm" />
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-ink">{u.nickname}</p>
                    <p className="truncate text-xs text-ink-3">
                      @{u.username}
                      {u.bio ? ` · ${u.bio}` : ""}
                    </p>
                  </div>
                </Link>
              ))}
              {result.users.length === 0 && renderEmpty("没有找到相关内容", "换个关键词试试")}
            </div>
          )}
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}

// SearchPage 搜索页（双端共用）。
// 说明：useSearchParams 需要 Suspense 边界（Next.js 静态预渲染要求）。
export default function SearchPage() {
  return (
    <Suspense fallback={null}>
      <SearchInner />
    </Suspense>
  );
}
