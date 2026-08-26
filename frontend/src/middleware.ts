// frontend/src/middleware.ts
// 全站安装引导中间件：未完成安装时将全部页面重定向到 /setup 安装向导。
//
// 说明：
//   - 安装状态经后端 /api/setup/status 查询（middleware 运行于 Node 侧，
//     需用 BACKEND_URL 绝对地址直连后端，不走浏览器 rewrites）
//   - 查询结果内存缓存 15 秒：安装期页面跳转频繁，避免每请求打后端；
//     后端不可达视为未安装（引导用户进向导，向导内会给出明确报错）
//   - 静态资源与后端代理路径不拦截（matcher 排除）

import { NextResponse, type NextRequest } from "next/server";

// 后端地址（Docker 编排注入；本地开发默认 8080）。
const BACKEND_URL: string = process.env.BACKEND_URL ?? "http://localhost:8080";

// 安装状态缓存（模块级，随 Next 服务进程存活）。
let cache: { installed: boolean; at: number } | null = null;

// CACHE_TTL 缓存有效期（毫秒）。
const CACHE_TTL = 15_000;

// fetchInstalled 查询安装状态（带内存缓存；后端不可达按未安装处理）。
async function fetchInstalled(): Promise<boolean> {
  if (cache !== null && Date.now() - cache.at < CACHE_TTL) {
    return cache.installed;
  }
  try {
    const res = await fetch(`${BACKEND_URL}/api/setup/status`, {
      cache: "no-store",
      signal: AbortSignal.timeout(3000),
    });
    const body: { code: number; data: { installed: boolean } } = await res.json();
    const installed = body.code === 0 && body.data.installed;
    cache = { installed, at: Date.now() };
    return installed;
  } catch {
    // 查询失败不缓存：下次请求重试（后端可能在启动中）
    return false;
  }
}

// middleware 请求拦截入口。
export async function middleware(request: NextRequest): Promise<NextResponse> {
  const { pathname } = request.nextUrl;
  // 安装向导页面本身不拦截（已安装时由页面自行跳回首页）
  if (pathname === "/setup") {
    return NextResponse.next();
  }
  const installed = await fetchInstalled();
  if (!installed) {
    return NextResponse.redirect(new URL("/setup", request.url));
  }
  return NextResponse.next();
}

// config 拦截范围：排除 Next 静态资源与后端代理路径（媒体/插件资产/开放接口）。
export const config = {
  matcher: [
    "/((?!_next/static|_next/image|favicon.ico|media|plugin-assets|plugin-sdk|api).*)",
  ],
};
