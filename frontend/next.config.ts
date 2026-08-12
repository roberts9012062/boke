// next.config.ts
// Next.js 配置（ESM 模块系统，遵循 AGENTS.md 规则）。
//
// 说明：
//   - 开发联调：将 /api 请求代理到后端服务（localhost:8080），
//     前端代码统一请求相对路径 /api/v1/...（开发流程文档第 8 章约定）。
//   - M3.6：/plugin-assets 插件前端资源同样代理到后端（运行时动态加载）。
//   - 插件后置：CSP 安全头——script-src 含 'unsafe-inline'（Next 15 内联 RSC 脚本）
//     与 blob:（插件 ESM 模块 Blob URL 动态 import）；frame-src 'self'（插件 iframe 沙箱同源）。
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 开发代理：/api、/media 与 /plugin-assets → 后端 Gin 服务
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
      {
        source: "/media/:path*",
        destination: "http://localhost:8080/media/:path*",
      },
      {
        source: "/plugin-assets/:path*",
        destination: "http://localhost:8080/plugin-assets/:path*",
      },
    ];
  },
  // 全站安全响应头（CSP：限制脚本/样式/媒体来源，抵御 XSS 与注入）
  // 说明：dev 模式追加 'unsafe-eval'（Next webpack devtool source map 使用 eval）；
  //       生产构建严格（插件 ESM 经原生 import 执行，无需 eval）。
  async headers() {
    const scriptSrc = ["'self'", "'unsafe-inline'", "blob:"];
    if (process.env.NODE_ENV === "development") {
      scriptSrc.push("'unsafe-eval'");
    }
    return [
      {
        source: "/(.*)",
        headers: [
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              `script-src ${scriptSrc.join(" ")}`,
              "style-src 'self' 'unsafe-inline'",
              "img-src 'self' data: blob:",
              "media-src 'self'",
              "font-src 'self' data:",
              "frame-src 'self'",
              "connect-src 'self' ws://localhost:3000",
              "object-src 'none'",
              "base-uri 'self'",
              "form-action 'self'",
            ].join("; "),
          },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "SAMEORIGIN" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        ],
      },
    ];
  },
};

export default nextConfig;
