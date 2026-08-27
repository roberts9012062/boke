// frontend/src/components/compose/ai-assist-panel.tsx
// 发帖 AI 辅助面板（MiniMax 多模态）：内容扩写 / 润色 / 配图 / 配乐 / 图片识别。
// 文本类结果可替换或追加到正文；配图进图集；配乐设为帖子音频；识图对已选图片生成描述。
// 任务提示词与启停在后台「AI 设置-任务配置」调整（post.expand / post.polish /
// post.image / post.music / image.recognize）。

"use client";

import { useState } from "react";

import { apiAiAssist, ApiError, type AiAssistAction, type AiAssistResult } from "@/lib/api";
import type { MediaDTO } from "@/types/api";

// 动作定义（按钮顺序即展示顺序）。
const ACTIONS: readonly { key: AiAssistAction; label: string; hint: string }[] = [
  { key: "expand", label: "扩写", hint: "扩充实内容，保留原风格" },
  { key: "polish", label: "润色", hint: "修正错别字，优化表达" },
  { key: "image", label: "配图", hint: "按内容意境生成封面图" },
  { key: "music", label: "配乐", hint: "按内容氛围生成纯音乐" },
  { key: "recognize", label: "识图", hint: "识别已选图片内容与文字" },
];

// toMediaDTO 生成结果转媒体项（纯函数；id=0 表示登记缺失，仅地址可用）。
function toMediaDTO(result: AiAssistResult): MediaDTO {
  return {
    id: result.media_id ?? 0,
    type: result.media_type === "audio" ? "audio" : "image",
    url: result.media_url ?? "",
    mime_type: result.media_mime ?? "",
    size_bytes: result.media_size ?? 0,
    width: 0,
    height: 0,
  };
}

// AiAssistPanel 发帖 AI 辅助面板。
// 参数：contentText 正文纯文本（AI 输入）；images 已选图集（识图来源）；
//       onApplyText 文本应用（replace 替换正文 / append 追加）；onAddImage 配图入图集；
//       onSetAudio 配乐设为帖子音频（替换现有音频）。
export function AiAssistPanel({
  contentText,
  images,
  onApplyText,
  onAddImage,
  onSetAudio,
}: {
  contentText: string;
  images: MediaDTO[];
  onApplyText: (text: string, mode: "replace" | "append") => void;
  onAddImage: (media: MediaDTO) => void;
  onSetAudio: (media: MediaDTO) => void;
}) {
  const [busy, setBusy] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [textResult, setTextResult] = useState<string>(""); // 文本结果（识图/扩写/润色预览）
  const [recognizeOpen, setRecognizeOpen] = useState<boolean>(false);
  const [recognizeIndex, setRecognizeIndex] = useState<number>(0);

  // run 执行动作并应用结果（文本类进预览；生成类直接应用）
  const run = async (action: AiAssistAction): Promise<void> => {
    if (busy) return;
    setError("");
    setBusy(action);
    try {
      const imageURL =
        action === "recognize" && images[recognizeIndex]
          ? `${window.location.origin}${images[recognizeIndex].url}`
          : "";
      const result = await apiAiAssist(action, contentText, imageURL);
      if (result.text) {
        setTextResult(result.text);
      } else if (result.media_type === "image") {
        onAddImage(toMediaDTO(result));
        setTextResult("");
      } else if (result.media_type === "audio") {
        onSetAudio(toMediaDTO(result));
        setTextResult("");
      }
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "AI 辅助失败，请稍后重试");
    } finally {
      setBusy("");
    }
  };

  return (
    <div className="rounded-lg border border-line bg-elevated/60 px-4 py-3">
      {/* 标题行 + 动作按钮 */}
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-ink-2">AI 辅助</span>
        {ACTIONS.map((item) => (
          <button
            key={item.key}
            type="button"
            title={item.hint}
            disabled={busy !== "" || (item.key === "recognize" && images.length === 0)}
            onClick={() => {
              if (item.key === "recognize") {
                setRecognizeOpen((v) => !v);
                setTextResult("");
                return;
              }
              void run(item.key);
            }}
            className={`rounded-full border px-3 py-1 text-xs transition-colors disabled:opacity-50 ${
              item.key === "recognize" && recognizeOpen
                ? "border-accent/50 text-glow"
                : "border-line text-ink-2 hover:border-accent/40 hover:text-ink"
            }`}
          >
            {busy === item.key ? "生成中…" : `✦ ${item.label}`}
          </button>
        ))}
        {images.length === 0 && (
          <span className="text-[10px] text-ink-3">上传图片后可识别</span>
        )}
      </div>

      {/* 识图：选择图集中的图片 */}
      {recognizeOpen && images.length > 0 && (
        <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-line pt-3">
          <span className="text-xs text-ink-3">选择图片：</span>
          {images.map((img, index) => (
            <button
              key={img.id || index}
              type="button"
              onClick={() => setRecognizeIndex(index)}
              className={`h-10 w-10 overflow-hidden rounded border-2 transition-colors ${
                index === recognizeIndex ? "border-accent" : "border-transparent opacity-70"
              }`}
              aria-label={`识别第 ${index + 1} 张图片`}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={img.url} alt="" className="h-full w-full object-cover" />
            </button>
          ))}
          <button
            type="button"
            disabled={busy !== ""}
            onClick={() => void run("recognize")}
            className="rounded-full bg-ink px-3 py-1 text-xs text-white disabled:opacity-50"
          >
            {busy === "recognize" ? "识别中…" : "开始识别"}
          </button>
        </div>
      )}

      {/* 文本结果预览（识图描述 / 扩写润色结果确认） */}
      {textResult && (
        <div className="mt-3 space-y-2 border-t border-line pt-3">
          <p className="max-h-40 overflow-y-auto whitespace-pre-wrap break-words rounded bg-muted px-3 py-2 text-xs leading-5 text-ink-2">
            {textResult}
          </p>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => {
                onApplyText(textResult, "replace");
                setTextResult("");
              }}
              className="rounded bg-ink px-3 py-1 text-xs text-white"
            >
              替换正文
            </button>
            <button
              type="button"
              onClick={() => {
                onApplyText(textResult, "append");
                setTextResult("");
              }}
              className="rounded border border-line px-3 py-1 text-xs text-ink-2"
            >
              追加到正文
            </button>
            <button
              type="button"
              onClick={() => setTextResult("")}
              className="rounded px-2 py-1 text-xs text-ink-3"
            >
              忽略
            </button>
          </div>
        </div>
      )}

      {error && <p className="mt-2 text-xs text-red-500">{error}</p>}
    </div>
  );
}
