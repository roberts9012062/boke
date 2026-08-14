// src/app/admin/plugins/[id]/settings/page.tsx
// 插件设置页（M3.7 设置端到端：schema 驱动通用渲染器）：
// 详情接口聚合「进程 Info 上报优先、市场清单兜底」→ 渲染 text/switch/select 字段
// → 保存到 plugin_instances.config（JSONB，未声明键被 service 过滤）→ 运行中推送即时生效。
"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { Switch } from "@/components/ui/switch";
import { ImageSettingField } from "@/components/admin/image-setting-field";
import {
  apiPluginDetail,
  apiPluginSaveConfig,
  ApiError,
  type PluginDetail,
  type PluginSettingField,
} from "@/lib/api";

// PluginSettings 插件设置页（:id 兼容实例 ID 数字与插件 ID 字符串——nav 动态入口/直达链接用插件 ID）。
export default function PluginSettings() {
  const params = useParams<{ id: string }>();
  const pluginRef = params.id; // 原样传给后端（后端解析：数字=实例 ID，字符串=插件 ID）

  const [plugin, setPlugin] = useState<PluginDetail | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [loaded, setLoaded] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [saved, setSaved] = useState<boolean>(false);

  // 加载插件详情（schema 聚合 + 已存配置）
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const { plugin: detail } = await apiPluginDetail(pluginRef);
        if (cancelled) return;
        setPlugin(detail);
        // 初始化表单值：已存配置优先，未设置项用 schema 默认值
        const next: Record<string, string> = {};
        for (const field of detail.settings_schema ?? []) {
          next[field.key] = detail.config?.[field.key] ?? field.default ?? "";
        }
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
  }, [pluginRef]);

  // 保存（service 按 schema 过滤未声明键；运行中推送即时生效）
  const handleSave = async () => {
    if (!plugin) return;
    setError("");
    setSaved(false);
    try {
      const { config } = await apiPluginSaveConfig(pluginRef, values);
      setValues(config); // 回显过滤后的实际保存值
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
          {fields.map((field: PluginSettingField) =>
            field.type === "image" ? (
              // 图片字段（M5：默认 OG 图等）：label 置顶 + 上传/裁剪/预览区
              <div key={field.key} className="flex flex-col items-start gap-1.5">
                <p className="text-sm text-ink">{field.label}</p>
                <ImageSettingField
                  field={field}
                  value={values[field.key] ?? ""}
                  onChange={(url) => setValues((prev) => ({ ...prev, [field.key]: url }))}
                />
              </div>
            ) : (
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
            )
          )}

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
