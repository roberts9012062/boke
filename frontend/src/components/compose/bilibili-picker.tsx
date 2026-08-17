// src/components/compose/bilibili-picker.tsx
// B站视频插入弹窗（bilibili-video 插件增强）：
//   输入 B 站视频地址（网页链接 / BV 号 / b23.tv 短链）→ 调插件 /resolve 解析
//   视频信息与清晰度档位（360P~1080P）→ 单选清晰度 → 插入 bilibiliEmbed 块。
//   插件未登录 B 站时，720P/1080P 档位标注「需登录」（后台上传插件页扫码）。
"use client";

import { useState } from "react";

import { authHeaders, ApiError } from "@/lib/api";

import type { BilibiliEmbedAttrs, BilibiliQuality } from "./bilibili-embed";

// BilibiliPickerProps 弹窗参数。
interface BilibiliPickerProps {
  defaultUrl: string; // 预填地址（从通用视频弹窗带入）
  onInsert: (attrs: BilibiliEmbedAttrs) => void; // 插入回调
  onClose: () => void; // 取消回调
}

// BilibiliVideoInfo 插件 /resolve 返回的视频信息。
interface BilibiliVideoInfo {
  bvid: string;
  cid: number;
  title: string;
  cover: string;
  duration: number;
  author: string;
}

// ResolvedState 解析成功后的暂存状态。
interface ResolvedState {
  video: BilibiliVideoInfo;
  qualities: BilibiliQuality[];
  adminLoggedIn: boolean;
}

// BilibiliPicker B站视频解析插入弹窗。
export function BilibiliPicker({ defaultUrl, onInsert, onClose }: BilibiliPickerProps) {
  const [url, setUrl] = useState(defaultUrl);
  const [resolving, setResolving] = useState(false);
  const [error, setError] = useState("");
  const [resolved, setResolved] = useState<ResolvedState | null>(null);
  const [quality, setQuality] = useState(32);

  // resolve 调插件解析地址（需登录——发帖场景必然已登录）。
  const resolve = async () => {
    const input = url.trim();
    if (!input) {
      setError("请输入 B 站视频地址");
      return;
    }
    setResolving(true);
    setError("");
    try {
      const res = await fetch("/api/v1/plugins/bilibili-video/resolve", {
        method: "POST",
        headers: { "Content-Type": "application/json", ...authHeaders() },
        body: JSON.stringify({ url: input }),
      });
      const data = (await res.json()) as {
        video?: BilibiliVideoInfo;
        qualities?: BilibiliQuality[];
        admin_logged_in?: boolean;
        error?: string;
      };
      if (data.error || !data.video) {
        setError(data.error ?? "解析失败");
        return;
      }
      const qualities = data.qualities ?? [];
      const highest = qualities.length > 0 ? qualities[qualities.length - 1].qn : 32;
      setResolved({ video: data.video, qualities, adminLoggedIn: Boolean(data.admin_logged_in) });
      setQuality(highest); // 默认选最高档（未登录时播放会自动降级）
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "解析失败，请稍后再试");
    } finally {
      setResolving(false);
    }
  };

  // insert 组装节点属性并回调插入。
  const insert = () => {
    if (!resolved) {
      return;
    }
    onInsert({
      bvid: resolved.video.bvid,
      cid: resolved.video.cid,
      title: resolved.video.title,
      cover: resolved.video.cover,
      author: resolved.video.author,
      duration: resolved.video.duration,
      quality,
      qualitiesJson: JSON.stringify(resolved.qualities),
    });
  };

  return (
    <div className="space-y-3">
      <p className="text-xs text-ink-3">B站视频 · 解析后可选清晰度（360P~1080P），帖子内嵌高清播放器</p>
      <div className="flex gap-2">
        <input
          autoFocus
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !resolved) void resolve();
          }}
          placeholder="https://www.bilibili.com/video/BV... 或 b23.tv 短链"
          className="flex-1 rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
        />
        <button
          type="button"
          disabled={resolving}
          onClick={() => void resolve()}
          className="rounded-lg bg-accent px-4 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
        >
          {resolving ? "解析中…" : "解析"}
        </button>
      </div>
      {error && <p className="text-xs text-like">{error}</p>}

      {resolved && (
        <>
          <div className="flex gap-3 rounded-lg border border-line bg-elevated p-3">
            {resolved.video.cover ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={resolved.video.cover}
                alt={resolved.video.title}
                referrerPolicy="no-referrer"
                className="h-[72px] w-[128px] flex-shrink-0 rounded object-cover"
              />
            ) : (
              <div className="flex h-[72px] w-[128px] flex-shrink-0 items-center justify-center rounded bg-muted text-2xl text-ink-3">▶</div>
            )}
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-ink">{resolved.video.title || resolved.video.bvid}</p>
              <p className="mt-1 text-xs text-ink-3">
                UP：{resolved.video.author || "未知"} · {Math.floor((resolved.video.duration || 0) / 60)}分{String((resolved.video.duration || 0) % 60).padStart(2, "0")}秒
              </p>
              {!resolved.adminLoggedIn && (
                <p className="mt-1 text-xs text-amber-500">未登录 B 站：高清档位游客观看将自动降级（后台上传插件页可扫码登录）</p>
              )}
            </div>
          </div>

          <div>
            <p className="mb-1.5 text-xs font-medium text-ink-2">选择清晰度（观看者仍可切换）</p>
            <div className="flex flex-wrap gap-2">
              {resolved.qualities.map((q) => {
                const active = q.qn === quality;
                const locked = q.need_login && !resolved.adminLoggedIn;
                return (
                  <button
                    key={q.qn}
                    type="button"
                    onClick={() => setQuality(q.qn)}
                    className={`rounded-full border px-3.5 py-1.5 text-xs transition-colors ${
                      active ? "border-accent bg-accent-soft font-medium text-glow" : "border-line bg-muted text-ink-2 hover:text-ink"
                    }`}
                  >
                    {q.desc}
                    {locked && <span className="ml-1 text-[10px] text-ink-3">需登录</span>}
                  </button>
                );
              })}
            </div>
          </div>
        </>
      )}

      <div className="flex justify-end gap-2">
        <button type="button" onClick={onClose} className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink">取消</button>
        <button
          type="button"
          disabled={!resolved}
          onClick={insert}
          className="rounded-full bg-accent px-5 py-1.5 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
        >
          插入视频
        </button>
      </div>
    </div>
  );
}
