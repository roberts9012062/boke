// next.config.ts
// Next.js 配置（ESM 模块系统，遵循 AGENTS.md 规则）。
//
// 说明：
//   - 开发联调：将 /api 请求代理到后端服务（localhost:8080），
//     前端代码统一请求相对路径 /api/v1/...（开发流程文档第 8 章约定）。
//   - M3.6：/plugin-assets 插件前端资源同样代理到后端（运行时动态加载）。
//   - 插件后置：CSP 安全头——script-src 含 'unsafe-inline'（Next 15 内联 RSC 脚本）
//     与 blob:（插件 ESM 模块 Blob URL 动态 import）。
//   - M5 富文本：frame-src 放开视频平台白名单（内嵌 bilibili/YouTube/腾讯/Vimeo 播放器）。
import type { NextConfig } from "next";

// 后端服务地址：开发默认 localhost:8080；容器/生产环境经 BACKEND_URL 注入
// （Docker 编排中为 http://backend:8080，rewrites 与 middleware 共用）。
const BACKEND_URL: string = process.env.BACKEND_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  // 容器部署：standalone 产物（.next/standalone 含精简 node_modules + server.js），
  // 本地 dev 不受影响
  output: "standalone",
  // rewrites 代理超时（默认 30s 会掐断 AI 长任务——润色/文章化推理可达数分钟，
  // 与后端 AI 客户端 300s 上限对齐）
  experimental: {
    proxyTimeout: 300_000,
  },
  // 代理：/api、/media、/plugin-assets 与 /plugin-sdk → 后端 Gin 服务
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${BACKEND_URL}/api/:path*`,
      },
      {
        source: "/media/:path*",
        destination: `${BACKEND_URL}/media/:path*`,
      },
      {
        source: "/plugin-assets/:path*",
        destination: `${BACKEND_URL}/plugin-assets/:path*`,
      },
      {
        // 插件前端共享 SDK（E2）：插件 ESM 模块以绝对路径 import /plugin-sdk/shared.js，
        // 模块图按同源 URL 解析——dev 下必须代理到 Gin（与生产同域反代行为一致）
        source: "/plugin-sdk/:path*",
        destination: `${BACKEND_URL}/plugin-sdk/:path*`,
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
              // img-src 放行 https 外链：正文插图来自任意图床/CDN（CF图床 imgs.* 等）；
              // 图片为无脚本执行的被动资源，风险低于 script/connect（M7/M8 音乐域已含于 https:）
              // 大世界（中继站聚合流）：测试环境的中继站可能无 TLS，经 ALLOW_HTTP_IMAGES
              // 构建开关追加 http: 外链（生产保持严格 https-only）。
              `img-src 'self' data: blob: ${process.env.ALLOW_HTTP_IMAGES === "true" ? "http: https:" : "https:"}`,
              // media-src 含 blob:：插件 MSE 播放器（B站 DASH 1080P）经 objectURL 装载媒体；
              // bilivideo 域群：B 站视频浏览器直连播放（mp4 durl，视频流量不经服务器中转）
              "media-src 'self' blob: https://*.music.126.net https://*.qq.com https://*.tc.qq.com https://*.bilivideo.com https://*.bilivideo.cn https://*.akamaized.net https://*.mcdn.bilivideo.cn",
              "font-src 'self' data:",
              "frame-src 'self' https://player.bilibili.com https://www.youtube.com https://www.youtube-nocookie.com https://v.qq.com https://player.vimeo.com https://music.163.com https://i.y.qq.com",
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
