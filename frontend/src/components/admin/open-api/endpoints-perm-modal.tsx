// src/components/admin/open-api/endpoints-perm-modal.tsx
// Key 权限设置弹窗：展示开放接口目录复选（复用 EndpointCatalog），
// 勾选增/减该 Key 可调用的接口，保存调 PUT /admin/open-api/keys/:id/endpoints。
"use client";

import { useState } from "react";

import { EndpointCatalog } from "@/components/admin/open-api/endpoint-catalog";
import { Modal } from "@/components/ui/modal";
import { ApiError } from "@/lib/api";
import { apiUpdateOpenApiKeyEndpoints } from "@/lib/api-openapi";
import type { CatalogEntry, OpenAPIKey } from "@/lib/api-openapi";

// EndpointsPermModal 权限设置弹窗属性。
interface EndpointsPermModalProps {
  open: boolean; // 弹窗可见性
  onClose: () => void; // 关闭回调
  apiKey: OpenAPIKey; // 目标 Key（初始勾选取其 endpoints）
  catalog: CatalogEntry[]; // 接口目录（复选数据源）
  onSaved: (record: OpenAPIKey) => void; // 保存成功回调（父组件刷新列表）
}

// EndpointsPermModal Key 权限设置弹窗。
export function EndpointsPermModal({ open, onClose, apiKey, catalog, onSaved }: EndpointsPermModalProps) {
  // 勾选状态：打开时以该 Key 当前授权初始化（key 变化时重置由父组件条件渲染保证）
  const [selected, setSelected] = useState<Set<string>>(new Set(apiKey.endpoints));
  const [saving, setSaving] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // toggleEndpoint 切换单个接口勾选
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

  // toggleAll 全选/清空
  const toggleAll = (checked: boolean) => {
    setSelected(checked ? new Set(catalog.map((e) => e.endpoint)) : new Set());
  };

  // handleSave 保存授权变更（成功后回传新记录并关闭）
  const handleSave = async () => {
    setError("");
    if (selected.size === 0) {
      setError("请至少保留一个授权接口");
      return;
    }
    setSaving(true);
    try {
      const record = await apiUpdateOpenApiKeyEndpoints(apiKey.id, [...selected]);
      onSaved(record);
      onClose();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="权限设置" maxWidth="max-w-2xl">
      <div className="space-y-4">
        <p className="text-xs text-ink-3">
          Key <code className="font-mono text-ink-2">{apiKey.key}</code>
          {apiKey.name ? `（${apiKey.name}）` : ""} · 勾选或取消接口后保存，立即生效
        </p>
        {error && (
          <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
            {error}
          </p>
        )}
        <EndpointCatalog entries={catalog} selected={selected} onToggle={toggleEndpoint} onToggleAll={toggleAll} />
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="h-9 rounded-full border border-line px-5 text-sm text-ink-2 transition-colors hover:text-ink"
          >
            取消
          </button>
          <button
            type="button"
            disabled={saving || selected.size === 0}
            onClick={() => void handleSave()}
            className="h-9 rounded-full bg-accent px-6 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {saving ? "保存中…" : "保存"}
          </button>
        </div>
      </div>
    </Modal>
  );
}
