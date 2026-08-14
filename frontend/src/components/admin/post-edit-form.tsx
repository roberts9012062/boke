// src/components/admin/post-edit-form.tsx
// 后台编辑帖子 · 编辑区（设计稿《后台编辑·文字/图片/音频/视频》左栏）：
// 标题 / 正文（文字帖）或媒体区（图片多张 / 音频 / 视频）+ 说明 / 标签 / 可见性；
// 保存（草稿/发布）由父组件经 ref 触发，成功后回调 onSaved 刷新页面数据。
"use client";

import { forwardRef, useImperativeHandle, useState } from "react";

import { AudioUploader } from "@/components/compose/audio-uploader";
import { ImageUploader } from "@/components/compose/image-uploader";
import { MAX_CONTENT } from "@/components/compose/config";
import { RichTextEditor } from "@/components/compose/rich-text-editor";
import { VideoUploader } from "@/components/compose/video-uploader";
import { apiAdminUpdatePost, apiUploadMedia, ApiError } from "@/lib/api";
import { apiAiGenReply, apiAiGenTags } from "@/lib/api-ai";
import { htmlToText } from "@/lib/rich-text";
import type { AdminPostDetail, MediaDTO } from "@/types/api";

// 可见性选项（设计稿《可见性》弹层三选项：公开/仅关注者/仅自己）
const VISIBILITY_OPTIONS = [
  { key: "public", label: "公开" },
  { key: "followers", label: "仅关注者" },
  { key: "private", label: "仅自己" },
] as const;

// parseTags 解析标签输入（空格分隔，支持 # 前缀："#月色 #夜读" → ["月色","夜读"]）。
function parseTags(text: string): string[] {
  return text
    .split(/\s+/)
    .map((t) => t.replace(/^#/, ""))
    .filter(Boolean);
}

// legacyContentToHtml 将旧格式（markdown/纯文本）正文转为编辑器可安全加载的 HTML。
// 说明：旧帖正文不含 HTML 语义，直接转义后按段落回填，避免 `<`/`&` 被误解析。
function legacyContentToHtml(text: string): string {
  const escaped = text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  if (escaped.trim() === "") {
    return "";
  }
  return escaped
    .split(/\n{2,}/)
    .map((p) => `<p>${p.replace(/\n/g, "<br>")}</p>`)
    .join("");
}

// PostEditFormHandle 表单操作句柄（父组件触发保存）。
export interface PostEditFormHandle {
  save: (status: "draft" | "published") => Promise<boolean>; // 保存成功返回 true
}

// PostEditForm 编辑区表单（四类型：文字/图片/音频/视频）。
export const PostEditForm = forwardRef<
  PostEditFormHandle,
  { detail: AdminPostDetail; onSaved: () => void }
>(function PostEditForm({ detail, onSaved }, ref) {
  // 表单状态（从详情初始化；类型只读，媒体按类型独立管理）
  const [title, setTitle] = useState<string>(detail.title);
  const [content, setContent] = useState<string>(
    detail.content_format === "html" ? detail.content : legacyContentToHtml(detail.content),
  );
  const [tagsText, setTagsText] = useState<string>(detail.tags.map((t) => `#${t}`).join(" "));
  const [visibility, setVisibility] = useState<string>(detail.visibility);
  const [images, setImages] = useState<MediaDTO[]>(detail.media);
  const [audio, setAudio] = useState<MediaDTO | null>(detail.media[0] ?? null);
  const [video, setVideo] = useState<MediaDTO | null>(detail.media[0] ?? null);
  const [coverUrl, setCoverUrl] = useState<string>(detail.cover_url);
  const [error, setError] = useState<string>("");
  // AI 标签建议（M4：生成后展示 chips，点击合并；不自动写入）
  const [aiTags, setAiTags] = useState<string[]>([]);
  const [aiBusy, setAiBusy] = useState(false);
  const [aiError, setAiError] = useState("");
  // AI 智能回复助手（续写/润色/翻译）
  const [replyBusy, setReplyBusy] = useState<string>(""); // 空=空闲；否则为当前动作
  const [replyError, setReplyError] = useState("");

  // genTags AI 生成标签建议（失败提示原因，未配 Key 引导去 AI 设置）。
  const genTags = async () => {
    setAiBusy(true);
    setAiError("");
    try {
      const r = await apiAiGenTags(detail.id);
      setAiTags(r.tags);
      if (r.tags.length === 0) {
        setAiError("AI 未返回标签建议，请重试");
      }
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "生成失败";
      setAiError(msg.includes("AI 设置") || msg.includes("API Key") ? `${msg}（可前往侧栏「AI 设置」配置）` : msg);
    } finally {
      setAiBusy(false);
    }
  };

  // mergeAiTag 合并建议标签进输入框（去重，≤5 个）。
  const mergeAiTag = (tag: string) => {
    setTagsText((prev) => {
      const current = parseTags(prev);
      if (current.includes(tag) || current.length >= 5) {
        return prev;
      }
      return [...current, tag].map((t) => `#${t}`).join(" ");
    });
  };

  // genReply AI 智能回复助手（续写/润色/翻译）：结果追加到正文末尾。
  // 注意：正文已是 HTML，传给 AI 前先 htmlToText 取纯文本；结果按新段落追加回 HTML。
  const genReply = async (action: "continue" | "polish" | "translate") => {
    setReplyBusy(action);
    setReplyError("");
    try {
      const r = await apiAiGenReply(detail.id, action, htmlToText(content));
      if (!r.text) {
        setReplyError("AI 未返回内容，请重试");
        return;
      }
      // 追加结果（新段落；转义 AI 文本避免破坏 HTML）
      const escaped = r.text
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;");
      setContent(`${content}<p>${escaped}</p>`);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "生成失败";
      setReplyError(msg.includes("AI 设置") || msg.includes("API Key") ? `${msg}（可前往侧栏「AI 设置」配置）` : msg);
    } finally {
      setReplyBusy("");
    }
  };

  // save 保存帖子（draft=保存草稿 / published=更新发布；成功返回 true）。
  const save = async (status: "draft" | "published"): Promise<boolean> => {
    setError("");
    // 按类型组装媒体 ID（图片多张有序 / 音频或视频一张）
    const mediaIDs =
      detail.content_type === "image"
        ? images.map((m) => m.id)
        : detail.content_type === "audio"
          ? audio
            ? [audio.id]
            : []
          : detail.content_type === "video"
            ? video
              ? [video.id]
              : []
            : [];
    // 封面：视频帖更换过才显式提交（其余类型后端按媒体推断/保留）
    const cover =
      detail.content_type === "video" && coverUrl !== detail.cover_url ? coverUrl : undefined;
    try {
      await apiAdminUpdatePost(detail.id, {
        title,
        content,
        content_format: "html",
        tags: parseTags(tagsText),
        media_ids: mediaIDs,
        visibility,
        cover_url: cover,
        status,
      });
      onSaved();
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
      return false;
    }
  };
  useImperativeHandle(ref, () => ({ save }));

  // changeCover 更换视频封面：上传图片 → 更新封面 URL（设计稿「更换封面」）。
  const changeCover = async (file: File | undefined) => {
    if (!file) {
      return;
    }
    setError("");
    try {
      const result = await apiUploadMedia(file);
      setCoverUrl(result.url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "封面上传失败");
    }
  };

  return (
    <div className="space-y-5">
      {/* 标题（设计稿：月光落在窗台上） */}
      <div>
        <label htmlFor="edit-title" className="mb-1.5 block text-sm text-ink-2">
          标题
        </label>
        <input
          id="edit-title"
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={100}
          placeholder="起个标题（可选）"
          className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </div>

      {/* 媒体区（按类型：图片多张 / 音频 / 视频+封面） */}
      {detail.content_type === "image" && (
        <div>
          <p className="mb-1.5 text-sm text-ink-2">图片（≤9 张）</p>
          <ImageUploader value={images} onChange={setImages} />
        </div>
      )}
      {detail.content_type === "audio" && (
        <div>
          <p className="mb-1.5 text-sm text-ink-2">音频文件</p>
          <AudioUploader value={audio} onChange={setAudio} />
        </div>
      )}
      {detail.content_type === "video" && (
        <div>
          <p className="mb-1.5 text-sm text-ink-2">视频</p>
          <VideoUploader value={video} onChange={setVideo} />
          {/* 封面（设计稿：更换封面；视频帖独立封面图） */}
          <div className="mt-3 flex items-center gap-3">
            {coverUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={coverUrl} alt="封面预览" className="h-14 w-24 rounded-md border border-line object-cover" />
            ) : (
              <span className="flex h-14 w-24 items-center justify-center rounded-md border border-dashed border-line text-xs text-ink-3">
                暂无封面
              </span>
            )}
            <label
              htmlFor="edit-cover"
              className="cursor-pointer rounded-full border border-line px-4 py-1.5 text-xs text-ink-2 hover:text-ink"
            >
              更换封面
            </label>
            <input
              id="edit-cover"
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(e) => void changeCover(e.target.files?.[0])}
            />
          </div>
        </div>
      )}

      {/* 正文 / 图说 / 说明（设计稿各画板对应文案） */}
      <div>
        <div className="mb-1.5 flex items-center justify-between">
          <label htmlFor="edit-content" className="block text-sm text-ink-2">
            {detail.content_type === "text" ? "正文" : detail.content_type === "image" ? "图说 / 正文" : "说明"}
          </label>
          {/* AI 智能回复助手（续写/润色/翻译，结果追加到正文末尾） */}
          <div className="flex gap-1.5">
            {(
              [
                { key: "continue", label: "续写" },
                { key: "polish", label: "润色" },
                { key: "translate", label: "翻译" },
              ] as const
            ).map((a) => (
              <button
                key={a.key}
                type="button"
                onClick={() => void genReply(a.key)}
                disabled={replyBusy !== ""}
                className="rounded-full bg-accent/10 px-2.5 py-1 text-xs text-glow hover:bg-accent/20 disabled:opacity-50"
              >
                {replyBusy === a.key ? "处理中…" : `AI ${a.label}`}
              </button>
            ))}
          </div>
        </div>
        {/* 正文（M5 富文本：WYSIWYG，图片上传/视频内嵌/外链；与发帖页共用编辑器） */}
        <RichTextEditor
          value={content}
          onChange={(v) => setContent(v)}
          placeholder="写下内容…"
          maxLength={MAX_CONTENT}
        />
        {replyError && <p className="mt-1 text-xs text-like">{replyError}</p>}
      </div>

      {/* 标签（设计稿：#月色 #夜读 #随笔；M4 增加 AI 生成建议） */}
      <div>
        <div className="mb-1.5 flex items-center justify-between">
          <label htmlFor="edit-tags" className="block text-sm text-ink-2">
            标签
          </label>
          <button
            type="button"
            onClick={() => void genTags()}
            disabled={aiBusy}
            className="rounded-full bg-accent/10 px-3 py-1 text-xs text-glow hover:bg-accent/20 disabled:opacity-50"
          >
            {aiBusy ? "生成中…" : "AI 生成标签"}
          </button>
        </div>
        <input
          id="edit-tags"
          type="text"
          value={tagsText}
          onChange={(e) => setTagsText(e.target.value)}
          placeholder="#月色 #夜读 #随笔（空格分隔，≤5 个）"
          className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
        {/* AI 建议标签（点击合并进输入框，≤5 个） */}
        {aiTags.length > 0 && (
          <div className="mt-2 flex flex-wrap gap-2">
            {aiTags.map((tag) => (
              <button
                key={tag}
                type="button"
                onClick={() => mergeAiTag(tag)}
                className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:border-accent hover:text-glow"
              >
                #{tag} ＋
              </button>
            ))}
          </div>
        )}
        {aiError && <p className="mt-2 text-xs text-like">{aiError}</p>}
      </div>

      {/* 可见性（设计稿《可见性》弹层三选项） */}
      <div>
        <p className="mb-1.5 text-sm text-ink-2">可见性</p>
        <div className="flex flex-wrap gap-2">
          {VISIBILITY_OPTIONS.map((opt) => (
            <button
              key={opt.key}
              type="button"
              onClick={() => setVisibility(opt.key)}
              className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                visibility === opt.key
                  ? "border-accent bg-accent-soft text-glow"
                  : "border-line text-ink-2 hover:text-ink"
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}
    </div>
  );
});
