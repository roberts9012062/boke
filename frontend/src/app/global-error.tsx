// src/app/global-error.tsx
// 500 错误页（设计稿《500》画板）：
// 「月光暂时熄了」+ 重试（客户端错误边界，Next App Router 约定）。
// 说明：根级错误边界独立渲染（不含根布局 CSS），配色用内联样式兜底。
"use client";

// ErrorBoundary 全局错误边界：渲染 500 状态页。
// 参数：reset 由 Next 注入，点击重试时重置错误边界。
export default function GlobalError({ reset }: { reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body
        style={{
          minHeight: "100vh",
          margin: 0,
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: "#0d1220",
          color: "#e8ecf4",
          fontFamily: "system-ui, sans-serif",
          textAlign: "center",
          padding: "0 2rem",
        }}
      >
        <p style={{ fontSize: "3.5rem", fontWeight: 700, margin: 0, color: "#5b6478" }}>500</p>
        <p style={{ fontSize: "1.25rem", fontWeight: 600, margin: "1rem 0 0" }}>
          月光暂时熄了
        </p>
        <p style={{ fontSize: "0.875rem", color: "#9aa3b8", margin: "0.5rem 0 0" }}>
          服务器出了点小差。我们已在修复，请稍后再试。
        </p>
        <div style={{ marginTop: "2rem", display: "flex", gap: "0.75rem" }}>
          <a
            href="/"
            style={{
              borderRadius: 999,
              backgroundColor: "#7aa2f7",
              color: "#0d1220",
              padding: "0.65rem 1.5rem",
              fontSize: "0.875rem",
              textDecoration: "none",
              fontWeight: 500,
            }}
          >
            返回首页
          </a>
          <button
            type="button"
            onClick={reset}
            style={{
              borderRadius: 999,
              border: "1px solid #2a3348",
              background: "transparent",
              color: "#9aa3b8",
              padding: "0.65rem 1.5rem",
              fontSize: "0.875rem",
              cursor: "pointer",
            }}
          >
            重试
          </button>
        </div>
      </body>
    </html>
  );
}
