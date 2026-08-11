// src/app/admin/plugins/[id]/settings/page.tsx
// 插件设置页（M3.2 前端扩展点：schema 驱动通用渲染器）：
// 从清单拉取插件 settings_schema → 渲染 text/switch/select 字段 → 保存到 settings（前缀键 plugin_{id}_）。
// 后端零改动（settings 扁平 Record 天然支持前缀键）。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { Switch } from "@/components/ui/switch";
import {
  apiAdminSaveSettings,
  apiAdminSettings,
  apiPluginMarket,
  ApiError,
  type PluginInfo,
  type PluginSettingField,
} from "@/lib/api";

// PluginSettings 插件设置页。
export default function PluginSettings() {
  const params = useParams<{ id: string }>();
  const pluginId = params.id;

  const [plugin, setPlugin] = useState<PluginInfo | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [saved, setSaved] = useState<boolean>(false);

  // 加载插件 schema + 已存配置
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const market = await apiPluginMarket();
        const found = market.items.find((p) => p.id === pluginId);
        if (!found) {
          setError("插件不存在");
          return;
        }
        if (cancelled) return;
        setPlugin(found);
        // 读取已存配置（前缀键 plugin_{id}_）
        const all = await apiAdminSettings();
        const prefix = `plugin_${pluginId}_`;
        const next: Record<string, string> = {};
        for (const field of found.settings_schema ?? []) {
          next[field.key] = all[prefix + field.key] ?? field.default ?? "";
        }
        if (cancelled) return;
        setValues(next);
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiError ? err.message : "加载失败");
      } finally {
        if (!cancelled) setLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [pluginId]);

  // 保存（前缀键写入 settings）
  const handleSave = async () => {
    if (!plugin) return;
    setError("");
    setSaved(false);
    const prefix = `plugin_${pluginId}_`;
    const updates: Record<string, string> = {};
    for (const field of plugin.settings_schema ?? []) {
      updates[prefix + field.key] = values[field.key] ?? "";
    }
    try {
      await apiAdminSaveSettings(updates);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    }
  };

  if (!loaded) {
    return <p className="py-16 text-center text-sm text-ink-3">加载中…</p>;
  }
  if (!plugin) {
    return (
      <div className="py-16 text-center">
        <p className="text-ink-2">{error || "插件不存在"}</p>
        <a href="/admin/plugins" className="mt-3 inline-block text-sm text-glow hover:underline">
          返回我的插件
        </a>
      </div>
    );
  }

  const fields = plugin.settings_schema ?? [];

  return (
    <div className="max-w-[560px]">
      <h1 className="font-display text-xl font-semibold text-ink">{plugin.name} · 设置</h1>
      <p className="mt-0.5 text-xs text-ink-3">v{plugin.version} · 配置保存后即时生效</p>

      {fields.length === 0 ? (
        <div className="mt-6 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">该插件暂无配置项</p>
        </div>
      ) : (
        <div className="mt-5 space-y-5 rounded-lg border border-line bg-elevated p-6">
          {fields.map((field: PluginSettingField) => (
            <div key={field.key} className="flex items-center justify-between gap-4">
              <p className="text-sm text-ink">{field.label}</p>
              {field.type === "switch" ? (
                <Switch
                  checked={values[field.key] === "on"}
                  onChange={(v) => setValues((prev) => ({ ...prev, [field.key]: v ? "on" : "off" }))}
                  label={field.label}
                />
              ) : field.type === "select" ? (
                <select
                  value={values[field.key] ?? ""}
                  onChange={(e) => setValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  className="h-10 w-48 rounded-lg border border-line bg-muted px-3 text-sm text-ink focus:border-accent focus:outline-none"
                >
                  {field.options?.map((opt) => (
                    <option key={opt} value={opt}>
                      {opt}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  value={values[field.key] ?? ""}
                  onChange={(e) => setValues((prev) => ({ ...prev, [field.key]: e.target.value }))}
                  className="h-10 w-64 rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
                />
              )}
            </div>
          ))}

          {error && (
            <p className="rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}
          {saved && (
            <p className="rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
              保存成功，设置已生效
            </p>
          )}

          <div className="flex justify-end">
            <button
              type="button"
              onClick={() => void handleSave()}
              className="rounded-full bg-accent px-8 py-2.5 text-sm font-medium text-on-accent hover:opacity-90"
            >
              保存设置
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
