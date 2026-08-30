// src/components/admin/open-api/open-api-manager.tsx
// 接口开放主管理：上半部「接口目录」多选 + 过期时间 + 生成 Key；
// 下半部「Key 列表」（key 展示/复制、绑定接口、时间信息、删除与 AI 开发手册按钮）。
"use client";

import { useEffect, useState } from "react";

import { AiManualModal } from "@/components/admin/open-api/ai-manual-modal";
import { EndpointsPermModal } from "@/components/admin/open-api/endpoints-perm-modal";
import { EndpointCatalog } from "@/components/admin/open-api/endpoint-catalog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ApiError } from "@/lib/api";
import {
  apiCreateOpenApiKey,
  apiDeleteOpenApiKey,
  apiOpenApiCatalog,
  apiOpenApiKeys,
} from "@/lib/api-openapi";
import type { CatalogEntry, OpenAPIKey } from "@/lib/api-openapi";

// 过期时间快捷选项（value 为天数；空串 = 永久；custom = 自定义天数输入）
const EXPIRE_OPTIONS: readonly { value: string; label: string }[] = [
  { value: "", label: "永久有效" },
  { value: "7", label: "7 天" },
  { value: "30", label: "30 天" },
  { value: "90", label: "90 天" },
  { value: "custom", label: "自定义" },
];

// formatDate 格式化「YYYY-MM-DD HH:mm」（空值兜底；纯函数）。
function formatDate(iso: string | null): string {
  if (!iso) {
    return "—";
  }
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// OpenApiManager 接口开放主组件。
export function OpenApiManager() {
  // 目录与凭证数据
  const [catalog, setCatalog] = useState<CatalogEntry[]>([]);
  const [keys, setKeys] = useState<OpenAPIKey[]>([]);
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 生成表单状态
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [name, setName] = useState<string>("");
  const [expireChoice, setExpireChoice] = useState<string>("");
  const [customDays, setCustomDays] = useState<string>("");
  const [creating, setCreating] = useState<boolean>(false);

  // Key 操作状态（删除确认 / 手册弹窗 / 单个 Key 复制反馈）
  const [deleteTarget, setDeleteTarget] = useState<OpenAPIKey | null>(null);
  const [deleting, setDeleting] = useState<boolean>(false);
  const [manualTarget, setManualTarget] = useState<OpenAPIKey | null>(null);
  const [permTarget, setPermTarget] = useState<OpenAPIKey | null>(null);
  const [copiedKeyId, setCopiedKeyId] = useState<number>(0);

  // 加载目录与凭证列表
  useEffect(() => {
    Promise.all([apiOpenApiCatalog(), apiOpenApiKeys()])
      .then(([catalogRes, keysRes]) => {
        setCatalog(catalogRes.items ?? []);
        setKeys(keysRes.items ?? []);
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoaded(true));
  }, []);

  // 切换接口勾选
  const toggleEndpoint = (endpoint: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(endpoint)) {
        next.delete(endpoint);
      } else {
        next.add(endpoint);
      }
      return next;
    });
  };

  // 全选/清空
  const toggleAll = (checked: boolean) => {
    setSelected(checked ? new Set(catalog.map((e) => e.endpoint)) : new Set());
  };

  // 解析过期天数（永久=0→null；自定义=输入值；快捷=选项值）
  const resolveExpireDays = (): number | null => {
    if (expireChoice === "" || expireChoice === "custom") {
      const days = Number(customDays);
      return expireChoice === "custom" && Number.isInteger(days) && days > 0 ? days : null;
    }
    return Number(expireChoice);
  };

  // 生成 Key（成功后清空选择区并刷新列表）
  const handleCreate = async () => {
    setError("");
    if (selected.size === 0) {
      setError("请至少选择一个接口");
      return;
    }
    setCreating(true);
    try {
      await apiCreateOpenApiKey({
        name: name.trim(),
        endpoints: [...selected],
        expire_days: resolveExpireDays(),
      });
      setSelected(new Set());
      setName("");
      setExpireChoice("");
      setCustomDays("");
      const res = await apiOpenApiKeys();
      setKeys(res.items ?? []);
    } catch (err) {
      setError(err instanceof ApiError || err instanceof ApiError ? err.message : "生成失败");
    } finally {
      setCreating(false);
    }
  };

  // 确认删除（删除后从列表移除）
  const handleDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setDeleting(true);
    try {
      await apiDeleteOpenApiKey(deleteTarget.id);
      setKeys((prev) => prev.filter((k) => k.id !== deleteTarget.id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "删除失败");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  // 权限保存成功：用返回记录原位替换列表项
  const handlePermSaved = (record: OpenAPIKey) => {
    setKeys((prev) => prev.map((k) => (k.id === record.id ? record : k)));
  };

  // 复制单个 Key（2 秒反馈）
  const handleCopyKey = async (record: OpenAPIKey) => {
    try {
      await navigator.clipboard.writeText(record.key);
      setCopiedKeyId(record.id);
      setTimeout(() => setCopiedKeyId(0), 2000);
    } catch {
      // 剪贴板不可用时静默（用户可手动选中文本）
    }
  };

  // 按接口标识查名称（Key 列表的授权 chips 提示用）
  const endpointName = (endpoint: string): string =>
    catalog.find((e) => e.endpoint === endpoint)?.name ?? endpoint;

  return (
    <div>
      {/* 页头 */}
      <div className="mb-5">
        <h1 className="font-display text-xl font-semibold text-ink">接口开放</h1>
        <p className="mt-0.5 text-xs text-ink-3">
          勾选接口生成 API Key，外部应用凭 X-Api-Key 调用 /api/v1/open/* 开放接口
        </p>
      </div>

      {error && (
        <p className="mb-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}

      {/* ---------- 上半部：接口目录 + 生成 Key ---------- */}
      <div className="rounded-xl border border-line bg-bg p-4">
        <h2 className="mb-3 font-display text-base font-semibold text-ink">开放接口目录</h2>
        <EndpointCatalog entries={catalog} selected={selected} onToggle={toggleEndpoint} onToggleAll={toggleAll} />

        {/* 生成区：备注名 + 过期时间 + 生成按钮 */}
        <div className="mt-4 flex flex-wrap items-end gap-3">
          <label className="flex flex-col gap-1">
            <span className="text-xs text-ink-3">备注名（可选）</span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              placeholder="如：Chrome 插件"
              className="h-9 w-44 rounded-lg border border-line bg-elevated px-3 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-xs text-ink-3">过期时间</span>
            <select
              value={expireChoice}
              onChange={(e) => setExpireChoice(e.target.value)}
              className="h-9 w-32 rounded-lg border border-line bg-elevated px-2 text-sm text-ink focus:border-accent focus:outline-none"
            >
              {EXPIRE_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </label>
          {expireChoice === "custom" && (
            <label className="flex flex-col gap-1">
              <span className="text-xs text-ink-3">自定义天数</span>
              <input
                type="number"
                min={1}
                value={customDays}
                onChange={(e) => setCustomDays(e.target.value)}
                placeholder="天数"
                className="h-9 w-24 rounded-lg border border-line bg-elevated px-3 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
              />
            </label>
          )}
          <button
            type="button"
            disabled={creating || selected.size === 0}
            onClick={() => void handleCreate()}
            className="h-9 rounded-full bg-accent px-6 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {creating ? "生成中…" : "生成 Key"}
          </button>
        </div>
      </div>

      {/* ---------- 下半部：Key 列表 ---------- */}
      <h2 className="mb-3 mt-6 font-display text-base font-semibold text-ink">已生成的 Key</h2>
      <div className="overflow-hidden rounded-lg border border-line bg-elevated">
        {loaded && keys.length === 0 ? (
          <div className="px-6 py-12 text-center text-sm text-ink-3">
            还没有生成过 Key，勾选上方接口后点击「生成 Key」
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line text-left text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">Key</th>
                <th className="px-4 py-3 font-normal">授权接口</th>
                <th className="px-4 py-3 font-normal">创建时间</th>
                <th className="px-4 py-3 font-normal">过期时间</th>
                <th className="px-4 py-3 font-normal">最近使用</th>
                <th className="px-4 py-3 text-right font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {!loaded &&
                Array.from({ length: 2 }).map((_, i) => (
                  <tr key={i}>
                    <td colSpan={6} className="px-4 py-4">
                      <div className="h-4 animate-pulse rounded bg-muted" aria-hidden />
                    </td>
                  </tr>
                ))}
              {keys.map((record) => (
                <tr key={record.id} className="align-top">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <code className="break-all font-mono text-xs text-ink">{record.key}</code>
                      <button
                        type="button"
                        onClick={() => void handleCopyKey(record)}
                        className="shrink-0 rounded-full border border-line px-2.5 py-1 text-xs text-ink-2 transition-colors hover:text-ink"
                      >
                        {copiedKeyId === record.id ? "已复制 ✓" : "复制"}
                      </button>
                    </div>
                    {record.name && <p className="mt-1 text-xs text-ink-3">{record.name}</p>}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex max-w-[240px] flex-wrap gap-1">
                      {record.endpoints.map((ep) => (
                        <span key={ep} className="rounded-full bg-accent-soft px-2 py-0.5 text-[11px] text-glow">
                          {endpointName(ep)}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-xs text-ink-2">{formatDate(record.created_at)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-xs text-ink-2">
                    {record.expires_at ? formatDate(record.expires_at) : "永久有效"}
                  </td>
                  <td className="whitespace-nowrap px-4 py-3 text-xs text-ink-2">{formatDate(record.last_used_at)}</td>
                  <td className="whitespace-nowrap px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => setPermTarget(record)}
                      className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:border-accent hover:text-glow"
                    >
                      权限设置
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeleteTarget(record)}
                      className="ml-2 rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:border-like/50 hover:text-like"
                    >
                      删除
                    </button>
                    <button
                      type="button"
                      onClick={() => setManualTarget(record)}
                      className="ml-2 rounded-full border border-line px-3 py-1 text-xs text-ink-2 transition-colors hover:text-glow"
                    >
                      AI 开发手册
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* 删除确认 */}
      <ConfirmDialog
        open={deleteTarget !== null}
        title="删除 API Key"
        description={`确定删除这个 Key 吗？删除后使用该 Key 的外部应用将立即无法调用开放接口（不可恢复）。`}
        confirmText="删除"
        danger
        loading={deleting}
        onConfirm={() => void handleDelete()}
        onClose={() => setDeleteTarget(null)}
      />

      {/* 权限设置弹窗（勾选增/减授权接口） */}
      {permTarget && (
        <EndpointsPermModal
          open={permTarget !== null}
          onClose={() => setPermTarget(null)}
          apiKey={permTarget}
          catalog={catalog}
          onSaved={handlePermSaved}
        />
      )}

      {/* AI 开发手册弹窗（打开时按 Key 授权的接口生成） */}
      {manualTarget && (
        <AiManualModal
          open={manualTarget !== null}
          onClose={() => setManualTarget(null)}
          apiKey={manualTarget}
          catalog={catalog}
        />
      )}
    </div>
  );
}
