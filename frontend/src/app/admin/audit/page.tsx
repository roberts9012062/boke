// src/app/admin/audit/page.tsx
// 后台审核队列（设计稿《审核队列》画板）：
// 待审帖子、评论与举报统一处理 → 统计条（待处理/高风险/今日已审/平均耗时）
// → 表格（内容摘要/类型/来源/风险/提交/操作）。
// MVP 实现：举报工单队列（帖子/评论/用户），处理 = 解决/驳回。
"use client";

import { useEffect, useState } from "react";

import {
  apiAdminReportStats,
  apiAdminReports,
  apiAdminSetReportStatus,
  type ReportDTO,
} from "@/lib/api";
import { formatDuration } from "@/lib/utils";

// 状态文案（附录 B 状态字典）
const STATUS_LABEL: Record<string, string> = {
  pending: "待处理",
  processing: "处理中",
  resolved: "已解决",
  rejected: "已驳回",
};

// 类型文案（设计稿：帖子/评论/用户）
const TYPE_LABEL: Record<string, string> = {
  post: "帖子",
  comment: "评论",
  user: "用户",
};

// AuditPage 审核队列页（举报工单管理）。
export default function AuditPage() {
  const [items, setItems] = useState<ReportDTO[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [stats, setStats] = useState<{ pending: number; resolved_today: number; avg_cost_seconds: number } | null>(null);
  const [filter, setFilter] = useState<string>(""); // 空=全部；pending/resolved/rejected
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载统计（待处理全量 + 今日已审）
  useEffect(() => {
    apiAdminReportStats()
      .then(setStats)
      .catch(() => setStats(null));
  }, []);

  // 加载工单
  useEffect(() => {
    let cancelled = false;
    setLoaded(false);
    apiAdminReports({ status: filter, page: 1 })
      .then((r) => {
        if (cancelled) return;
        setItems(r.items);
        setTotal(r.total);
      })
      .catch(() => {
        if (!cancelled) setItems([]);
      })
      .finally(() => {
        if (!cancelled) setLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [filter]);

  // 处理工单（解决/驳回）
  const handleStatus = async (report: ReportDTO, status: string) => {
    await apiAdminSetReportStatus(report.id, status);
    // 本地更新状态
    setItems((prev) => prev.map((x) => (x.id === report.id ? { ...x, status } : x)));
  };

  return (
    <div>
      {/* 标题（设计稿：审核队列 / 待审帖子、评论与举报统一处理） */}
      <div>
        <h1 className="font-display text-xl font-semibold text-ink">审核队列</h1>
        <p className="mt-0.5 text-xs text-ink-3">待审帖子、评论与举报统一处理</p>
      </div>

      {/* 统计条（设计稿：待处理 18 / 高风险 3 / 今日已审 47 / 平均耗时 4m；
          MVP 实现待处理/今日已审/工单总数/平均耗时（P1 处理时长埋点）；
          高风险需 AI 判定（M4）） */}
      <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          { label: "待处理", value: stats?.pending ?? 0 },
          { label: "今日已审", value: stats?.resolved_today ?? 0 },
          { label: "工单总数", value: total },
          { label: "平均耗时", value: formatDuration(stats?.avg_cost_seconds ?? 0) },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">{s.label}</p>
            <p className="mt-1 font-display text-2xl font-semibold text-ink">{s.value}</p>
          </div>
        ))}
      </div>

      {/* 状态筛选 Tab */}
      <div className="mt-4 flex gap-2">
        {[
          { key: "", label: "全部" },
          { key: "pending", label: "待处理" },
          { key: "resolved", label: "已解决" },
          { key: "rejected", label: "已驳回" },
        ].map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setFilter(t.key)}
            className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
              filter === t.key
                ? "bg-accent-soft font-medium text-glow"
                : "bg-muted text-ink-2 hover:text-ink"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* 工单表格（设计稿：内容摘要/类型/来源/风险/提交/操作） */}
      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && items.length === 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无举报工单</p>
        </div>
      )}
      {loaded && items.length > 0 && (
        <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead>
              <tr className="border-b border-line text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">内容摘要</th>
                <th className="px-4 py-3 font-normal">类型</th>
                <th className="px-4 py-3 font-normal">来源</th>
                <th className="px-4 py-3 font-normal">原因</th>
                <th className="px-4 py-3 font-normal">提交</th>
                <th className="px-4 py-3 font-normal">状态</th>
                <th className="px-4 py-3 font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.map((report) => (
                <tr key={report.id} className="hover:bg-muted/40">
                  <td className="max-w-[220px] px-4 py-3">
                    <p className="line-clamp-1 text-ink">{report.target_brief || "（内容已删除）"}</p>
                    {report.detail && (
                      <p className="mt-0.5 line-clamp-1 text-xs text-ink-3">补充：{report.detail}</p>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-ink-2">
                      {TYPE_LABEL[report.target_type] ?? report.target_type}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-ink-2">{report.reporter || "匿名"}</td>
                  <td className="px-4 py-3 text-ink-2">{report.reason}</td>
                  <td className="px-4 py-3 text-xs text-ink-3">
                    {new Date(report.created_at).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${
                        report.status === "pending"
                          ? "bg-like/15 text-like"
                          : report.status === "resolved"
                            ? "bg-accent-soft text-glow"
                            : "bg-muted text-ink-3"
                      }`}
                    >
                      {STATUS_LABEL[report.status] ?? report.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    {report.status === "pending" ? (
                      <div className="flex gap-2">
                        <button
                          type="button"
                          onClick={() => void handleStatus(report, "resolved")}
                          className="rounded-full bg-accent px-3 py-1 text-xs text-on-accent hover:opacity-90"
                        >
                          解决
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleStatus(report, "rejected")}
                          className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 hover:text-ink"
                        >
                          驳回
                        </button>
                      </div>
                    ) : (
                      <span className="text-xs text-ink-3">已处理</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
