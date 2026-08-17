// src/app/admin/pages/[id]/build/page.tsx
// AI 页面构建器（壳）：/admin/pages/new/build 新建；/admin/pages/{id}/build 编辑。
// 左预览 + 右 AI 对话 + 顶部路由自定义，逻辑在 components/admin/page-builder/。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { PageBuilder } from "@/components/admin/page-builder/page-builder";
import { ApiError } from "@/lib/api";
import { apiAdminPage } from "@/lib/api-pages";
import type { CustomPageDetail } from "@/lib/api-pages";

// AdminPageBuildPage AI 页面构建器页。
export default function AdminPageBuildPage() {
  // 路径参数：id 为 "new"（新建）或数字（编辑已有页面）
  const params = useParams<{ id: string }>();
  const isNew = params.id === "new";
  const pageId = isNew ? null : Number(params.id);

  const [initial, setInitial] = useState<CustomPageDetail | null>(null);
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 编辑场景：加载已有页面回显（新建无需请求）
  useEffect(() => {
    if (isNew || !pageId) {
      setLoaded(true);
      return;
    }
    apiAdminPage(pageId)
      .then(setInitial)
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoaded(true));
  }, [isNew, pageId]);

  if (error) {
    return (
      <div className="py-20 text-center">
        <p className="text-lg text-ink">{error}</p>
      </div>
    );
  }

  if (!loaded) {
    return (
      <div className="space-y-4">
        <div className="h-10 animate-pulse rounded-lg bg-muted" aria-hidden />
        <div className="h-[480px] animate-pulse rounded-lg bg-muted" aria-hidden />
      </div>
    );
  }

  return <PageBuilder pageId={pageId} initial={initial} />;
}
