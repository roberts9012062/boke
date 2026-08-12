// src/app/compose/page.tsx
// 发帖中心（设计稿 D/冷月/发帖中心 1400）：
// 写一帖 → Tab（文字/图片/音频/视频）→ 正文（把月光写成句子…）
// → 标签（#月色）→ 可见性（公开/私密）→ 0/2000 字数 → 草稿/发布。
// 走查纠偏：支持 ?draft=ID（草稿继续编辑）与 ?edit=ID（编辑已发布帖子，设计稿《编辑帖子》）。
"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { AudioUploader } from "@/components/compose/audio-uploader";
import { collectMediaIds, MAX_CONTENT, MAX_TAGS, TYPE_TABS, VISIBILITY_OPTIONS } from "@/components/compose/config";
import { ImageUploader } from "@/components/compose/image-uploader";
import { VideoUploader } from "@/components/compose/video-uploader";
import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import PluginSlot from "@/components/plugin-slot";
import { apiCreatePost, apiPostDetail, apiUpdatePost, ApiError } from "@/lib/api";
import type { PostSeoInput } from "@/types/api";
import { useAuth } from "@/lib/auth";
import type { MediaDTO, PostContentType } from "@/types/api";

// ComposePage 发帖中心（未登录跳登录页；?draft= 草稿继续编辑 / ?edit= 编辑已发布）。
// 说明：useSearchParams 需 Suspense 包裹（Next.js CSR bailout 要求，生产构建必检）。
export default function ComposePage() {
  return (
    <Suspense fallback={null}>
      <ComposeContent />
    </Suspense>
  );
}

// ComposeContent 发帖中心主体（含 useSearchParams 读取编辑参数）。
function ComposeContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user, loading } = useAuth();

  // 编辑目标（?draft=ID 草稿继续编辑 / ?edit=ID 编辑已发布；走查纠偏补）
  const draftParam = searchParams.get("draft");
  const editParam = searchParams.get("edit");
  const editId = draftParam ? Number(draftParam) : editParam ? Number(editParam) : 0;
  const editing = editId > 0;

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
  const [submitting, setSubmitting] = useState<"draft" | "published" | "edit" | null>(null);
  // SEO 输入（M4.1 插件通道：SEO 面板经 props.onChange 回写；随发帖/编辑提交）
  const [seo, setSeo] = useState<PostSeoInput | null>(null);

  // 编辑模式：加载帖子详情填充表单（草稿继续编辑与编辑已发布共用）
  useEffect(() => {
    if (!editing) {
      return;
    }
    apiPostDetail(editId)
      .then((d) => {
        setContentType(d.content_type);
        setContent(d.content);
        setTags(d.tags.map((t) => t.name.replace(/^#/, "")));
        setVisibility(d.visibility === "private" ? "private" : d.visibility === "followers" ? "followers" : "public");
        // 媒体按类型回填（图片多张 / 音频 / 视频）
        const medias = d.media ?? [];
        if (d.content_type === "image") {
          setImages(medias);
        } else if (d.content_type === "audio") {
          setAudio(medias[0] ?? null);
        } else if (d.content_type === "video") {
          setVideo(medias[0] ?? null);
        }
        // SEO 回填（编辑已发布帖子：SEO 面板初始值）
        if (d.seo) {
          setSeo({
            seo_title: d.seo.title,
            seo_description: d.seo.description,
            url_alias: d.seo.url_alias,
            robots: d.seo.robots,
          });
        }
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败，无法编辑"));
  }, [editing, editId]);

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

  // 提交（new：draft=存草稿 / published=发布；edit：保存修改）
  const submit = async (status: "draft" | "published" | "edit") => {
    setError("");
    // 发布/保存校验：正文非空（需求 3.4 发布必须内容）
    if (status !== "draft" && content.trim() === "") {
      setError("正文不能为空");
      return;
    }
    setSubmitting(status);
    try {
      // SEO 输入（M4.1 插件通道：SEO 面板提交；无内容时省略）
      const seoPayload = seo
        ? {
            seo_title: seo.seo_title,
            seo_description: seo.seo_description,
            url_alias: seo.url_alias,
            robots: seo.robots,
          }
        : undefined;
      if (editing) {
        // 编辑模式：更新帖子（类型不可变，后端 UpdatePostReq 不支持 content_type）
        await apiUpdatePost(editId, {
          content: content.trim(),
          tags,
          media_ids: collectMediaIds(contentType, images, audio, video),
          visibility,
          ...(seoPayload ? { seo: seoPayload } : {}),
        });
        router.push(`/posts/${editId}`);
      } else {
        // 新建模式：status 仅 draft/published（edit 分支已提前返回）
        const createStatus = status as "draft" | "published";
        const result = await apiCreatePost({
          content_type: contentType,
          content: content.trim(),
          tags,
          media_ids: collectMediaIds(contentType, images, audio, video),
          visibility,
          status: createStatus,
          ...(seoPayload ? { seo: seoPayload } : {}),
        });
        // 发布成功 → 发布成功页；草稿 → 草稿箱提示回首页
        if (createStatus === "published") {
          router.push(`/publish-success/${result.id}`);
        } else {
          router.push("/drafts");
        }
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
        {/* 标题（编辑模式：设计稿《编辑帖子》「编辑帖子」；走查纠偏补） */}
        <h1 className="mb-6 font-display text-2xl font-semibold text-ink">{editing ? "编辑帖子" : "写一帖"}</h1>

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

        {/* 插件扩展点：compose.seo（M4.1 发帖 SEO 面板——由 seo-optimizer 插件渲染；
            无插件时槽位为空，发帖界面保持原样；卸载后自动还原） */}
        <PluginSlot slot="compose.seo" props={{ initial: seo, onChange: setSeo }} />

        {/* 错误提示 */}
        {error && (
          <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}

        {/* 操作按钮（设计稿：草稿 / 发布；编辑模式：取消 / 保存修改，《编辑帖子》画板） */}
        <div className="mt-8 flex justify-end gap-3">
          {editing ? (
            <>
              <button
                type="button"
                disabled={submitting !== null}
                onClick={() => router.back()}
                className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
              >
                取消
              </button>
              <button
                type="button"
                disabled={submitting !== null}
                onClick={() => void submit("edit")}
                className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
              >
                {submitting === "edit" ? "保存中…" : "保存修改"}
              </button>
            </>
          ) : (
            <>
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
            </>
          )}
        </div>
      </main>
      <MobileTabbar />
    </div>
  );
}
