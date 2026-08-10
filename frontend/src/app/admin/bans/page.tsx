// src/app/admin/bans/page.tsx
// 后台封禁管理（设计稿《封禁管理》画板）：
// IP、设备与账号封禁 → 封禁记录列表（用户/原因/期限/操作者/时间）。
// 说明：封禁操作在「用户管理」页发起（原因 + 期限），本页为记录台账。
"use client";

import { useEffect, useState } from "react";

import { apiAdminBans, apiAdminUserStats, type BanRecordDTO } from "@/lib/api";

// BansPage 封禁记录页。
export default function BansPage() {
  const [items, setItems] = useState<BanRecordDTO[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [userStats, setUserStats] = useState<{ total: number; banned: number } | null>(null);
  const [loaded, setLoaded] = useState<boolean>(false);

  // 加载用户统计（设计稿统计条：全部用户/已禁言）
  useEffect(() => {
    apiAdminUserStats()
      .then(setUserStats)
      .catch(() => setUserStats(null));
  }, []);

  // 加载封禁记录
  useEffect(() => {
    let cancelled = false;
    setLoaded(false);
    apiAdminBans({ page: 1 })
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
  }, []);

  return (
    <div>
      {/* 标题（设计稿：封禁管理 / IP、设备与账号封禁） */}
      <div>
        <h1 className="font-display text-xl font-semibold text-ink">封禁管理</h1>
        <p className="mt-0.5 text-xs text-ink-3">IP、设备与账号封禁</p>
      </div>

      {/* 统计条（设计稿：全部用户 12,480 / 本周新增 / 活跃 / 已禁言；MVP 展示全量/已禁言） */}
      <div className="mt-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
        {[
          { label: "全部用户", value: userStats?.total ?? 0 },
          { label: "已禁言", value: userStats?.banned ?? 0 },
          { label: "封禁记录", value: total },
          { label: "当前页", value: items.length },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-line bg-elevated p-4">
            <p className="text-xs text-ink-3">{s.label}</p>
            <p className="mt-1 font-display text-2xl font-semibold text-ink">{s.value}</p>
          </div>
        ))}
      </div>

      {/* 封禁记录列表 */}
      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && items.length === 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无封禁记录</p>
          <p className="mt-1 text-xs text-ink-3">封禁操作在「用户管理」页发起</p>
        </div>
      )}
      {loaded && items.length > 0 && (
        <div className="mt-4 overflow-hidden rounded-lg border border-line bg-elevated">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">用户</th>
                <th className="px-4 py-3 font-normal">封禁原因</th>
                <th className="px-4 py-3 font-normal">期限</th>
                <th className="px-4 py-3 font-normal">封禁时间</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.map((record) => (
                <tr key={record.id} className="hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium text-ink">
                    {record.nickname || `用户 #${record.user_id}`}
                  </td>
                  <td className="px-4 py-3 text-ink-2">{record.reason}</td>
                  <td className="px-4 py-3">
                    <span className="rounded-full bg-like/15 px-2 py-0.5 text-xs text-like">
                      {record.until ? `至 ${new Date(record.until).toLocaleDateString("zh-CN")}` : "永久"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink-3">
                    {new Date(record.created_at).toLocaleString("zh-CN")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="border-t border-line px-4 py-2 text-xs text-ink-3">共 {total} 条封禁记录</p>
        </div>
      )}
    </div>
  );
}
