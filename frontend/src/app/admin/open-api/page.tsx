// src/app/admin/open-api/page.tsx
// 接口开放页面壳（逻辑在 components/admin/open-api/open-api-manager.tsx，保持页面文件精简）。
"use client";

import { OpenApiManager } from "@/components/admin/open-api/open-api-manager";

// OpenApiPage 接口开放页面：外部接口开放管理（目录多选 → 生成 Key → Key 管理 / AI 开发手册）。
export default function OpenApiPage() {
  return <OpenApiManager />;
}
