// src/components/admin/ai/tasks-tab.tsx
// AI 设置页 · 任务配置 Tab（M4）：左侧任务列表 + 右侧设置面板（master-detail）。
// 布局依据：任务设置项将逐步增多（模型路由/生成参数/提示词…），左右分栏比双列卡片
// 更易扩展、信息层级更清晰；移动端自动上下堆叠。
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
  "reply.assistant": { label: "智能回复助手", desc: "作者编辑态「AI 续写 / 润色 / 翻译」，对正文执行相应处理" },
  "seo.advice": { label: "SEO 建议", desc: "对文章给出 SEO 标题 / 描述 / 关键词建议，接入 SEO 面板回填" },
};

// TaskDraft 任务配置草稿（仅保存时提交，未保存不生效）。
interface TaskDraft {
  model: string; // 模型名
  prompt: string; // 提示词模板
  maxTokens: number; // 最大输出 token
  providerId: number | null; // 绑定供应商（null=自动路由）
}

// TasksTab 任务配置管理。
export function TasksTab() {
  const [items, setItems] = useState<AiTaskDTO[]>([]);
  const [providers, setProviders] = useState<AiProviderDTO[]>([]); // 渠道商（模型下拉分组）
  const [loaded, setLoaded] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<string>(""); // 当前编辑任务（task_name）
  // 编辑态草稿：task_name → 表单值
  const [drafts, setDrafts] = useState<Record<string, TaskDraft>>({});

  // 加载任务列表与渠道商
  useEffect(() => {
    void apiAiProviders().then((r) => setProviders(r.items)).catch(() => setProviders([]));
    apiAiTasks()
      .then((r) => {
        setItems(r.items);
        // 初始化草稿（与后端同步）+ 默认选中第一个任务
        const init: Record<string, TaskDraft> = {};
        for (const t of r.items) {
          init[t.task_name] = { model: t.model, prompt: t.prompt_template, maxTokens: t.max_tokens, providerId: t.provider_id };
        }
        setDrafts(init);
        if (r.items.length > 0) {
          setSelected(r.items[0].task_name);
        }
      })
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, []);

  // 更新草稿（纯函数式更新，避免闭包旧值）
  const patchDraft = (taskName: string, patch: Partial<TaskDraft>) => {
    setDrafts((prev) => ({ ...prev, [taskName]: { ...prev[taskName], ...patch } }));
  };

  // 保存当前任务配置
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

  // bindingSummary 任务当前绑定摘要（左侧列表展示）。
  const bindingSummary = (t: AiTaskDTO): string => {
    if (t.provider_id === null) {
      return "自动路由";
    }
    const p = providers.find((x) => x.id === t.provider_id);
    return `${p?.name ?? "渠道"} · ${t.model || "默认模型"}`;
  };

  const current = items.find((t) => t.task_name === selected);
  const currentDraft = current ? drafts[current.task_name] : null;

  return (
    <div>
      {/* 全局提示（错误 / 保存成功） */}
      {error && <p className="mb-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink" role="alert">{error}</p>}
      {saved && <p className="mb-3 rounded-lg bg-accent-soft px-3 py-2 text-sm text-glow" role="status">保存成功，配置已生效</p>}

      {/* 加载骨架 */}
      {!loaded && (
        <div className="grid gap-4 lg:grid-cols-[240px_1fr]">
          <div className="h-64 animate-pulse rounded-lg bg-muted" aria-hidden />
          <div className="h-64 animate-pulse rounded-lg bg-muted" aria-hidden />
        </div>
      )}

      {/* 空态 */}
      {loaded && items.length === 0 && (
        <div className="rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无任务配置</p>
        </div>
      )}

      {/* 主体：左列表 + 右面板 */}
      {loaded && items.length > 0 && (
        <div className="grid items-start gap-4 lg:grid-cols-[240px_1fr]">
          {/* 左侧：任务列表 */}
          <nav aria-label="任务列表" className="overflow-hidden rounded-lg border border-line bg-elevated">
            {items.map((t) => {
              const meta = TASK_META[t.task_name] ?? { label: t.task_name, desc: "" };
              const active = t.task_name === selected;
              return (
                <button
                  key={t.task_name}
                  type="button"
                  onClick={() => setSelected(t.task_name)}
                  aria-current={active ? "true" : undefined}
                  className={`flex w-full items-center justify-between gap-2 border-b border-line px-4 py-3 text-left transition-colors last:border-b-0 ${
                    active ? "bg-accent-soft" : "hover:bg-muted/60"
                  }`}
                >
                  <span className="min-w-0">
                    <span className={`block truncate text-sm ${active ? "font-medium text-glow" : "text-ink"}`}>
                      {meta.label}
                    </span>
                    <span className="mt-0.5 block truncate text-[11px] text-ink-3">{bindingSummary(t)}</span>
                  </span>
                  {/* 启用状态点（绿=启用，灰=停用） */}
                  <span
                    aria-hidden
                    className={`h-2 w-2 shrink-0 rounded-full ${t.enabled ? "bg-glow" : "bg-ink-3"}`}
                  />
                </button>
              );
            })}
          </nav>

          {/* 右侧：选中任务的设置面板 */}
          {current && currentDraft && (
            <div className="rounded-lg border border-line bg-elevated p-5">
              {/* 头部：任务名 + 启用开关 */}
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h2 className="font-display text-base font-semibold text-ink">
                    {(TASK_META[current.task_name] ?? { label: current.task_name }).label}
                  </h2>
                  <p className="mt-0.5 text-xs text-ink-3">{current.task_name}</p>
                  <p className="mt-2 text-xs text-ink-2">
                    {(TASK_META[current.task_name] ?? { desc: "" }).desc}
                  </p>
                </div>
                <Switch
                  checked={current.enabled}
                  onChange={(v) => void handleToggle(current, v)}
                  label={`启用 ${(TASK_META[current.task_name] ?? { label: current.task_name }).label}`}
                />
              </div>

              {/* 分组：模型路由 */}
              <div className="mt-5 border-t border-line pt-4">
                <p className="text-xs font-medium text-ink-3">模型路由</p>
                <label className="mt-2 block">
                  <span className="text-xs text-ink-3">模型（按渠道商分组；自动路由 = 系统按优先级调度）</span>
                  <select
                    value={currentDraft.providerId !== null ? `${currentDraft.providerId}:${currentDraft.model}` : ""}
                    onChange={(e) => {
                      const v = e.target.value;
                      if (v === "") {
                        patchDraft(current.task_name, { providerId: null, model: "" });
                        return;
                      }
                      // value 格式 `${providerId}:${model}`（模型名可能含冒号，取首段为 ID）
                      const sep = v.indexOf(":");
                      patchDraft(current.task_name, {
                        providerId: Number(v.slice(0, sep)),
                        model: v.slice(sep + 1),
                      });
                    }}
                    className="mt-1 w-full max-w-md rounded-lg border border-line bg-elevated px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                  >
                    <option value="">自动路由（按优先级）</option>
                    {providers
                      .filter((p) => p.models.length > 0)
                      .map((p) => (
                        <optgroup key={p.id} label={`${p.name}${p.enabled ? "" : "（已停用）"}`}>
                          {p.models.map((m) => (
                            <option key={`${p.id}:${m}`} value={`${p.id}:${m}`}>
                              {m}
                            </option>
                          ))}
                        </optgroup>
                      ))}
                    {/* 历史手动模型（不在当前渠道商模型清单中时回显，避免选择丢失） */}
                    {currentDraft.providerId !== null &&
                      currentDraft.model !== "" &&
                      !providers.some((p) => p.id === currentDraft.providerId && p.models.includes(currentDraft.model)) && (
                        <option value={`${currentDraft.providerId}:${currentDraft.model}`}>
                          当前：{providers.find((p) => p.id === currentDraft.providerId)?.name ?? "未知渠道"} / {currentDraft.model}
                        </option>
                      )}
                  </select>
                </label>
              </div>

              {/* 分组：生成参数（后续温度/流式等新设置在此区块追加） */}
              <div className="mt-5 border-t border-line pt-4">
                <p className="text-xs font-medium text-ink-3">生成参数</p>
                <label className="mt-2 block">
                  <span className="text-xs text-ink-3">最大输出 token（1-8192）</span>
                  <input
                    type="number"
                    min={1}
                    max={8192}
                    value={currentDraft.maxTokens}
                    onChange={(e) => patchDraft(current.task_name, { maxTokens: Number(e.target.value) || 512 })}
                    className="mt-1 w-full max-w-[240px] rounded-lg border border-line bg-elevated px-2 py-1.5 text-sm text-ink outline-none focus:border-accent"
                  />
                </label>
              </div>

              {/* 分组：提示词模板 */}
              <div className="mt-5 border-t border-line pt-4">
                <p className="text-xs font-medium text-ink-3">提示词模板</p>
                <label className="mt-2 block">
                  <span className="text-xs text-ink-3">（{`{title}`} / {`{content}`} 占位符）</span>
                  <textarea
                    value={currentDraft.prompt}
                    onChange={(e) => patchDraft(current.task_name, { prompt: e.target.value })}
                    rows={6}
                    className="mt-1 w-full resize-y rounded-lg border border-line bg-elevated px-2 py-1.5 text-xs leading-relaxed text-ink outline-none focus:border-accent"
                  />
                </label>
              </div>

              {/* 操作 */}
              <div className="mt-5 flex justify-end">
                <button
                  type="button"
                  onClick={() => void handleSave(current)}
                  className="rounded-full bg-accent px-6 py-2 text-sm font-medium text-on-accent hover:opacity-90"
                >
                  保存配置
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
