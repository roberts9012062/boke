// next.config.ts
// Next.js 配置（ESM 模块系统，遵循 AGENTS.md 规则）。
//
// 说明：
//   - 开发联调：将 /api 请求代理到后端服务（localhost:8080），
//     前端代码统一请求相对路径 /api/v1/...（开发流程文档第 8 章约定）。
//   - M3.6：/plugin-assets 插件前端资源同样代理到后端（运行时动态加载）。
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 开发代理：/api 与 /plugin-assets → 后端 Gin 服务
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
      {
        source: "/plugin-assets/:path*",
        destination: "http://localhost:8080/plugin-assets/:path*",
      },
    ];
  },
};

export default nextConfig;
