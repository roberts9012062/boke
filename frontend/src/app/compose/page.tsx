// src/app/compose/page.tsx
// 发帖中心（设计稿 D/冷月/发帖中心 1400）：
// 写一帖 → Tab（文字/图片/音频/视频）→ 正文（把月光写成句子…）
// → 标签（#月色）→ 可见性（公开/私密）→ 0/2000 字数 → 草稿/发布。
"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { AudioUploader } from "@/components/compose/audio-uploader";
import { ImageUploader } from "@/components/compose/image-uploader";
import { VideoUploader } from "@/components/compose/video-uploader";
import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiCreatePost, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { MediaDTO, PostContentType } from "@/types/api";

// 正文上限（需求 3.4：0/2000）
const MAX_CONTENT = 2000;
// 标签上限
const MAX_TAGS = 5;

// 可见性选项（设计稿《可见性》弹层：公开/仅关注者/仅自己）
const VISIBILITY_OPTIONS = [
  { key: "public", label: "公开", desc: "所有人可见，可被推荐" },
  { key: "followers", label: "仅关注者", desc: "互相关注的人可见" },
  { key: "private", label: "仅自己", desc: "草稿箱式私密，不出现在主页" },
] as const;

// Tab 配置（文案与设计稿一致；文字/图片/音频/视频四类型 M2 全开放）
interface TypeTab {
  key: PostContentType; // Tab 键（text/image/audio/video）
  label: string; // 显示文案
}

const TYPE_TABS: readonly TypeTab[] = [
  { key: "text", label: "文字" },
  { key: "image", label: "图片" },
  { key: "audio", label: "音频" },
  { key: "video", label: "视频" },
];

// ComposePage 发帖中心（未登录跳登录页）。
export default function ComposePage() {
  const router = useRouter();
  const { user, loading } = useAuth();

  // 表单状态
  const [contentType, setContentType] = useState<PostContentType>("text");
  const [content, setContent] = useState<string>("");
  const [tagInput, setTagInput] = useState<string>("");
  const [tags, setTags] = useState<string[]>([]);
  const [visibility, setVisibility] = useState<"public" | "followers" | "private">("public");
  const [visibilityOpen, setVisibilityOpen] = useState<boolean>(false);
  const [images, setImages] = useState<MediaDTO[]>([]);
  const [audio, setAudio] = useState<MediaDTO | null>(null);
  const [video, setVideo] = useState<MediaDTO | null>(null); // M2：视频发帖
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<"draft" | "published" | null>(null);

  // 未登录：跳登录页
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-32 animate-pulse rounded bg-muted" aria-hidden />
      </div>
    );
  }
  if (!user) {
    router.replace("/login");
    return null;
  }

  // 字数统计（UTF-16 长度近似字符数）
  const charCount = Array.from(content).length;

  // 添加标签（# 前缀识别，回车/空格提交）
  const addTag = (raw: string) => {
    const name = raw.trim().replace(/^#/, "");
    if (!name || tags.includes(name)) {
      setTagInput("");
      return;
    }
    if (tags.length >= MAX_TAGS) {
      setError(`标签最多 ${MAX_TAGS} 个`);
      return;
    }
    setTags((prev) => [...prev, name]);
    setTagInput("");
  };

  // 提交（draft=存草稿 / published=发布）
  const submit = async (status: "draft" | "published") => {
    setError("");
    // 发布校验：正文非空（需求 3.4 发布必须内容）
    if (status === "published" && content.trim() === "") {
      setError("正文不能为空");
      return;
    }
    setSubmitting(status);
    try {
      const result = await apiCreatePost({
        content_type: contentType,
        content: content.trim(),
        tags,
        // 媒体：图片多选 / 音频单选 / 视频单选（M2）
        media_ids:
          contentType === "image"
            ? images.map((m) => m.id)
            : contentType === "video"
              ? video
                ? [video.id]
                : []
              : audio
                ? [audio.id]
                : [],
        visibility,
        status,
      });
      // 发布成功 → 发布成功页；草稿 → 草稿箱提示回首页
      if (status === "published") {
        router.push(`/publish-success/${result.id}`);
      } else {
        router.push("/drafts");
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败，请稍后再试");
    } finally {
      setSubmitting(null);
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[720px] flex-1 px-6 py-8">
        <h1 className="mb-6 font-display text-2xl font-semibold text-ink">写一帖</h1>

        {/* 类型 Tab（设计稿：文字/图片/音频/视频） */}
        <div className="flex gap-2 border-b border-line pb-4">
          {TYPE_TABS.map((tab) => (
            <button
              key={tab.key}
              type="button"
              onClick={() => setContentType(tab.key)}
              className={`whitespace-nowrap rounded-full px-5 py-1.5 text-sm transition-colors ${
                contentType === tab.key
                  ? "bg-accent-soft font-medium text-glow"
                  : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* 正文输入（设计稿占位：把月光写成句子…） */}
        <textarea
          value={content}
          onChange={(e) => {
            // 超限禁止输入（需求 3.4：0/2000）
            if (Array.from(e.target.value).length <= MAX_CONTENT) {
              setContent(e.target.value);
            }
          }}
          placeholder="把月光写成句子…"
          rows={6}
          className="mt-6 w-full resize-none rounded-lg border border-line bg-elevated p-4 text-sm leading-relaxed text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        {/* 字数统计（设计稿：0 / 2000） */}
        <p className="mt-1 text-right text-xs text-ink-3">
          {charCount} / {MAX_CONTENT}
        </p>

        {/* 图片上传区（仅图片 Tab 显示） */}
        {contentType === "image" && (
          <div className="mt-4">
            <ImageUploader value={images} onChange={setImages} />
          </div>
        )}

        {/* 音频上传区（仅音频 Tab 显示） */}
        {contentType === "audio" && (
          <div className="mt-4">
            <AudioUploader value={audio} onChange={setAudio} />
          </div>
        )}

        {/* 视频上传区（仅视频 Tab 显示，M2） */}
        {contentType === "video" && (
          <div className="mt-4">
            <VideoUploader value={video} onChange={setVideo} />
          </div>
        )}

        {/* 标签输入（设计稿占位：标签 #月色） */}
        <div className="mt-6">
          <div className="flex flex-wrap items-center gap-2">
            {tags.map((tag) => (
              <span
                key={tag}
                className="flex items-center gap-1 rounded-full bg-accent-soft px-3 py-1 text-xs text-glow"
              >
                #{tag}
                <button
                  type="button"
                  onClick={() => setTags((prev) => prev.filter((t) => t !== tag))}
                  className="text-ink-3 hover:text-like"
                  aria-label={`删除标签 ${tag}`}
                >
                  ✕
                </button>
              </span>
            ))}
            <input
              type="text"
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  addTag(tagInput);
                }
              }}
              onBlur={() => {
                if (tagInput.trim()) {
                  addTag(tagInput);
                }
              }}
              placeholder="标签 #月色"
              className="h-8 flex-1 rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </div>
          <p className="mt-1 text-xs text-ink-3">最多 {MAX_TAGS} 个标签，回车添加</p>
        </div>

        {/* 可见性（设计稿：公开 按钮 +「谁可以看」弹层） */}
        <div className="relative mt-6">
          <button
            type="button"
            onClick={() => setVisibilityOpen((v) => !v)}
            className="flex items-center gap-1.5 rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            {visibility === "public" ? "公开" : visibility === "followers" ? "仅关注者" : "仅自己"}
            <span className="text-xs text-ink-3" aria-hidden>
              ▾
            </span>
          </button>

          {/* 谁可以看 弹层（设计稿 D/冷月/可见性） */}
          {visibilityOpen && (
            <div className="absolute left-0 top-11 z-20 w-72 rounded-lg border border-line bg-elevated p-4 shadow-lg">
              <p className="font-display text-base font-semibold text-ink">谁可以看</p>
              <div className="mt-3 space-y-2">
                {VISIBILITY_OPTIONS.map((opt) => (
                  <button
                    key={opt.key}
                    type="button"
                    onClick={() => setVisibility(opt.key)}
                    className={`w-full rounded-md border px-3 py-2.5 text-left transition-colors ${
                      visibility === opt.key
                        ? "border-accent bg-accent-soft"
                        : "border-line hover:bg-muted"
                    }`}
                  >
                    <p className={`text-sm ${visibility === opt.key ? "text-glow" : "text-ink"}`}>
                      {opt.label}
                    </p>
                    <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
                  </button>
                ))}
              </div>
              <div className="mt-4 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setVisibilityOpen(false)}
                  className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
                >
                  取消
                </button>
                <button
                  type="button"
                  onClick={() => setVisibilityOpen(false)}
                  className="rounded-full bg-accent px-5 py-1.5 text-sm text-on-accent"
                >
                  完成
                </button>
              </div>
            </div>
          )}
        </div>

        {/* 错误提示 */}
        {error && (
          <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}

        {/* 操作按钮（设计稿：草稿 / 发布） */}
        <div className="mt-8 flex justify-end gap-3">
          <button
            type="button"
            disabled={submitting !== null}
            onClick={() => void submit("draft")}
            className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
          >
            {submitting === "draft" ? "保存中…" : "草稿"}
          </button>
          <button
            type="button"
            disabled={submitting !== null}
            onClick={() => void submit("published")}
            className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {submitting === "published" ? "发布中…" : "发布"}
          </button>
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
