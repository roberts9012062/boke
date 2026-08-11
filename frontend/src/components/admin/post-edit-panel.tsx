// src/components/admin/post-edit-panel.tsx
// 后台编辑帖子 · 右栏信息面板（设计稿《后台编辑》四画板右侧）：
// 发布信息（类型/状态/可见性/创建/更新）+ 互动数据（赞/评/览）+ 操作（下架/删除）+ SEO 占位。
"use client";

import { apiAdminDeletePost, apiAdminSetPostStatus } from "@/lib/api";
import { formatCompact, formatDateTime } from "@/lib/utils";
import type { AdminPostDetail } from "@/types/api";

// 类型/状态/可见性文案映射（与后台列表页一致）
const TYPE_LABEL: Record<string, string> = { text: "文字", image: "图片", audio: "音频", video: "视频" };
const STATUS_LABEL: Record<string, string> = {
  published: "已发布",
  draft: "草稿",
  taken_down: "已下架",
};
const VISIBILITY_LABEL: Record<string, string> = {
  public: "公开",
  followers: "仅关注者",
  private: "仅自己",
};

// PostEditPanel 右栏信息面板。
// 参数：detail 帖子详情；onChanged 操作完成回调（刷新页面数据）。
export function PostEditPanel({
  detail,
  onChanged,
}: {
  detail: AdminPostDetail;
  onChanged: () => void;
}) {
  // 下架/上架切换（复用内容管理上下架接口）
  const toggleStatus = async () => {
    const next = detail.status === "published" ? "taken_down" : "published";
    await apiAdminSetPostStatus(detail.id, next);
    onChanged();
  };

  // 删除帖子（二次确认）
  const handleDelete = async () => {
    if (!window.confirm("确定删除该帖子？删除后不可恢复")) {
      return;
    }
    await apiAdminDeletePost(detail.id);
    // 删除后跳回内容管理列表
    window.location.href = "/admin/posts";
  };

  return (
    <div className="space-y-5">
      {/* 发布信息（设计稿：类型/状态/可见性/创建/更新） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">发布信息</h2>
        <dl className="mt-3 space-y-2 text-xs">
          <div className="flex justify-between">
            <dt className="text-ink-3">类型</dt>
            <dd className="text-ink">{TYPE_LABEL[detail.content_type] ?? detail.content_type}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">状态</dt>
            <dd className="text-ink">{STATUS_LABEL[detail.status] ?? detail.status}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">可见性</dt>
            <dd className="text-ink">{VISIBILITY_LABEL[detail.visibility] ?? detail.visibility}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">创建</dt>
            <dd className="text-ink">{formatDateTime(detail.created_at)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-ink-3">更新</dt>
            <dd className="text-ink">{formatDateTime(detail.updated_at)}</dd>
          </div>
        </dl>
      </section>

      {/* 互动数据（设计稿：128 赞 / 24 评 / 1.2k 览，千位缩写） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">互动数据</h2>
        <div className="mt-3 flex gap-6 text-center">
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.like_count)}</p>
            <p className="text-xs text-ink-3">赞</p>
          </div>
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.comment_count)}</p>
            <p className="text-xs text-ink-3">评</p>
          </div>
          <div className="flex-1">
            <p className="text-lg font-semibold text-ink">{formatCompact(detail.view_count)}</p>
            <p className="text-xs text-ink-3">览</p>
          </div>
        </div>
      </section>

      {/* 操作（设计稿：下架 / 删除帖子） */}
      <section className="rounded-lg border border-line bg-elevated p-5">
        <h2 className="text-sm font-semibold text-ink">操作</h2>
        <div className="mt-3 flex flex-col gap-2">
          <button
            type="button"
            onClick={() => void toggleStatus()}
            className="rounded-full border border-line px-4 py-2 text-sm text-ink-2 hover:text-ink"
          >
            {detail.status === "published" ? "下架" : "上架"}
          </button>
          <button
            type="button"
            onClick={() => void handleDelete()}
            className="rounded-full border border-like/30 px-4 py-2 text-sm text-like hover:bg-like/10"
          >
            删除帖子
          </button>
        </div>
      </section>

      {/* SEO 面板占位（设计稿《后台编辑·文字·SEO》：SEO 标题/SEO 描述/本帖 SEO/SEO 选项） */}
      <section className="rounded-lg border border-dashed border-line p-5">
        <h2 className="text-sm font-semibold text-ink-2">SEO 标题 / SEO 描述</h2>
        <p className="mt-1 text-xs text-ink-3">
          本帖 SEO（标题字/描述字/收录）· noindex / 自定义 OG 图 —— M4 里程碑开放
        </p>
      </section>
    </div>
  );
}
