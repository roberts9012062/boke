// src/app/admin/serp/page.tsx
// 后台 SERP 预览（设计稿 D/冷月/SEO·SERP预览 1400×1100）：
// 搜索结果样式预览（桌面 Google 风格）+ 刷新预览 + 标题字数/检查项 + 缺少 SEO 字段警告 + 去修复。
"use client";

import { useEffect, useState } from "react";

import { apiAdminPosts, apiSerpPreview, ApiError, type SerpPreview } from "@/lib/api";

// AdminSerp SERP 预览页。
export default function AdminSerp() {
  const [posts, setPosts] = useState<{ id: number; title: string }[]>([]);
  const [postId, setPostId] = useState<number>(0);
  const [preview, setPreview] = useState<SerpPreview | null>(null);
  const [error, setError] = useState<string>("");

  // 加载帖子列表（选择预览目标）
  useEffect(() => {
    apiAdminPosts({ page: 1 })
      .then((r) => setPosts(r.items.map((p) => ({ id: p.id, title: p.title || p.summary || `帖子 #${p.id}` }))))
      .catch(() => undefined);
  }, []);

  // 生成预览
  const loadPreview = async (id: number) => {
    if (!id) return;
    setError("");
    try {
      setPreview(await apiSerpPreview(id));
    } catch (err) {
      setPreview(null);
      setError(err instanceof ApiError ? err.message : "预览生成失败");
    }
  };

  // 切换帖子
  const selectPost = (id: number) => {
    setPostId(id);
    void loadPreview(id);
  };

  return (
    <div className="max-w-[720px]">
      <h1 className="font-display text-xl font-semibold text-ink">SERP 预览</h1>
      <p className="mt-0.5 text-xs text-ink-3">搜索结果样式预览 · 桌面 Google 风格</p>

      {/* 选择帖子 + 刷新预览 */}
      <div className="mt-4 flex flex-wrap items-center gap-3">
        <select
          value={postId}
          onChange={(e) => selectPost(Number(e.target.value))}
          className="h-10 w-72 rounded-full border border-line bg-elevated px-4 text-sm text-ink focus:border-accent focus:outline-none"
        >
          <option value={0}>选择要预览的帖子…</option>
          {posts.map((p) => (
            <option key={p.id} value={p.id}>
              #{p.id} · {(p.title || "").slice(0, 30)}
            </option>
          ))}
        </select>
        <button
          type="button"
          disabled={!postId}
          onClick={() => postId && void loadPreview(postId)}
          className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
        >
          刷新预览
        </button>
      </div>

      {error && (
        <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* Google 风格预览（设计稿：搜索结果样式） */}
      {preview && (
        <div className="mt-6 rounded-lg border border-line bg-elevated p-6">
          <p className="text-xs text-ink-3">预览说明：SERP 为根据当前配置生成的示意预览，实际展示以搜索引擎为准。</p>
          <div className="mt-4">
            <p className="text-[10px] text-ink-3">{preview.url || ""}</p>
            <span className="mt-1 inline-block rounded-full bg-muted px-2 py-0.5 text-[10px] text-ink-3">
              标题 {preview.title_len ?? 0} 字
            </span>
            <a
              href={preview.url || "#"}
              target="_blank"
              rel="noreferrer"
              className="mt-0.5 block text-lg text-blue-600 hover:underline"
            >
              {preview.title || ""}
            </a>
            <p className="mt-1 line-clamp-2 text-sm text-ink-2">{preview.description || ""}</p>
          </div>

          {/* 检查项（设计稿：标题 10-60 字 / 描述 50-160 字 / 唯一 URL / 建议补 OG 图） */}
          <div className="mt-4 flex flex-wrap gap-2 border-t border-line pt-4">
            {(preview.checks ?? []).map((check) => (
              <span key={check} className="rounded-full bg-muted px-3 py-1 text-xs text-ink-2">
                {check}
              </span>
            ))}
          </div>

          {/* 缺少 SEO 字段警告 + 去修复 */}
          {(preview.warnings?.length ?? 0) > 0 && (
            <div className="mt-4 rounded-md bg-like/10 px-4 py-3">
              <p className="text-xs text-like">{(preview.warnings ?? [])[0] || ""}</p>
              <a href="/admin/seo-health" className="mt-2 inline-block text-xs text-glow hover:underline">
                去修复 →
              </a>
            </div>
          )}
        </div>
      )}
      {!preview && !error && (
        <div className="mt-8 rounded-lg border border-dashed border-line py-14 text-center">
          <p className="text-sm text-ink-3">选择帖子后生成搜索预览</p>
        </div>
      )}
    </div>
  );
}
