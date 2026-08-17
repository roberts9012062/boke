// src/app/admin/pages/[id]/edit/page.tsx
// 自定义页面编辑页（壳）：/admin/pages/new 新建；/admin/pages/{id}/edit 编辑。
// 表单逻辑在 components/admin/page-edit-form.tsx（文件行数硬性指标拆分）。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { PageEditForm } from "@/components/admin/page-edit-form";
import { ApiError } from "@/lib/api";
import { apiAdminPage } from "@/lib/api-pages";
import type { CustomPageDetail } from "@/lib/api-pages";

// AdminPageEditPage 自定义页面编辑页。
export default function AdminPageEditPage() {
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

  // 加载中骨架（编辑场景）
  if (!loaded) {
    return (
      <div className="space-y-4">
        <div className="h-10 animate-pulse rounded-lg bg-muted" aria-hidden />
        <div className="h-[420px] animate-pulse rounded-lg bg-muted" aria-hidden />
      </div>
    );
  }

  return <PageEditForm pageId={pageId} initial={initial} />;
}
