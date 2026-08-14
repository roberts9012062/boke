// frontend/src/components/bgm-widget.tsx
// 首页背景音乐悬浮播放器（M7）：站长在 QQ 音乐插件设置中开启后，
// 首页右下角浮现渐变悬浮按钮；hover 展开歌单可切换歌曲；点击按钮暂停/播放。
"use client";

import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { apiQqBgm, type QqBgmSong } from "@/lib/api";

// BgmWidget 首页右下角悬浮播放器（配置关闭或无歌单时不渲染）。
export function BgmWidget() {
  const [songs, setSongs] = useState<QqBgmSong[]>([]);
  const [open, setOpen] = useState<boolean>(false);
  const [playing, setPlaying] = useState<boolean>(false);
  const [current, setCurrent] = useState<QqBgmSong | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);

  // 挂载时拉取配置（关闭或歌单为空时不渲染）
  useEffect(() => {
    let cancelled = false;
    apiQqBgm()
      .then((r) => {
        if (!cancelled && r.enabled && r.songs && r.songs.length > 0) setSongs(r.songs);
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, []);

  if (songs.length === 0) return null;

  // playAt 从指定下标播放；版权受限（fetch 失败/加载错误）时自动顺延下一首，最多尝试 tries 次。
  const playAt = (index: number, tries: number) => {
    if (tries <= 0) return;
    const song = songs[index % songs.length];
    fetch(`/api/v1/music/qq-url?songmid=${encodeURIComponent(song.song_mid)}`)
      .then((res) => res.json())
      .then((r: { url?: string; error?: string }) => {
        if (!r || !r.url) { playAt(index + 1, tries - 1); return; }
        audioRef.current?.pause();
        const a = new Audio(r.url);
        audioRef.current = a;
        a.play().catch(() => {});
        a.onended = () => setPlaying(false);
        a.onerror = () => playAt(index + 1, tries - 1);
        setCurrent(song);
        setPlaying(true);
      })
      .catch(() => playAt(index + 1, tries - 1));
  };

  // play 切换歌曲（从歌单面板点击）。
  const play = (song: QqBgmSong) => playAt(songs.indexOf(song), 5);

  // toggle 暂停/播放（未播放时随机播一首）。
  const toggle = () => {
    const a = audioRef.current;
    if (a && !a.paused) {
      a.pause();
      setPlaying(false);
    } else if (a && current) {
      a.play().catch(() => {});
      setPlaying(true);
    } else {
      playAt(Math.floor(Math.random() * songs.length), 5);
    }
  };

  // Portal 挂到 body：避免页面 Reveal 动画祖先的 transform 劫持 fixed 定位
  return createPortal(
    <div
      className="fixed bottom-6 right-6 z-50 flex flex-col items-end"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
    >
      {/* 歌单面板：hover 展开，点击歌曲切换 */}
      {open && (
        <div className="mb-3 max-h-72 w-64 overflow-y-auto rounded-xl border border-line/70 bg-elevated/95 p-2 shadow-2xl shadow-black/30 backdrop-blur-md">
          <p className="px-2 pb-1.5 text-xs font-medium text-ink-2">
            首页背景音乐 · {current ? current.name : "点击歌曲切换"}
          </p>
          {songs.map((s) => (
            <button
              key={s.song_mid}
              type="button"
              onClick={() => play(s)}
              className={
                "flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-xs transition-colors " +
                (current && current.song_mid === s.song_mid
                  ? "bg-accent-soft text-glow"
                  : "text-ink hover:bg-line/60")
              }
            >
              <span className="min-w-0 flex-1 truncate">
                {s.name} - {s.artist}
              </span>
              {current && current.song_mid === s.song_mid && playing && (
                <span className="flex gap-0.5">
                  <i className="h-2 w-0.5 animate-pulse bg-accent" />
                  <i className="h-2 w-0.5 animate-pulse bg-accent [animation-delay:150ms]" />
                  <i className="h-2 w-0.5 animate-pulse bg-accent [animation-delay:300ms]" />
                </span>
              )}
            </button>
          ))}
        </div>
      )}

      {/* 悬浮按钮：渐变圆形 + 光晕；播放中显示旋转光环 */}
      <button
        type="button"
        onClick={toggle}
        aria-label={playing ? "暂停背景音乐" : "播放背景音乐"}
        className="relative flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-[#31c27c] to-[#0d9488] text-white shadow-lg shadow-emerald-500/40 ring-2 ring-white/25 transition-transform hover:scale-105 active:scale-95"
      >
        {playing && (
          <span className="absolute inset-0 animate-spin rounded-full border-2 border-dashed border-white/40 [animation-duration:4s]" />
        )}
        {playing ? (
          <svg viewBox="0 0 24 24" className="h-6 w-6" fill="currentColor">
            <rect x="6" y="5" width="4" height="14" rx="1.2" />
            <rect x="14" y="5" width="4" height="14" rx="1.2" />
          </svg>
        ) : (
          <svg viewBox="0 0 24 24" className="ml-0.5 h-6 w-6" fill="currentColor">
            <path d="M8 5.5v13a1 1 0 0 0 1.5.87l11-6.5a1 1 0 0 0 0-1.74l-11-6.5A1 1 0 0 0 8 5.5z" />
          </svg>
        )}
      </button>
    </div>,
    document.body,
  );
}

