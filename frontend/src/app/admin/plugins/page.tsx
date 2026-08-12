// src/app/admin/plugins/page.tsx
// 后台我的插件（M3.1，设计稿《插件卸载·SEO/成功》配套管理页）：
// 已安装插件列表（名称/版本/来源/状态/安装时间/操作：启用禁用 · 卸载）+ 空态引导去商城。
// M3.4 新增：本地 .bpk 上传安装入口。
"use client";

import { useEffect, useRef, useState } from "react";

import {
  apiActivateLicense,
  apiInstalledPlugins,
  apiSetPluginState,
  apiUninstallPlugin,
  apiUploadPluginBpk,
  ApiError,
  type InstalledPlugin,
} from "@/lib/api";
import { formatDateTime } from "@/lib/utils";

// 状态文案（插件实例状态字典；crashed = 连续崩溃熔断，M3.3 进程外插件）
const STATE_LABEL: Record<string, string> = {
  running: "已启用",
  disabled: "已禁用",
  installed: "已安装",
  crashed: "已熔断",
};

// AdminPlugins 我的插件。
export default function AdminPlugins() {
  const [items, setItems] = useState<InstalledPlugin[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string>("");
  // 卸载确认弹层
  const [uninstallTarget, setUninstallTarget] = useState<InstalledPlugin | null>(null);
  const [uninstalling, setUninstalling] = useState<boolean>(false);
  const [uninstalled, setUninstalled] = useState<boolean>(false);
  // 本地上传安装（M3.4）
  const [uploading, setUploading] = useState<boolean>(false);
  const [uploadDone, setUploadDone] = useState<boolean>(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  // 许可证激活（M3.5）
  const [licenseTarget, setLicenseTarget] = useState<InstalledPlugin | null>(null);
  const [licenseJwt, setLicenseJwt] = useState<string>("");
  const [activating, setActivating] = useState<boolean>(false);
  const [licenseDone, setLicenseDone] = useState<boolean>(false);

  // 选择 .bpk 后上传安装（成功后刷新列表）
  const onPickFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    setUploadDone(false);
    setError("");
    apiUploadPluginBpk(file)
      .then(() => {
        setUploadDone(true);
        load();
      })
      .catch((err) => setError(err instanceof ApiError ? err.message : "安装失败"))
      .finally(() => {
        setUploading(false);
        e.target.value = ""; // 允许重复选择同一文件
      });
  };

  // 加载列表
  const load = () => {
    setLoading(true);
    apiInstalledPlugins()
      .then((r) => setItems(r.items))
      .catch((err) => setError(err instanceof ApiError ? err.message : "加载失败"))
      .finally(() => setLoading(false));
  };
  useEffect(load, []);

  // 启用/禁用（非运行态 → 启用；运行态 → 禁用；含 crashed 熔断后重新启用）
  const toggleState = async (plugin: InstalledPlugin) => {
    const next = plugin.state === "running" ? "disabled" : "running";
    try {
      await apiSetPluginState(plugin.id, next);
      setItems((prev) => prev.map((p) => (p.id === plugin.id ? { ...p, state: next } : p)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "操作失败");
    }
  };

  // 确认卸载（设计稿《插件卸载·SEO》：确认 → 卸载成功）
  const confirmUninstall = async () => {
    if (!uninstallTarget) return;
    setUninstalling(true);
    try {
      await apiUninstallPlugin(uninstallTarget.id);
      setUninstalled(true);
      setItems((prev) => prev.filter((p) => p.id !== uninstallTarget.id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "卸载失败");
    } finally {
      setUninstalling(false);
    }
  };

  // 提交激活许可证（M3.5：验签成功 → 提示 + 刷新列表）
  const confirmActivateLicense = async () => {
    if (!licenseTarget) return;
    setActivating(true);
    try {
      await apiActivateLicense(licenseTarget.plugin_id, licenseJwt.trim());
      setLicenseDone(true);
      load();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "激活失败");
    } finally {
      setActivating(false);
    }
  };

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">我的插件</h1>
          <p className="mt-0.5 text-xs text-ink-3">管理已安装插件 · 共 {items.length} 个</p>
        </div>
        {/* 本地上传 .bpk（M3.4：开发/验证便利通道，正式分发走 GitHub Release） */}
        <input
          ref={fileInputRef}
          type="file"
          accept=".bpk"
          className="hidden"
          onChange={(e) => onPickFile(e)}
          aria-label="选择 .bpk 安装包"
        />
        <button
          type="button"
          disabled={uploading}
          onClick={() => fileInputRef.current?.click()}
          className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
        >
          {uploading ? "安装中…" : "本地安装 .bpk"}
        </button>
      </div>

      {error && (
        <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
          {error}
        </p>
      )}
      {uploadDone && (
        <p className="mt-4 rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
          ✓ 安装包已上传并安装，可在下方列表启用
        </p>
      )}

      {loading ? (
        <div className="mt-5 space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-20 animate-pulse rounded-lg bg-muted" aria-hidden />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div className="mt-10 rounded-lg border border-line bg-elevated py-14 text-center">
          <p className="text-sm text-ink-2">还没有安装插件</p>
          <a href="/admin/plugin-market" className="mt-2 inline-block text-sm text-glow hover:underline">
            去插件商城逛逛 →
          </a>
        </div>
      ) : (
        <div className="mt-5 overflow-hidden rounded-lg border border-line bg-elevated">
          {items.map((plugin) => (
            <div
              key={plugin.id}
              className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-5 py-4 last:border-b-0"
            >
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <p className="font-medium text-ink">{plugin.name}</p>
                  <span className="text-xs text-ink-3">v{plugin.version}</span>
                  <span
                    className={`rounded-full px-2 py-0.5 text-xs ${
                      plugin.state === "disabled"
                        ? "bg-muted text-ink-3"
                        : plugin.state === "crashed"
                          ? "bg-like/10 text-like"
                          : "bg-accent-soft text-glow"
                    }`}
                  >
                    {STATE_LABEL[plugin.state] ?? plugin.state}
                  </span>
                </div>
                <p className="mt-0.5 truncate text-xs text-ink-3">
                  {plugin.repo_url || "内置"} · 安装于 {formatDateTime(plugin.created_at)}
                </p>
                {plugin.last_error && (
                  <p className="mt-0.5 truncate text-xs text-like" title={plugin.last_error}>
                    {plugin.last_error}
                  </p>
                )}
                {/* 许可证状态（M3.5：付费插件） */}
                {plugin.license && (
                  <p className="mt-0.5 text-xs">
                    {plugin.license.degraded ? (
                      <span className="text-like">许可证已过期降级（demo 模式）</span>
                    ) : plugin.license.activated ? (
                      <span className="text-glow">
                        Pro 已激活
                        {plugin.license.expires_at
                          ? ` · 至 ${new Date(plugin.license.expires_at * 1000).toLocaleDateString()}`
                          : " · 永久"}
                      </span>
                    ) : (
                      <span className="text-ink-3">demo 模式（Pro 功能未激活）</span>
                    )}
                  </p>
                )}
              </div>
              <div className="flex shrink-0 gap-2 text-xs">
                {/* 激活许可证（M3.5：付费插件 demo 或已降级时显示） */}
                {plugin.license && (!plugin.license.activated || plugin.license.degraded) && (
                  <button
                    type="button"
                    onClick={() => {
                      setLicenseTarget(plugin);
                      setLicenseJwt("");
                      setLicenseDone(false);
                      setError("");
                    }}
                    className="rounded-full border border-glow/30 px-4 py-1.5 text-glow hover:bg-accent-soft"
                  >
                    激活许可证
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => void toggleState(plugin)}
                  className="rounded-full border border-line px-4 py-1.5 text-ink-2 hover:text-ink"
                >
                  {plugin.state === "running" ? "禁用" : "启用"}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setUninstallTarget(plugin);
                    setUninstalled(false);
                    setError("");
                  }}
                  className="rounded-full border border-like/30 px-4 py-1.5 text-like hover:bg-like/10"
                >
                  卸载
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 卸载确认弹层（设计稿《插件卸载·SEO》：确认 → 成功） */}
      {uninstallTarget && (        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="卸载插件"
          onClick={() => {
            if (!uninstalling && !uninstalled) setUninstallTarget(null);
          }}
        >
          <div
            className="w-full max-w-[380px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            {uninstalled ? (
              <div className="py-4 text-center">
                <p className="text-4xl" aria-hidden>
                  ✓
                </p>
                <h2 className="mt-3 font-display text-lg font-semibold text-ink">卸载成功</h2>
                <p className="mt-1 text-xs text-ink-3">「{uninstallTarget.name}」已从站点移除</p>
                <button
                  type="button"
                  onClick={() => setUninstallTarget(null)}
                  className="mt-5 rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent hover:opacity-90"
                >
                  完成
                </button>
              </div>
            ) : (
              <>
                <h2 className="font-display text-lg font-semibold text-ink">
                  卸载「{uninstallTarget.name}」？
                </h2>
                <p className="mt-2 text-sm text-ink-2">
                  卸载后侧栏入口与设置将移除，内容不受影响。
                </p>
                {/* 能力影响清单（设计稿《插件卸载·SEO》：移除入口/清除配置/停用功能/可重装） */}
                <ul className="mt-3 space-y-1.5">
                  {["移除侧栏入口", "清除插件配置", "停用相关功能", "可随时重新安装"].map((item) => (
                    <li key={item} className="flex items-center gap-2 text-xs text-ink-2">
                      <span className="text-ink-3" aria-hidden>
                        ·
                      </span>
                      {item}
                    </li>
                  ))}
                </ul>
                <div className="mt-5 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setUninstallTarget(null)}
                    disabled={uninstalling}
                    className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
                  >
                    再想想
                  </button>
                  <button
                    type="button"
                    onClick={() => void confirmUninstall()}
                    disabled={uninstalling}
                    className="rounded-full bg-like px-5 py-2 text-sm font-medium text-white hover:opacity-90 disabled:opacity-60"
                  >
                    {uninstalling ? "卸载中…" : "确认卸载"}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* 许可证激活弹层（M3.5：输入作者签发的 license.jwt） */}
      {licenseTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-6"
          role="dialog"
          aria-modal="true"
          aria-label="激活许可证"
          onClick={() => {
            if (!activating && !licenseDone) setLicenseTarget(null);
          }}
        >
          <div
            className="w-full max-w-[420px] rounded-xl border border-line bg-elevated p-6"
            onClick={(e) => e.stopPropagation()}
          >
            {licenseDone ? (
              <div className="py-4 text-center">
                <p className="text-4xl" aria-hidden>
                  ✓
                </p>
                <h2 className="mt-3 font-display text-lg font-semibold text-ink">激活成功</h2>
                <p className="mt-1 text-xs text-ink-3">
                  「{licenseTarget.name}」已升级为 Pro，付费功能立即生效
                </p>
                <button
                  type="button"
                  onClick={() => setLicenseTarget(null)}
                  className="mt-5 rounded-full bg-accent px-8 py-2 text-sm font-medium text-on-accent hover:opacity-90"
                >
                  完成
                </button>
              </div>
            ) : (
              <>
                <h2 className="font-display text-lg font-semibold text-ink">
                  激活「{licenseTarget.name}」许可证
                </h2>
                <p className="mt-2 text-sm text-ink-2">
                  粘贴作者签发的 license.jwt（购买后由插件作者发放），验签通过后 Pro 功能启用。
                </p>
                <textarea
                  value={licenseJwt}
                  onChange={(e) => setLicenseJwt(e.target.value)}
                  rows={6}
                  placeholder='{"sub":"plugin:xxx","edition":"pro",...}'
                  className="mt-3 w-full rounded-lg border border-line bg-elevated p-3 font-mono text-xs text-ink outline-none focus:border-glow"
                />
                <div className="mt-5 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setLicenseTarget(null)}
                    disabled={activating}
                    className="rounded-full border border-line px-5 py-2 text-sm text-ink-2 hover:text-ink disabled:opacity-60"
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    onClick={() => void confirmActivateLicense()}
                    disabled={activating || licenseJwt.trim() === ""}
                    className="rounded-full bg-accent px-5 py-2 text-sm font-medium text-on-accent hover:opacity-90 disabled:opacity-60"
                  >
                    {activating ? "验签中…" : "激活"}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
