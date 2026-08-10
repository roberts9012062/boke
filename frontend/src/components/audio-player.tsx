// src/components/audio-player.tsx
// 音频播放条（设计稿 Card Audio / 音频播放画板）：
// 播放/暂停、进度条（可点击跳转）、当前时间/总时长。
// 说明：使用原生 <audio> 元素承载播放，进度与时间由 JS 状态驱动。
"use client";

import { useEffect, useRef, useState } from "react";

// formatTime 将秒数格式化为 m:ss。
function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "0:00";
  }
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

// AudioPlayer 音频播放条。
// 参数：src 音频地址；duration 预估时长（秒，未知传 0）；autoplay 是否自动播放（外观设置驱动）。
export function AudioPlayer({ src, duration, autoplay }: { src: string; duration: number; autoplay: boolean }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState<boolean>(false);
  const [current, setCurrent] = useState<number>(0);
  const [total, setTotal] = useState<number>(duration);
  const [loaded, setLoaded] = useState<boolean>(false);

  // 音频元数据加载完成：获取真实总时长
  const handleLoadedMetadata = () => {
    const audio = audioRef.current;
    if (audio && Number.isFinite(audio.duration)) {
      setTotal(audio.duration);
    }
    setLoaded(true);
  };

  // 自动播放（外观设置）与真实播放状态同步
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !autoplay || !loaded) {
      return;
    }
    // 静音自动播放（浏览器自动播放策略要求）
    audio.muted = true;
    audio.play().catch(() => {
      // 自动播放被浏览器拦截时静默（用户手动点击播放）
    });
  }, [autoplay, loaded]);

  // 播放状态同步（原生事件驱动，自动播放/手动均一致）
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    return () => {
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
    };
  }, []);

  // 播放/暂停切换
  const togglePlay = async () => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    if (audio.paused) {
      await audio.play();
    } else {
      audio.pause();
    }
  };

  // 点击进度条跳转
  const seek = (e: React.MouseEvent<HTMLDivElement>) => {
    const audio = audioRef.current;
    if (!audio || total <= 0) {
      return;
    }
    const rect = e.currentTarget.getBoundingClientRect();
    const ratio = Math.min(Math.max((e.clientX - rect.left) / rect.width, 0), 1);
    audio.currentTime = ratio * total;
  };

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) {
      return;
    }
    // 时间更新与播放状态同步
    const onTime = () => setCurrent(audio.currentTime);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onEnd = () => {
      setPlaying(false);
      setCurrent(0);
    };
    audio.addEventListener("timeupdate", onTime);
    audio.addEventListener("play", onPlay);
    audio.addEventListener("pause", onPause);
    audio.addEventListener("ended", onEnd);
    return () => {
      audio.removeEventListener("timeupdate", onTime);
      audio.removeEventListener("play", onPlay);
      audio.removeEventListener("pause", onPause);
      audio.removeEventListener("ended", onEnd);
    };
  }, []);

  // 播放进度百分比
  const progress = total > 0 ? (current / total) * 100 : 0;

  return (
    <div className="flex items-center gap-3 rounded-lg border border-line bg-muted/60 px-4 py-3">
      {/* 隐藏的 audio 元素（承载播放） */}
      <audio ref={audioRef} src={src} preload="metadata" onLoadedMetadata={handleLoadedMetadata} />

      {/* 播放/暂停按钮（accent 圆形） */}
      <button
        type="button"
        onClick={togglePlay}
        className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-accent text-sm text-on-accent transition-opacity hover:opacity-90"
        aria-label={playing ? "暂停" : "播放"}
      >
        {playing ? "❚❚" : "▶"}
      </button>

      {/* 进度条 + 时间 */}
      <div className="min-w-0 flex-1">
        <div
          className="group relative h-1.5 cursor-pointer rounded-full bg-line"
          onClick={seek}
          role="slider"
          aria-label="音频进度"
          aria-valuemin={0}
          aria-valuemax={Math.round(total)}
          aria-valuenow={Math.round(current)}
        >
          {/* 已播放部分（accent 色） */}
          <div
            className="absolute left-0 top-0 h-full rounded-full bg-accent"
            style={{ width: `${progress}%` }}
          />
        </div>
        {/* 时间（设计稿：1:24 / 4:08） */}
        <div className="mt-1 flex justify-between text-[11px] text-ink-3">
          <span>{formatTime(current)}</span>
          <span>{loaded || total > 0 ? formatTime(total) : "…"}</span>
        </div>
      </div>
    </div>
  );
}
