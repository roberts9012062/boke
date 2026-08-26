// src/app/compose/page.tsx
// 发帖中心（设计稿 D/冷月/发帖中心 1400）：
// 写一帖 → 形态切换（写说说 / 写文章）→ Tab（文字/图片/音频/视频）→ 正文
// → 标签（#月色）→ 可见性（公开/私密）→ 字数 → 草稿/发布。
// 文章形态：标题必填 + 长正文（≤20000 字）+ 图集（图片走 media_ids）。
// 走查纠偏：支持 ?draft=ID（草稿继续编辑）与 ?edit=ID（编辑已发布帖子）。
"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { AudioUploader } from "@/components/compose/audio-uploader";
import {
  collectMediaIds,
  MAX_ARTICLE_CONTENT,
  MAX_CONTENT,
  TYPE_TABS,
} from "@/components/compose/config";
import { GalleryStylePicker } from "@/components/compose/gallery-style-picker";
import { ImageUploader } from "@/components/compose/image-uploader";
import { RichTextEditor } from "@/components/compose/rich-text-editor";
import { TagInput } from "@/components/compose/tag-input";
import { VideoUploader } from "@/components/compose/video-uploader";
import { VisibilitySelect } from "@/components/compose/visibility-select";
import type { GalleryStyle } from "@/components/image-gallery";
import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import PluginSlot from "@/components/plugin-slot";
import { apiCreatePost, apiPostDetail, apiUpdatePost, ApiError } from "@/lib/api";
import type { PostKind, PostSeoInput } from "@/types/api";
import { htmlToText } from "@/lib/rich-text";
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
  const [postKind, setPostKind] = useState<PostKind>("moment"); // 帖子形态：说说/文章（创建后不可变）
  const [contentType, setContentType] = useState<PostContentType>("text");
  const [title, setTitle] = useState<string>(""); // 文章标题（必填）
  const [content, setContent] = useState<string>("");
  const [tags, setTags] = useState<string[]>([]);
  const [visibility, setVisibility] = useState<"public" | "followers" | "private">("public");
  const [images, setImages] = useState<MediaDTO[]>([]);
  const [galleryStyle, setGalleryStyle] = useState<GalleryStyle>(""); // 图片展示风格
  const [audio, setAudio] = useState<MediaDTO | null>(null);
  const [video, setVideo] = useState<MediaDTO | null>(null); // M2：视频发帖
  const [error, setError] = useState<string>("");
  const [submitting, setSubmitting] = useState<"draft" | "published" | "edit" | null>(null);
  // SEO 输入（M4.1 插件通道：SEO 面板经 props.onChange 回写；随发帖/编辑提交）
  const [seo, setSeo] = useState<PostSeoInput | null>(null);
  // 编辑模式回填完成标记：SEO 面板须在历史值加载完成后挂载（PluginSlot props 为挂载时快照，
  // 提前挂载会拿到 initial:null 导致已发布帖子的 SEO 不回填）
  const [editLoaded, setEditLoaded] = useState<boolean>(!editing);
  // 是否有插件订阅 compose.seo（有插件时桌面切换为左内容 + 右插件两栏布局）
  const [hasSeoPlugin, setHasSeoPlugin] = useState<boolean>(false);

  // 编辑模式：加载帖子详情填充表单（草稿继续编辑与编辑已发布共用）
  useEffect(() => {
    if (!editing) {
      return;
    }
    apiPostDetail(editId)
      .then((d) => {
        setPostKind(d.post_kind === "article" ? "article" : "moment");
        setContentType(d.content_type);
        setTitle(d.title ?? "");
        setContent(d.content);
        setTags(d.tags.map((t) => t.name.replace(/^#/, "")));
        setVisibility(d.visibility === "private" ? "private" : d.visibility === "followers" ? "followers" : "public");
        setGalleryStyle((d.gallery_style as GalleryStyle) ?? "");
        // 媒体按类型回填（图片多张 / 音频 / 视频；文章形态图片走图集）
        const medias = d.media ?? [];
        if (d.post_kind === "article" || d.content_type === "image") {
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
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败，无法编辑"))
      .finally(() => setEditLoaded(true));
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

  // 当前形态的正文上限与占位文案
  const isArticle = postKind === "article";
  const maxLength = isArticle ? MAX_ARTICLE_CONTENT : MAX_CONTENT;

  // 提交（new：draft=存草稿 / published=发布；edit：保存修改）
  const submit = async (status: "draft" | "published" | "edit") => {
    setError("");
    // 文章：标题必填（草稿同样校验，与后端一致）
    if (isArticle && title.trim() === "") {
      setError("文章标题不能为空");
      return;
    }
    // 发布/保存校验：正文非空（需求 3.4 发布必须内容；HTML 按纯文本判空）
    if (status !== "draft" && htmlToText(content).trim() === "") {
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
      // 媒体组装：文章携带图集；说说按媒体形态类型取值
      const mediaIds = collectMediaIds(postKind, contentType, images, audio, video);
      if (editing) {
        // 编辑模式：更新帖子（形态/类型不可变，后端 UpdatePostReq 不支持）
        await apiUpdatePost(editId, {
          title: isArticle ? title.trim() : undefined,
          content,
          content_format: "html",
          tags,
          media_ids: mediaIds,
          gallery_style: galleryStyle,
          visibility,
          ...(seoPayload ? { seo: seoPayload } : {}),
        });
        router.push(`/posts/${editId}`);
      } else {
        // 新建模式：status 仅 draft/published（edit 分支已提前返回）
        const createStatus = status as "draft" | "published";
        const result = await apiCreatePost({
          content_type: isArticle ? "text" : contentType,
          post_kind: postKind,
          title: isArticle ? title.trim() : undefined,
          content,
          content_format: "html",
          tags,
          media_ids: mediaIds,
          gallery_style: galleryStyle,
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
      <main className={`mx-auto w-full flex-1 px-6 py-8 ${hasSeoPlugin ? "lg:max-w-[1240px]" : "max-w-[720px]"}`}>
        {/* 标题（编辑模式：设计稿《编辑帖子》；文章形态文案区分） */}
        <h1 className="mb-6 font-display text-2xl font-semibold text-ink">
          {editing ? "编辑" : "写"}
          {isArticle ? "文章" : "一帖"}
        </h1>

        {/* 两栏布局：左=发布内容，右=插件面板（仅桌面；移动端回退单栏自然堆叠） */}
        <div className="lg:flex lg:items-start lg:gap-8">
          {/* 左栏：发布内容 */}
          <div className="lg:min-w-0 lg:flex-1">

        {/* 形态切换（写说说 / 写文章；编辑模式形态不可变故隐藏切换） */}
        {!editing && (
          <div className="mb-4 flex gap-2">
            <button
              type="button"
              onClick={() => setPostKind("moment")}
              className={`rounded-full px-5 py-1.5 text-sm transition-colors ${
                postKind === "moment" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              写说说
            </button>
            <button
              type="button"
              onClick={() => setPostKind("article")}
              className={`rounded-full px-5 py-1.5 text-sm transition-colors ${
                postKind === "article" ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
              }`}
            >
              写文章
            </button>
          </div>
        )}

        {/* 类型 Tab（设计稿：文字/图片/音频/视频；文章形态固定文字不显示） */}
        {!isArticle && (
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
        )}

        {/* 文章标题输入（文章形态必填） */}
        {isArticle && (
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={100}
            placeholder="文章标题（必填，100 字以内）"
            className="w-full rounded-lg border border-line bg-elevated px-4 py-3 font-display text-lg font-semibold text-ink placeholder:font-normal placeholder:text-base placeholder:text-ink-3 focus:border-accent focus:outline-none"
          />
        )}

        {/* 正文输入（M5 富文本：WYSIWYG 编辑器，图片上传/视频内嵌/外链） */}
        <div className="mt-6">
          <RichTextEditor
            value={content}
            onChange={(v) => setContent(v)}
            placeholder={isArticle ? "把长文写成篇章…" : "把月光写成句子…"}
            maxLength={maxLength}
          />
        </div>

        {/* 图片上传区（说说图片 Tab / 文章形态图集，均显示） */}
        {(isArticle || contentType === "image") && (
          <div className="mt-4">
            <ImageUploader value={images} onChange={setImages} />
            {/* 展示效果选择器（上传图片下方；点击风格实时预览） */}
            <GalleryStylePicker value={galleryStyle} onChange={setGalleryStyle} images={images} />
          </div>
        )}

        {/* 音频上传区（仅说说音频 Tab 显示） */}
        {!isArticle && contentType === "audio" && (
          <div className="mt-4">
            <AudioUploader value={audio} onChange={setAudio} />
          </div>
        )}

        {/* 视频上传区（仅说说视频 Tab 显示，M2） */}
        {!isArticle && contentType === "video" && (
          <div className="mt-4">
            <VideoUploader value={video} onChange={setVideo} />
          </div>
        )}

        {/* 标签输入（设计稿占位：标签 #月色；共用组件） */}
        <div className="mt-6">
          <TagInput tags={tags} onChange={setTags} />
        </div>

        {/* 可见性（设计稿：公开 按钮 +「谁可以看」弹层；共用组件） */}
        <div className="mt-6">
          <VisibilitySelect value={visibility} onChange={setVisibility} />
        </div>

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
          </div>{/* 左栏：发布内容 结束 */}

          {/* 右栏：插件面板（有插件订阅 compose.seo 时桌面显示；移动端自然堆叠到内容下方） */}
          <aside className={`lg:shrink-0 ${hasSeoPlugin ? "lg:w-[360px]" : "lg:hidden"}`}>
            {/* 插件扩展点：compose.seo（M4.1 发帖 SEO 面板——由 seo-optimizer 插件渲染；
                无插件时槽位为空；卸载后自动还原）。
                编辑模式须等详情回填完成再挂载（PluginSlot props 为挂载时快照，
                提前挂载 initial 恒为 null，已发布帖子的 SEO 历史值无法回填） */}
            {editLoaded && (
              <PluginSlot
                slot="compose.seo"
                props={{ initial: seo, onChange: setSeo }}
                onPluginsChange={(count) => setHasSeoPlugin(count > 0)}
              />
            )}
          </aside>
        </div>{/* 两栏布局 结束 */}
      </main>
      <MobileTabbar />
    </div>
  );
}
