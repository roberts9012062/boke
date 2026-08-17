// src/components/admin/plugin-market/proxy-settings.tsx
// 插件商城 GitHub 加速代理设置（国内网络直连 api.github.com 失败时使用）：
// 直连 / 预置候选 / 自定义地址三态选择，保存 settings.plugin_proxy 后回调触发商城重新拉取。
"use client";

import { useEffect, useState } from "react";

import { PROXY_PRESETS } from "@/components/admin/plugin-market/labels";
import { apiAdminSaveSettings, apiAdminSettings, ApiError } from "@/lib/api";

// ProxySettingsProps 组件属性。
type ProxySettingsProps = {
  onApplied: () => void; // 应用代理后的回调（商城按新代理重新拉取）
};

// ProxySettings 代理选择行（设置持久化 + 即时生效，无需重启服务）。
export function ProxySettings({ onApplied }: ProxySettingsProps) {
  // 选择态：""=直连（默认）"custom"=自定义输入 其余=预置代理地址
  const [selected, setSelected] = useState<string>("");
  const [custom, setCustom] = useState<string>(""); // 自定义代理地址输入
  const [current, setCurrent] = useState<string>(""); // 当前生效代理（设置回显）
  const [saving, setSaving] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  // 初始读取已保存代理（回显选择状态；读取失败保持直连默认展示）
  useEffect(() => {
    apiAdminSettings()
      .then((settings) => {
        const saved = (settings.plugin_proxy ?? "").trim();
        setCurrent(saved);
        if (saved === "") {
          return;
        }
        const preset = PROXY_PRESETS.find((p) => p.value === saved);
        setSelected(preset ? saved : "custom");
        if (!preset) {
          setCustom(saved);
        }
      })
      .catch(() => {
        /* 设置读取失败静默（保持直连默认展示） */
      });
  }, []);

  // 应用代理（保存设置 → 触发商城重拉；空值=恢复直连）
  const apply = async () => {
    const proxy = selected === "custom" ? custom.trim() : selected;
    if (selected === "custom" && !/^https:\/\//.test(proxy)) {
      setError("自定义代理地址需以 https:// 开头");
      return;
    }
    setError("");
    setSaving(true);
    try {
      await apiAdminSaveSettings({ plugin_proxy: proxy });
      setCurrent(proxy);
      onApplied();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存代理设置失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mt-2 flex flex-wrap items-center gap-2">
      <span className="text-xs text-ink-3">加速代理：</span>
      <select
        value={selected}
        onChange={(e) => setSelected(e.target.value)}
        className="h-9 rounded-full border border-line bg-elevated px-3 text-sm text-ink focus:border-accent focus:outline-none"
        aria-label="GitHub 加速代理"
      >
        <option value="">直连（默认）</option>
        {PROXY_PRESETS.map((p) => (
          <option key={p.value} value={p.value}>
            {p.label}
          </option>
        ))}
        <option value="custom">自定义…</option>
      </select>
      {selected === "custom" && (
        <input
          type="text"
          value={custom}
          onChange={(e) => setCustom(e.target.value)}
          placeholder="https://your-proxy.example.com"
          className="h-9 w-64 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      )}
      <button
        type="button"
        disabled={saving}
        onClick={() => void apply()}
        className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink disabled:opacity-50"
      >
        应用
      </button>
      <span className="text-xs text-ink-3">
        {current
          ? `当前：${current}（代理模式下不发送 GitHub Token，仅支持公开仓库）`
          : "网络无法直连 GitHub 时选择代理后重新拉取"}
      </span>
      {error && (
        <span className="text-xs text-like" role="alert">
          {error}
        </span>
      )}
    </div>
  );
}
