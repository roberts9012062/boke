// frontend/src/app/setup/steps.tsx
// 安装向导各步骤展示组件：进度指示、依赖检查列表、数据库/管理员表单、完成页。
//
// 说明：本文件只负责展示与输入收集（受控组件），状态机与 API 调用在 page.tsx。

import type { JSX } from "react";

import type { AdminForm, CheckItem, CheckResult, DatabaseForm, InstallResult } from "@/lib/api-setup";

// StepMeta 步骤元信息（进度条渲染用）。
export interface StepMeta {
  key: string;
  label: string;
}

// STEPS 全部步骤定义（database 步骤 Docker 模式下隐藏）。
export const STEPS: readonly StepMeta[] = [
  { key: "check", label: "环境检查" },
  { key: "database", label: "数据库" },
  { key: "admin", label: "管理员" },
  { key: "done", label: "完成" },
];

// StatusIcon 检查项状态图标（ok=绿勾 warn=黄叹号 fail=红叉 pending=灰点）。
function StatusIcon({ status }: { status: CheckItem["status"] }): JSX.Element {
  const cls: Record<CheckItem["status"], string> = {
    ok: "bg-emerald-500/15 text-emerald-600",
    warn: "bg-amber-500/15 text-amber-600",
    fail: "bg-red-500/15 text-red-600",
    pending: "bg-gray-500/10 text-gray-400",
  };
  const glyph: Record<CheckItem["status"], string> = {
    ok: "✓",
    warn: "!",
    fail: "✕",
    pending: "•",
  };
  return (
    <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-sm font-bold ${cls[status]}`}>
      {glyph[status]}
    </span>
  );
}

// StepIndicator 步骤进度指示条（当前步高亮，Docker 模式跳过数据库步）。
export function StepIndicator({ current, mode }: { current: string; mode: string }): JSX.Element {
  const visible = mode === "docker" ? STEPS.filter((s) => s.key !== "database") : STEPS;
  return (
    <ol className="flex items-center justify-center gap-2 sm:gap-4">
      {visible.map((step, index) => {
        const active = step.key === current;
        const reached = visible.findIndex((s) => s.key === current) >= index;
        return (
          <li key={step.key} className="flex items-center gap-2">
            <span
              className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold transition-colors ${
                active ? "bg-ink text-white" : reached ? "bg-ink/10 text-ink" : "bg-gray-200/60 text-gray-400"
              }`}
            >
              {index + 1}
            </span>
            <span className={`text-sm ${active ? "font-semibold text-ink" : "text-ink-2"}`}>{step.label}</span>
            {index < visible.length - 1 && <span className="h-px w-6 bg-gray-300 sm:w-10" />}
          </li>
        );
      })}
    </ol>
  );
}

// CheckListPanel 环境检查面板：逐项结果 + 自动配置按钮。
export function CheckListPanel({
  result,
  fixing,
  onFix,
}: {
  result: CheckResult | null;
  fixing: boolean;
  onFix: () => void;
}): JSX.Element {
  return (
    <div className="space-y-4">
      <p className="text-sm leading-6 text-ink-2">
        正在检查服务器运行环境。存在失败项时可点击「自动配置」修复（自动创建目录、等待数据库就绪）。
      </p>
      <ul className="space-y-3">
        {(result?.checks ?? []).map((item) => (
          <li key={item.id} className="flex items-start gap-3 rounded-lg border border-gray-200/70 px-4 py-3">
            <StatusIcon status={item.status} />
            <div className="min-w-0">
              <p className="text-sm font-semibold text-ink">{item.name}</p>
              <p className="mt-0.5 break-all text-xs leading-5 text-ink-2">{item.detail}</p>
            </div>
          </li>
        ))}
        {result === null && <li className="py-6 text-center text-sm text-ink-2">正在检查环境依赖…</li>}
      </ul>
      <div className="flex justify-center">
        <button
          type="button"
          onClick={onFix}
          disabled={fixing}
          className="rounded-lg bg-ink px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-ink/85 disabled:opacity-50"
        >
          {fixing ? "正在自动配置…" : "一键自动配置"}
        </button>
      </div>
    </div>
  );
}

// Field 通用表单输入行（label + input）。
function Field({
  label,
  value,
  onChange,
  type,
  placeholder,
  hint,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type: string;
  placeholder: string;
  hint: string;
}): JSX.Element {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium text-ink">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoComplete="off"
        className="w-full rounded-lg border border-gray-300/80 bg-white px-3.5 py-2.5 text-sm text-ink outline-none transition-colors placeholder:text-gray-400 focus:border-ink/50 focus:ring-2 focus:ring-ink/10"
      />
      <span className="mt-1 block text-xs text-ink-2">{hint}</span>
    </label>
  );
}

// DatabaseStepPanel 数据库配置表单（裸机模式）。
export function DatabaseStepPanel({
  form,
  onChange,
}: {
  form: DatabaseForm;
  onChange: (next: DatabaseForm) => void;
}): JSX.Element {
  const set = (key: keyof DatabaseForm) => (v: string) => onChange({ ...form, [key]: v });
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <Field label="主机地址" value={form.host} onChange={set("host")} type="text" placeholder="127.0.0.1" hint="PostgreSQL 服务器地址" />
      <Field label="端口" value={form.port} onChange={set("port")} type="text" placeholder="5432" hint="默认 5432" />
      <Field label="用户名" value={form.user} onChange={set("user")} type="text" placeholder="postgres" hint="数据库账号" />
      <Field label="密码" value={form.password} onChange={set("password")} type="password" placeholder="••••••••" hint="数据库账号密码" />
      <div className="sm:col-span-2">
        <Field label="数据库名" value={form.database} onChange={set("database")} type="text" placeholder="boke" hint="不存在时安装过程会自动创建" />
      </div>
    </div>
  );
}

// AdminStepPanel 管理员账号表单。
export function AdminStepPanel({
  form,
  onChange,
}: {
  form: AdminForm;
  onChange: (next: AdminForm) => void;
}): JSX.Element {
  const set = (key: keyof AdminForm) => (v: string) => onChange({ ...form, [key]: v });
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <Field label="用户名" value={form.username} onChange={set("username")} type="text" placeholder="admin" hint="3-30 个字符，登录后台使用" />
      <Field label="昵称（可选）" value={form.nickname} onChange={set("nickname")} type="text" placeholder="站长" hint="缺省与用户名相同" />
      <Field label="邮箱（可选）" value={form.email} onChange={set("email")} type="email" placeholder="admin@example.com" hint="缺省自动生成，可用于找回密码" />
      <Field label="密码" value={form.password} onChange={set("password")} type="password" placeholder="至少 8 位" hint="后台登录密码，请妥善保管" />
    </div>
  );
}

// DonePanel 安装完成面板：前后台地址与后续提示。
export function DonePanel({ result }: { result: InstallResult }): JSX.Element {
  return (
    <div className="space-y-5 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/15 text-2xl text-emerald-600">✓</div>
      <div>
        <h2 className="font-display text-2xl font-bold text-ink">安装完成</h2>
        <p className="mt-1 text-sm text-ink-2">月言博客平台已成功初始化，祝你创作愉快。</p>
      </div>
      <div className="mx-auto max-w-md space-y-3 text-left">
        <a
          href={result.frontend_url}
          className="block rounded-lg border border-gray-200/80 px-4 py-3 transition-colors hover:border-ink/30"
        >
          <p className="text-xs text-ink-2">前台首页</p>
          <p className="mt-0.5 break-all text-sm font-semibold text-ink">{result.frontend_url}</p>
        </a>
        <a
          href={result.admin_url}
          className="block rounded-lg border border-gray-200/80 px-4 py-3 transition-colors hover:border-ink/30"
        >
          <p className="text-xs text-ink-2">后台管理（管理员：{result.admin_username}）</p>
          <p className="mt-0.5 break-all text-sm font-semibold text-ink">{result.admin_url}</p>
        </a>
      </div>
      <p className="text-xs leading-5 text-ink-2">
        {result.restart === "auto"
          ? "服务正在自动重启并切换到正常运行模式，约 10 秒后即可访问。"
          : "裸机部署：请重启后端服务（scripts/dev-server.sh）使新配置生效后访问。"}
      </p>
      <a href={result.frontend_url} className="inline-block rounded-lg bg-ink px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-ink/85">
        进入首页
      </a>
    </div>
  );
}
