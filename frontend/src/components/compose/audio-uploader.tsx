// src/components/compose/audio-uploader.tsx
// 发帖中心 · 音频上传区（需求 3.4「收声音」）：
// 浏览器录音（MediaRecorder，≤5 分钟）或上传音频文件（mp3/m4a/wav，≤20MB）。
"use client";

import { useEffect, useRef, useState } from "react";

import { apiUploadMedia } from "@/lib/api";
import type { MediaDTO } from "@/types/api";

// 录音时长上限（5 分钟）
const MAX_RECORD_SECONDS = 300;

// formatTime 秒数格式化为 m:ss。
function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, "0")}`;
}

// AudioUploader 音频上传区（录音 / 文件上传二选一）。
// 参数：value 已上传音频（单选）；onChange 变化回调。
export function AudioUploader({
  value,
  onChange,
}: {
  value: MediaDTO | null;
  onChange: (media: MediaDTO | null) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [recording, setRecording] = useState<boolean>(false);
  const [recordSeconds, setRecordSeconds] = useState<number>(0);
  const [uploading, setUploading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 清理录音定时器
  useEffect(() => {
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, []);

  // 开始录音（MediaRecorder，webm/opus 格式）
  const startRecord = async () => {
    setError("");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const recorder = new MediaRecorder(stream);
      recorderRef.current = recorder;
      chunksRef.current = [];

      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) {
          chunksRef.current.push(e.data);
        }
      };
      // 录音结束：停止轨道并上传
      recorder.onstop = () => {
        stream.getTracks().forEach((t) => t.stop());
        const blob = new Blob(chunksRef.current, { type: recorder.mimeType });
        void uploadBlob(blob);
      };

      recorder.start();
      setRecording(true);
      setRecordSeconds(0);
      // 计时（≤5 分钟自动停止）
      timerRef.current = setInterval(() => {
        setRecordSeconds((s) => {
          if (s + 1 >= MAX_RECORD_SECONDS) {
            stopRecord();
            return s;
          }
          return s + 1;
        });
      }, 1000);
    } catch {
      setError("无法访问麦克风，请检查浏览器权限或改用文件上传");
    }
  };

  // 停止录音
  const stopRecord = () => {
    if (recorderRef.current && recorderRef.current.state !== "inactive") {
      recorderRef.current.stop();
    }
    setRecording(false);
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
  };

  // 上传音频 blob（录音结果）
  const uploadBlob = async (blob: Blob) => {
    setUploading(true);
    try {
      // 文件名含扩展名以通过后端类型校验（webm → 转 m4a 扩展名由后端 mime 判定；此处统一 mp3 命名）
      const file = new File([blob], "recording.mp3", { type: blob.type });
      const result = await apiUploadMedia(file);
      onChange({
        id: result.id,
        type: result.type,
        url: result.url,
        mime_type: result.mime_type,
        size_bytes: result.size_bytes,
        width: 0,
        height: 0,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "录音上传失败");
    } finally {
      setUploading(false);
    }
  };

  // 选择音频文件
  const handleFile = async (file: File | undefined) => {
    if (!file) {
      return;
    }
    setError("");
    setUploading(true);
    try {
      const result = await apiUploadMedia(file);
      onChange({
        id: result.id,
        type: result.type,
        url: result.url,
        mime_type: result.mime_type,
        size_bytes: result.size_bytes,
        width: 0,
        height: 0,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "音频上传失败");
    } finally {
      setUploading(false);
    }
  };

  // 已上传：显示播放条 + 替换/删除
  if (value) {
    return (
      <div className="rounded-lg border border-line bg-muted/40 p-4">
        <audio src={value.url} controls className="w-full" />
        <div className="mt-2 flex gap-2 text-xs">
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            className="rounded-full border border-line px-3 py-1 text-ink-2 hover:text-ink"
          >
            替换
          </button>
          <button
            type="button"
            onClick={() => onChange(null)}
            className="rounded-full border border-line px-3 py-1 text-like hover:opacity-80"
          >
            删除
          </button>
        </div>
        <input
          ref={inputRef}
          type="file"
          accept="audio/*"
          className="hidden"
          onChange={(e) => void handleFile(e.target.files?.[0])}
        />
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-dashed border-line p-6 text-center">
      {/* 录音状态区 */}
      {recording ? (
        <div>
          <p className="font-display text-lg text-like">● 录音中 {formatTime(recordSeconds)}</p>
          <p className="mt-1 text-xs text-ink-3">最长 5 分钟，点击停止后自动上传</p>
          <button
            type="button"
            onClick={stopRecord}
            className="mt-4 rounded-full bg-accent px-6 py-2 text-sm text-on-accent"
          >
            停止录音
          </button>
        </div>
      ) : (
        <div>
          <p className="text-sm text-ink-2">收一段声音（mp3/m4a/wav，≤20MB）</p>
          <div className="mt-4 flex justify-center gap-3">
            <button
              type="button"
              onClick={() => void startRecord()}
              disabled={uploading}
              className="rounded-full bg-accent px-6 py-2 text-sm text-on-accent disabled:opacity-60"
            >
              {uploading ? "上传中…" : "开始录音"}
            </button>
            <button
              type="button"
              onClick={() => inputRef.current?.click()}
              disabled={uploading}
              className="rounded-full border border-line px-6 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
            >
              上传文件
            </button>
          </div>
          <input
            ref={inputRef}
            type="file"
            accept=".mp3,.m4a,.wav,audio/*"
            className="hidden"
            onChange={(e) => void handleFile(e.target.files?.[0])}
          />
        </div>
      )}
      {error && <p className="mt-3 text-xs text-like">{error}</p>}
    </div>
  );
}
