// src/components/admin/image-crop-dialog.tsx
// 矩形图片裁剪弹层（M5 插件设置图片字段：默认 OG 图等）：
//   选图后弹出，16:9 矩形裁剪视口（OG 分享标准比例），可拖动定位 + 滚轮/滑杆缩放，
//   确认后按视口区域裁剪并输出 1200×630（OG 标准尺寸）回调 onConfirm。
// 复用头像裁剪器的交互模式（零新依赖，纯 React + canvas + pointer 事件）。
"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { useDismiss } from "@/components/motion/use-dismiss";
import { cropImageRect } from "@/lib/avatar";

// VIEWPORT_W/VIEWPORT_H 裁剪视口尺寸（CSS px，16:9 比例）。
const VIEWPORT_W = 400;
const VIEWPORT_H = 225;
// MAX_ZOOM 相对最小缩放（刚好覆盖视口）的最大倍数。
const MAX_ZOOM = 3;
// OUTPUT_W/OUTPUT_H 输出尺寸（OG 分享标准 1200×630）。
const OUTPUT_W = 1200;
const OUTPUT_H = 630;
// 缩放步进（滑杆每格约 1% 范围）。
const ZOOM_STEP_FACTOR = 0.01;

// ImageCropDialogProps 裁剪器参数。
interface ImageCropDialogProps {
  file: File; // 待裁剪原图
  onCancel: () => void; // 取消回调
  onConfirm: (croppedFile: File) => void; // 确认回调（产出裁剪压缩后的文件）
}

// ViewState 图片变换状态（translate + scale，transform-origin 为图片左上角）。
interface ViewState {
  scale: number; // 缩放倍数
  x: number; // 水平位移（px）
  y: number; // 垂直位移（px）
}

// ImageCropDialog 矩形图片裁剪器（16:9 → 1200×630）。
export function ImageCropDialog({ file, onCancel, onConfirm }: ImageCropDialogProps) {
  // 关闭过渡（先播离场动画，再回调父级卸载）
  const { closing, close } = useDismiss(onCancel, 180);
  const [objectUrl, setObjectUrl] = useState<string>("");
  const [natW, setNatW] = useState<number>(0); // 原图自然宽
  const [natH, setNatH] = useState<number>(0); // 原图自然高
  const [view, setView] = useState<ViewState>({ scale: 1, x: 0, y: 0 });
  const [busy, setBusy] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const viewportRef = useRef<HTMLDivElement>(null);
  // 拖拽起点（指针坐标 + 起始位移，用于计算增量）
  const dragRef = useRef<{ startX: number; startY: number; baseX: number; baseY: number } | null>(null);

  // 缩放边界：最小缩放 = 刚好让短边覆盖视口；最大 = 最小 × MAX_ZOOM
  const minScale = natW > 0 && natH > 0 ? VIEWPORT_H / Math.min(natW, natH) : 1;
  const maxScale = minScale * MAX_ZOOM;

  // clampScale 将缩放限制在边界内。
  const clampScale = useCallback(
    (value: number): number => Math.min(maxScale, Math.max(minScale, value)),
    [minScale, maxScale],
  );

  // clampX/clampY 位移约束：保证缩放后图片始终完整覆盖视口（不露空白）。
  const clampX = useCallback(
    (scale: number, x: number): number => Math.min(0, Math.max(VIEWPORT_W - natW * scale, x)),
    [natW],
  );
  const clampY = useCallback(
    (scale: number, y: number): number => Math.min(0, Math.max(VIEWPORT_H - natH * scale, y)),
    [natH],
  );

  // 生成 objectURL（自然尺寸改由渲染层隐藏 <img> 的 onLoad 读取，避免 StrictMode
  // 下独立 new Image() 捕获的 blob URL 被提前 revoke 而误报「加载失败」）。
  useEffect(() => {
    const url = URL.createObjectURL(file);
    setObjectUrl(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  // handleImgLoad 隐藏图片加载完成：读取自然尺寸并清除可能残留的错误。
  const handleImgLoad = useCallback((e: React.SyntheticEvent<HTMLImageElement>) => {
    setNatW(e.currentTarget.naturalWidth);
    setNatH(e.currentTarget.naturalHeight);
    setError("");
  }, []);

  // handleImgError 图片确实加载失败（如损坏文件）时提示。
  const handleImgError = useCallback(() => {
    setError("图片加载失败，请重新选择");
  }, []);

  // 尺寸就绪后：初始化为「刚好覆盖视口 + 居中」
  useEffect(() => {
    if (natW <= 0 || natH <= 0) {
      return;
    }
    const scale = VIEWPORT_H / Math.min(natW, natH);
    setView({
      scale,
      x: -(natW * scale - VIEWPORT_W) / 2,
      y: -(natH * scale - VIEWPORT_H) / 2,
    });
  }, [natW, natH]);

  // zoomBy 以视口中心为锚点相对缩放（滚轮用；setView 函数式更新避免闭包旧值）。
  const zoomBy = useCallback(
    (factor: number) => {
      setView((prev) => {
        const next = clampScale(prev.scale * factor);
        const cx = (VIEWPORT_W / 2 - prev.x) / prev.scale;
        const cy = (VIEWPORT_H / 2 - prev.y) / prev.scale;
        return {
          scale: next,
          x: clampX(next, VIEWPORT_W / 2 - cx * next),
          y: clampY(next, VIEWPORT_H / 2 - cy * next),
        };
      });
    },
    [clampScale, clampX, clampY],
  );

  // setScaleCentered 以视口中心为锚点设置绝对缩放（滑杆用）。
  const setScaleCentered = useCallback(
    (target: number) => {
      setView((prev) => {
        const next = clampScale(target);
        const cx = (VIEWPORT_W / 2 - prev.x) / prev.scale;
        const cy = (VIEWPORT_H / 2 - prev.y) / prev.scale;
        return {
          scale: next,
          x: clampX(next, VIEWPORT_W / 2 - cx * next),
          y: clampY(next, VIEWPORT_H / 2 - cy * next),
        };
      });
    },
    [clampScale, clampX, clampY],
  );

  // 滚轮缩放（原生非被动监听，阻止页面滚动）
  const zoomByRef = useRef(zoomBy);
  zoomByRef.current = zoomBy;
  useEffect(() => {
    const el = viewportRef.current;
    if (!el) {
      return;
    }
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      zoomByRef.current(e.deltaY < 0 ? 1.12 : 1 / 1.12);
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  // 拖拽：按下记录起点，移动更新位移（clamp 到边界）
  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      e.currentTarget.setPointerCapture(e.pointerId);
      dragRef.current = { startX: e.clientX, startY: e.clientY, baseX: view.x, baseY: view.y };
    },
    [view.x, view.y],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      if (!drag) {
        return;
      }
      const dx = e.clientX - drag.startX;
      const dy = e.clientY - drag.startY;
      setView((prev) => ({
        ...prev,
        x: clampX(prev.scale, drag.baseX + dx),
        y: clampY(prev.scale, drag.baseY + dy),
      }));
    },
    [clampX, clampY],
  );

  const onPointerUp = useCallback(() => {
    dragRef.current = null;
  }, []);

  // 确认裁剪：按当前视口坐标裁剪为 1200×630，回调父级上传
  const handleConfirm = async () => {
    if (busy || natW <= 0 || natH <= 0) {
      return;
    }
    setBusy(true);
    setError("");
    try {
      const cropped = await cropImageRect(file, {
        scale: view.scale,
        offsetX: view.x,
        offsetY: view.y,
        viewportWidth: VIEWPORT_W,
        viewportHeight: VIEWPORT_H,
        outputWidth: OUTPUT_W,
        outputHeight: OUTPUT_H,
      });
      onConfirm(cropped);
    } catch (err) {
      setError(err instanceof Error ? err.message : "裁剪失败，请重试");
      setBusy(false);
    }
  };

  return (
    <div
      className={`fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 ${
        closing ? "animate-fade-out" : "animate-fade-in"
      }`}
      role="dialog"
      aria-modal="true"
      aria-label="裁剪图片"
    >
      <div
        className={`w-full max-w-[520px] rounded-2xl border border-line bg-elevated p-5 ${
          closing ? "animate-scale-out" : "animate-scale-in"
        }`}
      >
        <h2 className="font-display text-lg font-semibold text-ink">裁剪图片</h2>
        <p className="mt-1 text-xs text-ink-3">拖动图片定位，滚动或拖动滑杆缩放 · 输出 1200×630</p>

        {/* 16:9 矩形遮罩裁剪视口 */}
        <div className="mt-4 flex justify-center">
          {natW <= 0 ? (
            // 加载骨架（尺寸就绪前占位）+ 隐藏图片读取自然尺寸
            <div className="relative">
              <div
                className="skeleton-shimmer rounded-lg"
                style={{ width: VIEWPORT_W, height: VIEWPORT_H }}
                aria-hidden
              />
              {objectUrl && (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={objectUrl}
                  alt=""
                  className="hidden"
                  onLoad={handleImgLoad}
                  onError={handleImgError}
                />
              )}
            </div>
          ) : (
            <div
              ref={viewportRef}
              className="relative cursor-grab overflow-hidden active:cursor-grabbing"
              style={{ width: VIEWPORT_W, height: VIEWPORT_H, touchAction: "none" }}
              onPointerDown={onPointerDown}
              onPointerMove={onPointerMove}
              onPointerUp={onPointerUp}
              onPointerCancel={onPointerUp}
            >
              {/* 图片层（transform-origin 左上角，translate + scale） */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={objectUrl}
                alt=""
                draggable={false}
                className="absolute left-0 top-0 max-w-none select-none"
                style={{
                  width: natW,
                  height: natH,
                  transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
                  transformOrigin: "0 0",
                }}
              />
              {/* 16:9 裁剪框描边（视口即裁剪框）+ 三分参考线（辅助构图） */}
              <div
                className="pointer-events-none absolute rounded-md border-2 border-white/80"
                style={{ left: 1, top: 1, width: VIEWPORT_W - 2, height: VIEWPORT_H - 2 }}
              />
              <div className="pointer-events-none absolute inset-0 opacity-40">
                <div className="absolute left-1/3 top-0 h-full w-px bg-white/60" />
                <div className="absolute left-2/3 top-0 h-full w-px bg-white/60" />
                <div className="absolute left-0 top-1/3 h-px w-full bg-white/60" />
                <div className="absolute left-0 top-2/3 h-px w-full bg-white/60" />
              </div>
            </div>
          )}
        </div>

        {/* 缩放滑杆 */}
        <div className="mt-4 flex items-center gap-3">
          <span className="shrink-0 text-xs text-ink-3">缩放</span>
          <input
            type="range"
            min={minScale}
            max={maxScale}
            step={Math.max((maxScale - minScale) * ZOOM_STEP_FACTOR, 0.001)}
            value={view.scale}
            onChange={(e) => setScaleCentered(Number(e.target.value))}
            disabled={natW <= 0}
            className="flex-1 accent-[var(--yy-accent)]"
            aria-label="图片缩放"
          />
        </div>

        {error && (
          <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-xs text-like" role="alert">
            {error}
          </p>
        )}

        {/* 操作 */}
        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={close}
            disabled={busy}
            className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 transition-colors hover:text-ink disabled:opacity-60"
          >
            取消
          </button>
          <button
            type="button"
            onClick={() => void handleConfirm()}
            disabled={busy || natW <= 0}
            className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {busy ? "裁剪中…" : "确认"}
          </button>
        </div>
      </div>
    </div>
  );
}
