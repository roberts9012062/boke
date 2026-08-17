// src/components/admin/page-builder/preview-pane.tsx
// AI 页面构建器左侧预览：iframe srcDoc 沙箱渲染（sandbox="allow-scripts" 隔离源，
// AI 代码可执行但无法访问站点 cookie/localStorage/DOM）+ postMessage 高度自适应。
"use client";

import { useEffect, useRef, useState } from "react";

import { injectHeightReport } from "@/lib/page-html";

// PreviewPaneProps 预览面板参数。
interface PreviewPaneProps {
  html: string; // AI 生成的完整 HTML 文档（空 = 未生成，显示引导占位）
  generating: boolean; // AI 是否正在生成（显示遮罩）
}

// PreviewPane 页面实时预览。
export function PreviewPane({ html, generating }: PreviewPaneProps) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState<number>(600);

  // 监听沙箱 iframe 的高度上报（校验来源为当前 iframe，忽略外部消息）
  useEffect(() => {
    const onMessage = (e: MessageEvent): void => {
      const data = e.data as { type?: string; height?: number } | null;
      if (
        data &&
        data.type === "yy-page-height" &&
        typeof data.height === "number" &&
        e.source === iframeRef.current?.contentWindow
      ) {
        setHeight(Math.min(Math.max(data.height, 320), 20000));
      }
    };
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  // 未生成：引导占位
  if (!html) {
    return (
      <div className="flex flex-1 items-center justify-center rounded-lg border border-dashed border-line bg-muted/40 p-10 text-center">
        <div>
          <p className="text-sm text-ink-2">还没有页面内容</p>
          <p className="mt-1 text-xs text-ink-3">
            在右侧告诉 AI 你想要什么页面（如「做一个友情链接页」），生成后会在这里实时预览
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="relative flex-1 overflow-hidden rounded-lg border border-line bg-elevated">
      {/* 生成中遮罩（不重载 iframe，避免流式期间闪烁） */}
      {generating && (
        <div className="absolute inset-x-0 top-0 z-10 bg-accent-soft/90 px-3 py-1.5 text-center text-xs text-glow">
          AI 正在生成新版本，完成后自动更新预览…
        </div>
      )}
      {/* 沙箱渲染：allow-scripts 允许脚本执行（隔离源）；高度随内容自适应 */}
      <iframe
        ref={iframeRef}
        title="页面预览"
        srcDoc={injectHeightReport(html)}
        sandbox="allow-scripts"
        className="w-full border-0 bg-bg"
        style={{ height: `${height}px` }}
      />
    </div>
  );
}
