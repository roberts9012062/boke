// src/components/admin/user-role-modal.tsx
// 用户管理 · 调整角色弹层（M5：五级角色单选 + 保存）。
// 设计：复用通用 Modal 基座；目标为自己时禁用（后端同样拦截）。
"use client";

import { useState } from "react";

import { Modal } from "@/components/ui/modal";
import { apiAdminSetUserRole, ApiError } from "@/lib/api";
import { ROLE_LABEL } from "@/lib/rbac";
import type { UserProfile } from "@/types/api";

// 角色选项（M5 五级，设计稿《后台角色》）。
const ROLE_OPTIONS: readonly { key: UserProfile["role"]; desc: string }[] = [
  { key: "superadmin", desc: "全部权限" },
  { key: "editor", desc: "内容·评论·媒体·审核" },
  { key: "author", desc: "发布·媒体上传" },
  { key: "visitor", desc: "阅读·评论（默认）" },
  { key: "restricted", desc: "只读（禁言/限流）" },
];

// UserRoleModal 调整角色弹层。
// 参数：user 目标用户；me 当前用户（自身禁用）；onClose 关闭；onChanged 成功后回调。
export function UserRoleModal({
  user,
  me,
  onClose,
  onChanged,
}: {
  user: UserProfile;
  me?: UserProfile | null;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [role, setRole] = useState<UserProfile["role"]>(user.role);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const isSelf = user.id === me?.id;

  // 保存角色（落库 + casbin 即时生效；需该用户重新登录）
  const handleSave = async () => {
    setBusy(true);
    setError("");
    try {
      await apiAdminSetUserRole(user.id, role);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "角色调整失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal open title={`调整角色 · ${user.nickname}`} onClose={onClose}>
      {isSelf && <p className="text-xs text-like">不能调整自己的角色</p>}
      <p className="mt-1 text-xs text-ink-3">变更后需该用户重新登录生效</p>
      <div className="mt-4 space-y-2">
        {ROLE_OPTIONS.map((opt) => {
          const active = role === opt.key;
          return (
            <button
              key={opt.key}
              type="button"
              disabled={isSelf}
              onClick={() => setRole(opt.key)}
              className={`w-full rounded-lg border p-3 text-left transition-colors disabled:opacity-60 ${
                active ? "border-accent bg-accent-soft" : "border-line hover:border-accent/50"
              }`}
            >
              <p className={`text-sm font-medium ${active ? "text-glow" : "text-ink"}`}>
                {ROLE_LABEL[opt.key]}
                {user.role === opt.key && <span className="ml-2 text-xs text-ink-3">当前</span>}
              </p>
              <p className="mt-0.5 text-xs text-ink-3">{opt.desc}</p>
            </button>
          );
        })}
      </div>
      {error && <p className="mt-3 rounded-md bg-like/10 px-3 py-2 text-sm text-like">{error}</p>}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          onClick={onClose}
          className="rounded-full border border-line px-4 py-1.5 text-sm text-ink-2 hover:text-ink"
        >
          取消
        </button>
        <button
          type="button"
          onClick={() => void handleSave()}
          disabled={busy || isSelf || role === user.role}
          className="rounded-full bg-accent px-4 py-1.5 text-sm text-on-accent hover:opacity-90 disabled:opacity-50"
        >
          {busy ? "保存中…" : "保存"}
        </button>
      </div>
    </Modal>
  );
}
