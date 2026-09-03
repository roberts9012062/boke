// src/components/admin/open-api/endpoint-catalog.tsx
// 开放接口目录多选列表（后台「接口开放」页面上半部）：
// 全部可开放接口以 checkbox 列出（名称/方法/路径/描述/参数），供生成 Key 时勾选。
"use client";

import type { CatalogEntry } from "@/lib/api-openapi";

// EndpointCatalog 目录多选列表（受控组件）。
// 参数：entries 目录数据；selected 已勾选的接口标识集合；onToggle 单项切换；onToggleAll 全选/清空。
export function EndpointCatalog({
  entries,
  selected,
  onToggle,
  onToggleAll,
}: {
  entries: CatalogEntry[];
  selected: Set<string>;
  onToggle: (endpoint: string) => void;
  onToggleAll: (checked: boolean) => void;
}) {
  const allChecked = entries.length > 0 && entries.every((e) => selected.has(e.endpoint));

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-elevated">
      {/* 表头：全选 + 计数 */}
      <div className="flex items-center gap-3 border-b border-line px-4 py-3">
        <input
          type="checkbox"
          checked={allChecked}
          onChange={(e) => onToggleAll(e.target.checked)}
          className="h-3.5 w-3.5"
          aria-label="全选接口"
        />
        <span className="text-sm font-medium text-ink">
          全部接口（已选 {selected.size}/{entries.length}）
        </span>
      </div>

      {/* 接口清单 */}
      <ul className="divide-y divide-line">
        {entries.map((entry) => {
          const checked = selected.has(entry.endpoint);
          return (
            <li key={entry.endpoint}>
              <label className="flex cursor-pointer items-start gap-3 px-4 py-3 transition-colors hover:bg-muted">
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => onToggle(entry.endpoint)}
                  className="mt-1 h-3.5 w-3.5 shrink-0"
                  aria-label={`选择接口 ${entry.name}`}
                />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium text-ink">{entry.name}</span>
                    {/* 方法徽标（GET / POST） */}
                    <span className="rounded bg-accent-soft px-1.5 py-0.5 text-[10px] font-medium text-glow">
                      {entry.method}
                    </span>
                    <code className="break-all rounded bg-muted px-1.5 py-0.5 text-xs text-ink-2">
                      {entry.path}
                    </code>
                    {/* 来源徽标：插件声明的接口随插件安装/升级自动出现 */}
                    {entry.source === "plugin" && (
                      <span
                        className="rounded border border-line px-1.5 py-0.5 text-[10px] text-ink-3"
                        title={`由插件「${entry.plugin_name ?? "未知插件"}」声明，随插件安装/升级自动增删`}
                      >
                        插件 · {entry.plugin_name ?? ""}
                      </span>
                    )}
                  </div>
                  <p className="mt-0.5 text-xs text-ink-3">{entry.description}</p>
                  {/* 参数说明（有参数时折叠为一行摘要） */}
                  {entry.params.length > 0 && (
                    <p className="mt-0.5 text-[11px] text-ink-3">
                      参数：{entry.params.map((p) => `${p.name}${p.required ? "*" : ""}`).join("、")}
                    </p>
                  )}
                </div>
              </label>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
