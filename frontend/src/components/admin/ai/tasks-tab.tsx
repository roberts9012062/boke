// src/components/admin/ai/tasks-tab.tsx
// AI 设置页 · 任务配置 Tab（M4）：三个内置任务卡片（模型/最大 token/提示词/启用开关）。
// 设计依据：无 AI 画板，参照后台设置表单模式（SEO 设置页风格）自行设计。
"use client";

import { useEffect, useState } from "react";

import { apiAiProviders, apiAiTasks, apiAiToggleTask, apiAiUpdateTask, type AiProviderDTO, type AiTaskDTO } from "@/lib/api-ai";
import { ApiError } from "@/lib/api";
import { Switch } from "@/components/ui/switch";

// 任务说明（中文，后台展示用）。
const TASK_META: Record<string, { label: string; desc: string }> = {
  "post.summary": { label: "帖子摘要", desc: "生成帖子摘要，写入 SEO 元数据（seo_meta.summary），编辑页「AI 生成摘要」触发" },
  "post.tags": { label: "自动标签", desc: "根据标题与正文提炼 3-5 个标签建议，编辑页「AI 生成标签」触发，确认后写入" },
  "comment.review": { label: "评论审核", desc: "新评论异步预审，高风险自动隐藏并进入审核队列；后台评论管理可手动批量" },
};

// TasksTab 任务配置管理。
export function TasksTab() {
  const [items, setItems] = useState<AiTaskDTO[]>([]);
  const [providers, setProviders] = useState<AiProviderDTO[]>([]); // 供应商（绑定下拉选项）
  const [loaded, setLoaded] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");
  // 编辑态草稿：task_name → 表单值（仅保存时提交，未保存不生效）
  const [drafts, setDrafts] = useState<Record<string, { model: string; prompt: string; maxTokens: number; providerId: number | null }>>({});

  // 加载任务列表与供应商（绑定下拉）
  useEffect(() => {
    void apiAiProviders().then((r) => setProviders(r.items)).catch(() => setProviders([]));
    apiAiTasks()
      .then((r) => {
        setItems(r.items);
        // 初始化草稿（与后端同步）
        const init: Record<string, { model: string; prompt: string; maxTokens: number; providerId: number | null }> = {};
        for (const t of r.items) {
          init[t.task_name] = { model: t.model, prompt: t.prompt_template, maxTokens: t.max_tokens, providerId: t.provider_id };
        }
        setDrafts(init);
      })
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, []);

  // 保存单个任务配置
  const handleSave = async (t: AiTaskDTO) => {
    const d = drafts[t.task_name];
    if (!d) return;
    setError("");
    try {
      await apiAiUpdateTask(t.task_name, {
        provider_id: d.providerId,
        model: d.model,
        prompt_template: d.prompt,
        max_tokens: d.maxTokens,
      });
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    }
  };

  // 启停任务（即时生效）
  const handleToggle = async (t: AiTaskDTO, enabled: boolean) => {
    try {
      await apiAiToggleTask(t.task_name, enabled);
      setItems((prev) => prev.map((x) => (x.task_name === t.task_name ? { ...x, enabled } : x)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  return (
    <div>
      {error && <p className="mb-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink">{error}</p>}
      {saved && <p className="mb-3 rounded-lg bg-accent-soft px-3 py-2 text-sm text-glow">保存成功，配置已生效</p>}

      {!loaded && <div className="h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && items.length === 0 && (
        <div className="rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无任务配置</p>
        </div>
      )}

      <div className="grid gap-4 xl:grid-cols-2">
        {items.map((t) => {
          const meta = TASK_META[t.task_name] ?? { label: t.task_name, desc: "" };
          const d = drafts[t.task_name];
          return (
            <div key={t.task_name} className="rounded-lg border border-line bg-elevated p-4">
              {/* 头部：任务名 + 启用开关 */}
              <div className="flex items-start justify-between">
                <div>
                  <p className="font-display text-sm font-semibold text-ink">{meta.label}</p>
                  <p className="mt-0.5 text-xs text-ink-3">{t.task_name}</p>
                </div>
                <Switch checked={t.enabled} onChange={(v) => void handleToggle(t, v)} label={`启用 ${meta.label}`} />
              </div>
              <p className="mt-2 text-xs text-ink-2">{meta.desc}</p>

              {/* 编辑区（草稿态，保存后生效） */}
              {d && (
                <div className="mt-3 space-y-2">
                  <div className="grid grid-cols-2 gap-2">
                    <label className="block">
                      <span className="text-xs text-ink-3">绑定供应商（留空=自动路由）</span>
                      <select
                        value={d.providerId ?? ""}
                        onChange={(e) =>
                          setDrafts({ ...drafts, [t.task_name]: { ...d, providerId: e.target.value === "" ? null : Number(e.target.value) } })
                        }
                        className="mt-1 w-full rounded-lg border border-line bg-elevated px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                      >
                        <option value="">自动路由（按优先级）</option>
                        {providers.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name}
                            {p.enabled ? "" : "（已停用）"}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="block">
                      <span className="text-xs text-ink-3">模型（留空=供应商默认）</span>
                      <input
                        value={d.model}
                        onChange={(e) => setDrafts({ ...drafts, [t.task_name]: { ...d, model: e.target.value } })}
                        placeholder="deepseek-chat"
                        className="mt-1 w-full rounded-lg border border-line bg-elevated px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                      />
                    </label>
                  </div>
                  <label className="block">
                    <span className="text-xs text-ink-3">最大输出 token（1-8192）</span>
                    <input
                      type="number"
                      min={1}
                      max={8192}
                      value={d.maxTokens}
                      onChange={(e) => setDrafts({ ...drafts, [t.task_name]: { ...d, maxTokens: Number(e.target.value) || 512 } })}
                      className="mt-1 w-full rounded-lg border border-line bg-elevated px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                    />
                  </label>
                  <label className="block">
                    <span className="text-xs text-ink-3">提示词模板（{`{title}`} / {`{content}`} 占位符）</span>
                    <textarea
                      value={d.prompt}
                      onChange={(e) => setDrafts({ ...drafts, [t.task_name]: { ...d, prompt: e.target.value } })}
                      rows={4}
                      className="mt-1 w-full resize-y rounded-lg border border-line bg-elevated px-2 py-1.5 text-xs text-ink outline-none focus:border-accent"
                    />
                  </label>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={() => void handleSave(t)}
                      className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90"
                    >
                      保存配置
                    </button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
