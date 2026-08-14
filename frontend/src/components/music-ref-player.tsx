// src/components/music-ref-player.tsx
// 网易云音乐自研播放器（M7 插件）：音乐引用渲染——
//   接收歌曲 ID/歌名/歌手/封面，实时经插件 API 获取播放地址，<audio> 自研 UI 播放。
//   双主题适配（CSS 变量），不再嵌套第三方 iframe。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// formatTime 秒数格式化 m:ss（纯函数）。
function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "0:00";
  }
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

// MusicRefPlayer 音乐自研播放器（网易云 / QQ 音乐）。
// 参数：songId 歌曲 ID（网易云 song_id / QQ songmid）；title 歌名；artist 歌手；
//       coverUrl 封面（可空）；platform 平台（netease / qq，默认 netease）。
export function MusicRefPlayer({
  songId,
  title,
  artist,
  coverUrl,
  platform = "netease",
}: {
  songId: string;
  title?: string;
  artist?: string;
  coverUrl?: string;
  platform?: "netease" | "qq";
}) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [url, setUrl] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [playing, setPlaying] = useState<boolean>(false);
  const [current, setCurrent] = useState<number>(0);
  const [total, setTotal] = useState<number>(0);

  // 获取播放地址（主进程公开端点 → 转发插件；访客无需登录）
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError("");
    const url =
      platform === "qq"
        ? `/api/v1/music/qq-url?songmid=${encodeURIComponent(songId)}`
        : `/api/v1/music/netease-url?song_id=${encodeURIComponent(songId)}`;
    fetch(url)
      .then((res) => res.json())
      .then((r: { url?: string; error?: string }) => {
        if (cancelled) return;
        if (r.error) {
          setError(r.error);
        } else if (r.url) {
          setUrl(r.url);
        }
        setLoading(false);
      })
      .catch(() => {
        if (!cancelled) {
          setError("获取播放地址失败");
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [songId, platform]);

  // 播放/暂停切换
  const togglePlay = useCallback(() => {
    const audio = audioRef.current;
    if (!audio) return;
    if (audio.paused) {
      void audio.play();
    } else {
      audio.pause();
    }
  }, []);

  // 点击进度条跳转
  const seek = (e: React.MouseEvent<HTMLDivElement>) => {
    const audio = audioRef.current;
    if (!audio || total <= 0) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const ratio = Math.min(Math.max((e.clientX - rect.left) / rect.width, 0), 1);
    audio.currentTime = ratio * total;
  };

  const progress = total > 0 ? (current / total) * 100 : 0;

  return (
    <div className="my-3 flex items-center gap-3 rounded-lg border border-line bg-elevated p-3">
      {/* 隐藏 audio（承载播放；url 空时不设 src，避免空字符串属性警告） */}
      <audio
        ref={audioRef}
        src={url || undefined}
        preload="metadata"
        onLoadedMetadata={(e) => setTotal(e.currentTarget.duration)}
        onTimeUpdate={(e) => setCurrent(e.currentTarget.currentTime)}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onEnded={() => {
          setPlaying(false);
          setCurrent(0);
        }}
      />

      {/* 封面 / 占位 */}
      <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted text-lg text-ink-3">
        {coverUrl ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={coverUrl} alt="" referrerPolicy="no-referrer" className="h-full w-full object-cover" />
        ) : (
          "♪"
        )}
      </div>

      {/* 歌名 / 歌手 */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium text-ink">{title || "网易云音乐"}</p>
        <p className="truncate text-xs text-ink-3">{artist || "未知歌手"}</p>
        {/* 进度条 */}
        <div
          className="mt-1.5 h-1 cursor-pointer rounded-full bg-line"
          onClick={seek}
          role="slider"
          aria-label="播放进度"
          aria-valuemin={0}
          aria-valuemax={Math.round(total)}
          aria-valuenow={Math.round(current)}
        >
          <div
            className="h-full rounded-full bg-accent transition-[width] duration-150 ease-linear"
            style={{ width: `${progress}%` }}
          />
        </div>
        {/* 时间 */}
        <div className="mt-0.5 flex justify-between text-[10px] text-ink-3">
          <span>{formatTime(current)}</span>
          <span>{total > 0 ? formatTime(total) : error || (loading ? "…" : "—")}</span>
        </div>
      </div>

      {/* 播放按钮 */}
      <button
        type="button"
        onClick={togglePlay}
        disabled={!url || loading}
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent text-sm text-on-accent transition-opacity hover:opacity-90 disabled:opacity-50"
        aria-label={playing ? "暂停" : "播放"}
      >
        {loading ? "…" : playing ? "❚❚" : "▶"}
      </button>
    </div>
  );
}
