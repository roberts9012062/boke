// src/app/admin/ai/page.tsx
// AI 设置页（M4-AI）：供应商 / 任务配置 / 用量统计 三个 Tab。
// 设计依据：设计稿无 AI 画板，参照《SEO设置》画板后台模式自行设计（差异记录见验收报告）。
"use client";

import { useState } from "react";

import { ProvidersTab } from "@/components/admin/ai/providers-tab";
import { TasksTab } from "@/components/admin/ai/tasks-tab";
import { UsageTab } from "@/components/admin/ai/usage-tab";

// Tab 定义（标签 + 说明）。
const TABS: readonly { key: string; label: string; desc: string }[] = [
  { key: "providers", label: "供应商", desc: "OpenAI 兼容接口配置（deepseek/qwen/kimi/glm/openai）" },
  { key: "tasks", label: "任务配置", desc: "内置场景（摘要/标签/评论审核）的模型与提示词" },
  { key: "usage", label: "用量统计", desc: "AI 调用次数与 Token 消耗" },
];

// AiPage AI 设置页。
export default function AiPage() {
  const [tab, setTab] = useState("providers");

  return (
    <div>
      {/* 标题 */}
      <div>
        <h1 className="font-display text-xl font-semibold text-ink">AI 设置</h1>
        <p className="mt-0.5 text-xs text-ink-3">OpenAI 兼容多供应商 · 内置场景（摘要 / 自动标签 / 评论审核）</p>
      </div>

      {/* Tab 切换 */}
      <div className="mt-4 flex gap-2">
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
              tab === t.key ? "bg-accent-soft font-medium text-glow" : "bg-muted text-ink-2 hover:text-ink"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* 当前 Tab 说明 */}
      <p className="mt-2 text-xs text-ink-3">{TABS.find((t) => t.key === tab)?.desc}</p>

      {/* Tab 内容 */}
      <div className="mt-4">
        {tab === "providers" && <ProvidersTab />}
        {tab === "tasks" && <TasksTab />}
        {tab === "usage" && <UsageTab />}
      </div>
    </div>
  );
}
