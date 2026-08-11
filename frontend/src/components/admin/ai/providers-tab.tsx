// src/components/admin/ai/providers-tab.tsx
// AI 设置页 · 供应商 Tab（M4）：供应商表格 + 新增/编辑弹层 + 测试连接 + 删除。
// 设计依据：无 AI 画板，参照后台既有表格/弹层模式（SEO 设置页风格）自行设计。
"use client";

import { useEffect, useState } from "react";

import {
  apiAiCreateProvider,
  apiAiDeleteProvider,
  apiAiProviders,
  apiAiTestProvider,
  apiAiUpdateProvider,
  type AiProviderDTO,
  type AiProviderInput,
} from "@/lib/api-ai";
import { ApiError } from "@/lib/api";
import { Modal } from "@/components/ui/modal";
import { Switch } from "@/components/ui/switch";

// 空表单初始值（新增弹层）。
const EMPTY_FORM: AiProviderInput = {
  name: "",
  base_url: "",
  api_key: "",
  models: [],
  enabled: true,
  priority: 10,
};

// ProvidersTab 供应商管理。
export function ProvidersTab() {
  const [items, setItems] = useState<AiProviderDTO[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [editing, setEditing] = useState<AiProviderDTO | null>(null); // 编辑对象（null=新增）
  const [form, setForm] = useState<AiProviderInput>(EMPTY_FORM);
  const [modelsText, setModelsText] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // 加载供应商列表
  useEffect(() => {
    apiAiProviders()
      .then((r) => setItems(r.items))
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, []);

  // 打开新增弹层
  const openCreate = () => {
    setEditing(null);
    setForm(EMPTY_FORM);
    setModelsText("");
    setError("");
  };

  // 打开编辑弹层（回填表单；API Key 不回显，留空 = 不修改）
  const openEdit = (p: AiProviderDTO) => {
    setEditing(p);
    setForm({
      name: p.name,
      base_url: p.base_url,
      api_key: "",
      models: p.models,
      enabled: p.enabled,
      priority: p.priority,
    });
    setModelsText(p.models.join(", "));
    setError("");
  };

  // 保存（新增/更新）
  const handleSave = async () => {
    setBusy(true);
    setError("");
    try {
      const input: AiProviderInput = {
        ...form,
        models: modelsText
          .split(/[,，]/)
          .map((m) => m.trim())
          .filter((m) => m !== ""),
      };
      if (editing) {
        await apiAiUpdateProvider(editing.id, input);
      } else {
        await apiAiCreateProvider(input);
      }
      const r = await apiAiProviders();
      setItems(r.items);
      setEditing(null);
      setForm(EMPTY_FORM);
      setModelsText("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  };

  // 测试连接
  const handleTest = async (p: AiProviderDTO) => {
    setError("");
    setBusy(true);
    try {
      const r = await apiAiTestProvider(p.id);
      alert(`「${p.name}」${r.message}`);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "连接失败");
    } finally {
      setBusy(false);
    }
  };

  // 删除供应商（确认后执行）
  const handleDelete = async (p: AiProviderDTO) => {
    if (!confirm(`确认删除供应商「${p.name}」？引用它的任务将自动恢复自动路由。`)) return;
    try {
      await apiAiDeleteProvider(p.id);
      setItems((prev) => prev.filter((x) => x.id !== p.id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between">
        <p className="text-xs text-ink-3">
          配置 OpenAI 兼容供应商（deepseek / qwen / kimi / glm / openai 已预置，填入 API Key 即可使用）
        </p>
        <button
          type="button"
          onClick={openCreate}
          className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90"
        >
          + 新增供应商
        </button>
      </div>

      {error && <p className="mt-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink">{error}</p>}

      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && items.length === 0 && (
        <div className="mt-4 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">暂无供应商，点击右上角新增</p>
        </div>
      )}
      {loaded && items.length > 0 && (
        <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
          <table className="w-full min-w-[680px] text-left text-sm">
            <thead>
              <tr className="border-b border-line text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">名称</th>
                <th className="px-4 py-3 font-normal">接口地址</th>
                <th className="px-4 py-3 font-normal">模型</th>
                <th className="px-4 py-3 font-normal">优先级</th>
                <th className="px-4 py-3 font-normal">API Key</th>
                <th className="px-4 py-3 font-normal">启用</th>
                <th className="px-4 py-3 font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {items.map((p) => (
                <tr key={p.id} className="hover:bg-muted/40">
                  <td className="px-4 py-3 font-medium text-ink">{p.name}</td>
                  <td className="max-w-[200px] px-4 py-3 text-xs text-ink-2">
                    <span className="line-clamp-1">{p.base_url}</span>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink-2">{p.models.join(" / ")}</td>
                  <td className="px-4 py-3 text-ink-2">{p.priority}</td>
                  <td className="px-4 py-3">
                    {p.api_key_set ? (
                      <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">已配置</span>
                    ) : (
                      <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-ink-3">未配置</span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <span className={`text-xs ${p.enabled ? "text-glow" : "text-ink-3"}`}>
                      {p.enabled ? "启用" : "停用"}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2 text-xs">
                      <button type="button" onClick={() => void handleTest(p)} disabled={busy} className="text-ink-2 hover:text-ink">
                        测试
                      </button>
                      <button type="button" onClick={() => openEdit(p)} className="text-ink-2 hover:text-ink">
                        编辑
                      </button>
                      <button type="button" onClick={() => void handleDelete(p)} className="text-like hover:opacity-80">
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* 新增/编辑弹层 */}
      <Modal open={editing !== null} title={editing ? `编辑供应商 · ${editing.name}` : "新增供应商"} onClose={() => setEditing(null)} maxWidth="max-w-[480px]">
        <div className="space-y-3">
          <label className="block">
            <span className="text-xs text-ink-3">名称（必填）</span>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="如 deepseek"
              className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-3">接口地址（必填，OpenAI 兼容）</span>
            <input
              value={form.base_url}
              onChange={(e) => setForm({ ...form, base_url: e.target.value })}
              placeholder="如 https://api.deepseek.com/v1"
              className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-3">API Key{editing ? "（留空 = 不修改）" : "（必填）"}</span>
            <input
              type="password"
              value={form.api_key}
              onChange={(e) => setForm({ ...form, api_key: e.target.value })}
              placeholder={editing ? "sk-**** 已配置" : "sk-..."}
              className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-3">模型列表（必填，逗号分隔）</span>
            <input
              value={modelsText}
              onChange={(e) => setModelsText(e.target.value)}
              placeholder="deepseek-chat, deepseek-reasoner"
              className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </label>
          <label className="block">
            <span className="text-xs text-ink-3">路由优先级（1-100，小先选）</span>
            <input
              type="number"
              min={1}
              max={100}
              value={form.priority}
              onChange={(e) => setForm({ ...form, priority: Number(e.target.value) || 10 })}
              className="mt-1 w-full rounded-lg border border-line bg-elevated px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            />
          </label>
          <div className="flex items-center justify-between rounded-lg bg-muted px-3 py-2">
            <span className="text-sm text-ink-2">启用该供应商</span>
            <Switch checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} label="启用供应商" />
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => setEditing(null)}
              className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={busy}
              className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90 disabled:opacity-50"
            >
              保存
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
