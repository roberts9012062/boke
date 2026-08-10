// next.config.ts
// Next.js 配置（ESM 模块系统，遵循 AGENTS.md 规则）。
//
// 说明：
//   - 开发联调：将 /api 请求代理到后端服务（localhost:8080），
//     前端代码统一请求相对路径 /api/v1/...（开发流程文档第 8 章约定）。
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // 开发代理：/api → 后端 Gin 服务
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/api/:path*",
      },
    ];
  },
};

export default nextConfig;
