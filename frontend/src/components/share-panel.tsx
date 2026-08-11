// src/components/share-panel.tsx
// 分享面板（设计稿 D/冷月/分享面板 1400×700 + M/冷月/分享面板 390×700）：
// 分享帖子 + 标题/作者·话题 → 四操作（复制链接/生成海报/私信好友/二维码）+ 链接预览与复制。
// #17 完整实现：复制链接真实；生成海报（canvas 冷月夜色海报 + 保存下载）；二维码（qrcode 库生成 + 下载）；
// 私信好友跳转消息中心（转发选人流程后置，差异记录）。
"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { downloadDataUrl, drawSharePoster, qrDataUrl, type ShareContent } from "@/lib/share";

// 分享操作项（设计稿四宫格）
const ACTIONS = [
  { key: "copy", icon: "🔗", label: "复制链接", desc: "复制到剪贴板" },
  { key: "poster", icon: "🖼️", label: "生成海报", desc: "图片分享" },
  { key: "message", icon: "💬", label: "私信好友", desc: "发送给好友" },
  { key: "qr", icon: "▦", label: "二维码", desc: "扫码打开" },
] as const;

// SharePanel 分享面板弹层。
// 参数：title 帖子标题；content 正文（海报正文区，可空）；media 图片 URL（海报图片区，可空）；
//       meta 作者 · 话题；shareUrl 分享链接；onClose 关闭回调。
export function SharePanel({
  title,
  content,
  media,
  meta,
  shareUrl,
  onClose,
}: {
  title: string;
  content?: string;
  media?: string[];
  meta: string;
  shareUrl: string;
  onClose: () => void;
}) {
  const router = useRouter();
  const [copied, setCopied] = useState<boolean>(false);
  // 视图：menu=四宫格 / qr=二维码 / poster=海报
  const [view, setView] = useState<"menu" | "qr" | "poster">("menu");
  const [qrData, setQrData] = useState<string>("");
  const [posterData, setPosterData] = useState<string>("");
  const [posterLoading, setPosterLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 组装分享内容（标题/正文/作者/话题/链接/图片）
  const shareContent = (): ShareContent => ({
    title,
    content: content || title,
    author: meta.split(" · ")[0] ?? meta,
    tags: meta.includes(" · ") ? meta.split(" · ").slice(1).join(" · ") : "",
    link: shareUrl,
    media,
  });

  // 复制链接（真实功能；失败降级提示）
  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("复制失败，请手动复制下方链接");
    }
  };

  // 操作点击
  const handleAction = async (key: string) => {
    setError("");
    if (key === "copy") {
      await handleCopy();
      return;
    }
    if (key === "message") {
      // 私信好友：跳转消息中心（转发选人流程后置）
      router.push("/messages");
      return;
    }
    if (key === "qr") {
      // 二维码视图：生成链接二维码
      setView("qr");
      return;
    }
    // 生成海报：二维码 + 海报 canvas 绘制
    setPosterLoading(true);
    try {
      const qr = await qrDataUrl(shareUrl, 200);
      setQrData(qr);
      const poster = await drawSharePoster(shareContent(), qr);
      setPosterData(poster);
      setView("poster");
    } catch (err) {
      setError(err instanceof Error ? err.message : "海报生成失败");
    } finally {
      setPosterLoading(false);
    }
  };

  // 二维码视图：进入时生成
  useEffect(() => {
    if (view === "qr" && !qrData) {
      qrDataUrl(shareUrl, 260)
        .then(setQrData)
        .catch(() => setError("二维码生成失败"));
    }
  }, [view, qrData, shareUrl]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-black/50 sm:items-center sm:px-6"
      role="dialog"
      aria-modal="true"
      aria-label="分享帖子"
      onClick={onClose}
    >
      <div
        className="relative w-full max-w-[420px] rounded-t-2xl border border-line bg-elevated p-6 sm:rounded-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* 头部（设计稿：分享帖子 + 标题 + 作者 · 话题） */}
        <h2 className="font-display text-lg font-semibold text-ink">
          {view === "qr" ? "二维码" : view === "poster" ? "生成海报" : "分享帖子"}
        </h2>
        {view === "menu" && (
          <>
            <p className="mt-1 line-clamp-1 text-sm text-ink">{title}</p>
            <p className="mt-0.5 text-xs text-ink-3">{meta}</p>
          </>
        )}

        {/* 视图：四宫格（默认） */}
        {view === "menu" && (
          <>
            <div className="mt-5 grid grid-cols-4 gap-3">
              {ACTIONS.map((action) => (
                <button
                  key={action.key}
                  type="button"
                  onClick={() => void handleAction(action.key)}
                  className="flex flex-col items-center gap-1.5 rounded-lg border border-line py-3 transition-colors hover:border-accent"
                >
                  <span className="text-xl" aria-hidden>
                    {action.icon}
                  </span>
                  <span className="text-xs text-ink">{action.label}</span>
                  <span className="text-[10px] text-ink-3">{action.desc}</span>
                </button>
              ))}
            </div>
            {error && (
              <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-xs text-like" role="alert">
                {error}
              </p>
            )}
            {/* 链接预览 + 复制（设计稿） */}
            <div className="mt-5 flex items-center gap-2 rounded-lg border border-line bg-muted px-3 py-2.5">
              <span className="min-w-0 flex-1 truncate text-xs text-ink-3">{shareUrl}</span>
              <button
                type="button"
                onClick={() => void handleCopy()}
                className="shrink-0 rounded-full bg-accent px-4 py-1.5 text-xs font-medium text-on-accent hover:opacity-90"
              >
                {copied ? "已复制 ✓" : "复制"}
              </button>
            </div>
          </>
        )}

        {/* 视图：二维码（生成 + 下载） */}
        {view === "qr" && (
          <div className="mt-5 flex flex-col items-center">
            {qrData ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={qrData} alt="分享二维码" className="h-52 w-52 rounded-lg border border-line" />
            ) : (
              <div className="h-52 w-52 animate-pulse rounded-lg bg-muted" aria-hidden />
            )}
            <p className="mt-3 text-xs text-ink-3">{shareUrl}</p>
            <button
              type="button"
              disabled={!qrData}
              onClick={() => qrData && downloadDataUrl(qrData, "yueyan-share-qr.png")}
              className="mt-4 rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
            >
              保存二维码
            </button>
            <button
              type="button"
              onClick={() => setView("menu")}
              className="mt-2 text-xs text-ink-3 hover:text-ink"
            >
              ← 返回
            </button>
          </div>
        )}

        {/* 视图：海报（预览 + 保存下载） */}
        {view === "poster" && (
          <div className="mt-5 flex flex-col items-center">
            {posterLoading ? (
              <div className="flex h-72 w-full items-center justify-center">
                <p className="text-sm text-ink-3">海报生成中…</p>
              </div>
            ) : posterData ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={posterData}
                alt="分享海报"
                className="max-h-[420px] w-full rounded-lg border border-line object-contain"
              />
            ) : (
              <p className="py-12 text-sm text-ink-3">海报生成失败</p>
            )}
            <button
              type="button"
              disabled={!posterData}
              onClick={() => posterData && downloadDataUrl(posterData, "yueyan-poster.png")}
              className="mt-4 rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
            >
              保存海报
            </button>
            <button
              type="button"
              onClick={() => setView("menu")}
              className="mt-2 text-xs text-ink-3 hover:text-ink"
            >
              ← 返回
            </button>
          </div>
        )}

        {/* 移动端取消（设计稿 M：取消） */}
        <button
          type="button"
          onClick={onClose}
          className="mt-4 w-full rounded-full border border-line py-2.5 text-sm text-ink-2 hover:text-ink sm:hidden"
        >
          取消
        </button>
        {/* 桌面关闭（右上角 ×） */}
        <button
          type="button"
          onClick={onClose}
          aria-label="关闭"
          className="absolute right-4 top-4 hidden h-8 w-8 items-center justify-center rounded-full text-ink-3 hover:bg-muted hover:text-ink sm:flex"
        >
          ✕
        </button>
      </div>
    </div>
  );
}
