// src/app/admin/roles/page.tsx
// 角色权限页（设计稿《后台角色》#96/#101）：
// 搜索角色… + 统计条（角色数/管理员/编辑/访客）+ 表格（角色|人数|权限范围|状态|创建|操作）
// + 「权限」弹层（权限域复选，settings 持久化）+「创建」按钮（内置角色提示）。
"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/modal";
import { apiAdminRoles, apiUpdateRolePermissions, ApiError, type RoleMatrixItem } from "@/lib/api";
import { DOMAIN_LABEL, formatPermissions, ROLE_LABEL } from "@/lib/rbac";

// 权限域清单（与后端 casbin.AllDomains 对齐；弹层复选选项）。
const ALL_DOMAINS = Object.keys(DOMAIN_LABEL);

// RolesPage 角色权限页。
export default function RolesPage() {
  const [items, setItems] = useState<RoleMatrixItem[]>([]);
  const [keyword, setKeyword] = useState(""); // 搜索角色（本地过滤）
  const [loaded, setLoaded] = useState(false);
  const [editing, setEditing] = useState<RoleMatrixItem | null>(null); // 权限编辑对象
  const [selected, setSelected] = useState<string[]>([]); // 弹层复选态
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [savedTip, setSavedTip] = useState("");

  // 加载角色矩阵
  useEffect(() => {
    apiAdminRoles()
      .then((r) => setItems(r.roles))
      .catch(() => setItems([]))
      .finally(() => setLoaded(true));
  }, []);

  // 打开权限编辑弹层（superadmin 不可编辑，仅查看）
  const openEdit = (item: RoleMatrixItem) => {
    setEditing(item);
    setSelected(item.permissions);
    setError("");
  };

  // 保存权限域（写 settings 持久化 + 即时生效）
  const handleSave = async () => {
    if (!editing) return;
    setBusy(true);
    setError("");
    try {
      await apiUpdateRolePermissions(editing.role, selected);
      // 本地更新 + 提示
      setItems((prev) => prev.map((x) => (x.role === editing.role ? { ...x, permissions: selected } : x)));
      setSavedTip(`「${ROLE_LABEL[editing.role as keyof typeof ROLE_LABEL] ?? editing.role}」权限已更新`);
      setTimeout(() => setSavedTip(""), 2500);
      setEditing(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  };

  // 搜索过滤（本地：角色名/中文名/权限范围）
  const filtered = items.filter((item) => {
    if (!keyword.trim()) return true;
    const k = keyword.trim().toLowerCase();
    const label = ROLE_LABEL[item.role as keyof typeof ROLE_LABEL] ?? "";
    return item.role.includes(k) || label.includes(k) || formatPermissions(item.permissions).includes(k);
  });

  // 统计条（设计稿：角色数 5 / 管理员 N / 编辑 N / 访客 —）
  const countOf = (role: string) => items.find((x) => x.role === role)?.count ?? 0;

  return (
    <div>
      {/* 页头（设计稿：角色权限 / 配置角色与访问权限矩阵） */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="font-display text-xl font-semibold text-ink">角色权限</h1>
          <p className="mt-0.5 text-xs text-ink-3">配置角色与访问权限矩阵</p>
        </div>
        {/* 创建按钮（设计稿右上角；内置角色固定，自定义后置） */}
        <button
          type="button"
          onClick={() => setError("内置 5 角色固定，自定义角色后置（差异记录）")}
          className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90"
        >
          + 创建
        </button>
      </div>

      {/* 搜索（设计稿：搜索角色…） */}
      <input
        type="text"
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
        placeholder="搜索角色…"
        className="mt-4 h-9 w-64 rounded-full border border-line bg-elevated px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
      />

      {error && <p className="mt-3 rounded-lg bg-like/10 px-3 py-2 text-sm text-ink">{error}</p>}
      {savedTip && <p className="mt-3 rounded-lg bg-accent-soft px-3 py-2 text-sm text-glow">{savedTip}</p>}

      {/* 统计条（设计稿：角色数 5 / 管理员 3 / 编辑 12 / 访客 —） */}
      <div className="mt-4 flex flex-wrap gap-3">
        {[
          { label: "角色数", value: items.length || "—" },
          { label: "超级管理员", value: countOf("superadmin") || "—" },
          { label: "编辑", value: countOf("editor") || "—" },
          { label: "访客", value: countOf("visitor") || "—" },
        ].map((s) => (
          <div key={s.label} className="rounded-lg border border-line bg-elevated px-5 py-3">
            <p className="font-display text-xl font-semibold text-ink">{s.value}</p>
            <p className="mt-0.5 text-xs text-ink-3">{s.label}</p>
          </div>
        ))}
      </div>

      {/* 角色表格（设计稿：角色/人数/权限范围/状态/创建/操作） */}
      {!loaded && <div className="mt-4 h-32 animate-pulse rounded-lg bg-muted" aria-hidden />}
      {loaded && (
        <div className="mt-4 overflow-x-auto rounded-lg border border-line bg-elevated">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead>
              <tr className="border-b border-line text-xs text-ink-3">
                <th className="px-4 py-3 font-normal">角色</th>
                <th className="px-4 py-3 font-normal">人数</th>
                <th className="px-4 py-3 font-normal">权限范围</th>
                <th className="px-4 py-3 font-normal">状态</th>
                <th className="px-4 py-3 font-normal">创建</th>
                <th className="px-4 py-3 font-normal">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {filtered.map((item) => {
                const label = ROLE_LABEL[item.role as keyof typeof ROLE_LABEL] ?? item.role;
                return (
                  <tr key={item.role} className="hover:bg-muted/40">
                    <td className="px-4 py-3">
                      <p className="font-medium text-ink">{label}</p>
                      <p className="text-xs text-ink-3">
                        {item.role === "superadmin" ? "系统内置 · 不可删除" : "内置角色"}
                      </p>
                    </td>
                    <td className="px-4 py-3 text-ink-2">{item.count}</td>
                    <td className="max-w-[260px] px-4 py-3 text-xs text-ink-2">
                      <span className="line-clamp-1">{formatPermissions(item.permissions)}</span>
                    </td>
                    <td className="px-4 py-3">
                      {item.status === "restricted" ? (
                        <span className="rounded-full bg-like/15 px-2 py-0.5 text-xs text-like">限制</span>
                      ) : (
                        <span className="rounded-full bg-accent-soft px-2 py-0.5 text-xs text-glow">启用</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs text-ink-3">系统内置</td>
                    <td className="px-4 py-3">
                      <div className="flex gap-2 text-xs">
                        {item.role === "superadmin" ? (
                          <button type="button" onClick={() => openEdit(item)} className="text-ink-2 hover:text-ink">
                            查看
                          </button>
                        ) : (
                          <button type="button" onClick={() => openEdit(item)} className="text-glow hover:underline">
                            编辑 · 权限
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-10 text-center text-sm text-ink-3">
                    没有匹配的角色
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {/* 权限矩阵弹层（设计稿「权限」入口：域复选 + 保存） */}
      <Modal
        open={editing !== null}
        title={`权限范围 · ${editing ? (ROLE_LABEL[editing.role as keyof typeof ROLE_LABEL] ?? editing.role) : ""}`}
        onClose={() => setEditing(null)}
        maxWidth="max-w-[480px]"
      >
        <p className="text-xs text-ink-3">
          {editing?.role === "superadmin" ? "超级管理员拥有全部权限，不可编辑" : "勾选该角色可访问的后台模块（保存后即时生效）"}
        </p>
        <div className="mt-3 grid grid-cols-2 gap-2">
          {ALL_DOMAINS.map((domain) => {
            const checked = selected.includes(domain);
            const disabled = editing?.role === "superadmin";
            return (
              <button
                key={domain}
                type="button"
                disabled={disabled}
                onClick={() =>
                  setSelected((prev) => (checked ? prev.filter((d) => d !== domain) : [...prev, domain]))
                }
                className={`flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors disabled:opacity-100 ${
                  checked ? "border-accent bg-accent-soft text-glow" : "border-line text-ink-2 hover:border-accent/50"
                }`}
              >
                <span
                  className={`flex h-4 w-4 items-center justify-center rounded border text-[10px] ${
                    checked ? "border-accent bg-accent text-on-accent" : "border-line"
                  }`}
                  aria-hidden
                >
                  {checked ? "✓" : ""}
                </span>
                {DOMAIN_LABEL[domain] ?? domain}
              </button>
            );
          })}
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            onClick={() => setEditing(null)}
            className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
          >
            取消
          </button>
          {editing?.role !== "superadmin" && (
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={busy}
              className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90 disabled:opacity-50"
            >
              保存
            </button>
          )}
        </div>
      </Modal>
    </div>
  );
}
