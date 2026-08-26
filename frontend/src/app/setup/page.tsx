// frontend/src/app/setup/page.tsx
// 安装向导主页面：环境检查 → 数据库配置（Docker 模式自动跳过）→ 管理员创建 →
// 执行安装 → 展示前后台访问地址。
//
// 状态机：check → database（manual 模式）→ admin → installing → done
// 说明：已安装时本页自动跳回首页；未安装时全站其他页面由 middleware 引导至此。

"use client";

import { useCallback, useEffect, useState, type JSX } from "react";
import { useRouter } from "next/navigation";

import {
  fetchStatus,
  runChecks,
  runFix,
  runInstall,
  testDatabase,
  SetupApiError,
  type AdminForm,
  type CheckResult,
  type DatabaseForm,
  type InstallResult,
} from "@/lib/api-setup";
import {
  AdminStepPanel,
  CheckListPanel,
  DatabaseStepPanel,
  DonePanel,
  StepIndicator,
} from "./steps";

// Step 向导步骤状态。
type Step = "check" | "database" | "admin" | "installing" | "done";

// 数据库表单默认值（常见本地部署参数，减少输入成本）。
const EMPTY_DB: DatabaseForm = { host: "127.0.0.1", port: "5432", user: "postgres", password: "", database: "boke" };

// 管理员表单默认值。
const EMPTY_ADMIN: AdminForm = { username: "admin", email: "", password: "", nickname: "" };

// SetupPage 安装向导页面。
export default function SetupPage() {
  const router = useRouter();

  // 页面与流程状态
  const [step, setStep] = useState<Step>("check");
  const [mode, setMode] = useState<string>("manual");
  const [loading, setLoading] = useState<boolean>(true);
  const [busy, setBusy] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [checkResult, setCheckResult] = useState<CheckResult | null>(null);
  const [fixing, setFixing] = useState<boolean>(false);
  const [dbForm, setDbForm] = useState<DatabaseForm>(EMPTY_DB);
  const [dbVerified, setDbVerified] = useState<boolean>(false);
  const [adminForm, setAdminForm] = useState<AdminForm>(EMPTY_ADMIN);
  const [installResult, setInstallResult] = useState<InstallResult | null>(null);

  // 初始化：查询安装状态（已安装跳回首页），随后自动执行环境检查
  useEffect(() => {
    let cancelled = false;
    const init = async (): Promise<void> => {
      try {
        const status = await fetchStatus();
        if (cancelled) return;
        if (status.installed) {
          router.replace("/");
          return;
        }
        setMode(status.mode);
        const result = await runChecks();
        if (!cancelled) setCheckResult(result);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "初始化失败");
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    void init();
    return () => {
      cancelled = true;
    };
  }, [router]);

  // handleFix 一键自动配置：修复后刷新检查结果
  const handleFix = useCallback(async (): Promise<void> => {
    setFixing(true);
    setError("");
    try {
      const result = await runFix();
      setCheckResult(result);
    } catch (err) {
      setError(err instanceof SetupApiError ? err.message : "自动配置失败");
    } finally {
      setFixing(false);
    }
  }, []);

  // handleRecheck 重新检查
  const handleRecheck = useCallback(async (): Promise<void> => {
    setBusy(true);
    setError("");
    try {
      setCheckResult(await runChecks());
    } catch (err) {
      setError(err instanceof SetupApiError ? err.message : "检查失败");
    } finally {
      setBusy(false);
    }
  }, []);

  // handleDatabase 提交数据库配置：验证通过后进入管理员步骤
  const handleDatabase = useCallback(async (): Promise<void> => {
    setBusy(true);
    setError("");
    try {
      await testDatabase(dbForm);
      setDbVerified(true);
      setStep("admin");
    } catch (err) {
      setError(err instanceof SetupApiError ? err.message : "数据库验证失败");
    } finally {
      setBusy(false);
    }
  }, [dbForm]);

  // handleInstall 提交管理员并执行安装
  const handleInstall = useCallback(async (): Promise<void> => {
    if (adminForm.username.trim().length < 3 || adminForm.password.length < 8) {
      setError("用户名至少 3 个字符，密码至少 8 位");
      return;
    }
    setBusy(true);
    setError("");
    setStep("installing");
    try {
      const database = mode === "docker" ? null : dbForm;
      const result = await runInstall(
        { ...adminForm, username: adminForm.username.trim() },
        database,
        window.location.origin,
      );
      setInstallResult(result);
      setStep("done");
    } catch (err) {
      setError(err instanceof SetupApiError ? err.message : "安装失败，请检查后重试");
      setStep("admin");
    } finally {
      setBusy(false);
    }
  }, [adminForm, dbForm, mode]);

  // 步骤进度键（installing 归入 admin→done 过渡，进度条显示在「管理员」步）
  const progressKey: Step = step === "installing" ? "admin" : step;

  // 渲染各步骤主体内容
  const renderBody = (): JSX.Element => {
    if (loading) {
      return <p className="py-10 text-center text-sm text-ink-2">正在加载安装向导…</p>;
    }
    if (step === "check") {
      return <CheckListPanel result={checkResult} fixing={fixing} onFix={() => void handleFix()} />;
    }
    if (step === "database") {
      return (
        <div className="space-y-4">
          <p className="text-sm leading-6 text-ink-2">填写现有 PostgreSQL 数据库连接信息，安装程序将验证连接并自动建库建表。</p>
          <DatabaseStepPanel form={dbForm} onChange={setDbForm} />
        </div>
      );
    }
    if (step === "admin" || step === "installing") {
      return (
        <div className="space-y-4">
          {mode === "docker" && (
            <p className="rounded-lg bg-emerald-500/10 px-4 py-2.5 text-sm text-emerald-700">
              Docker 部署：数据库已自动绑定，无需填写连接信息。
            </p>
          )}
          {mode === "manual" && dbVerified && (
            <p className="rounded-lg bg-emerald-500/10 px-4 py-2.5 text-sm text-emerald-700">
              数据库连接已验证（{dbForm.host}:{dbForm.port}/{dbForm.database}）。
            </p>
          )}
          <AdminStepPanel form={adminForm} onChange={setAdminForm} />
          {step === "installing" && (
            <p className="text-center text-sm text-ink-2">正在执行安装（建库建表、初始化数据、创建管理员）…</p>
          )}
        </div>
      );
    }
    if (step === "done" && installResult) {
      return <DonePanel result={installResult} />;
    }
    return <p className="py-10 text-center text-sm text-ink-2">未知状态，请刷新页面重试。</p>;
  };

  // 下一步是否可用（check 步骤要求 pass；database 已验证）
  const canNext: boolean =
    step === "check" ? (checkResult?.pass ?? false) : step === "database" ? dbVerified : false;

  // 点击下一步（check → database/admin）
  const handleNext = useCallback((): void => {
    if (!canNext) return;
    setStep(mode === "docker" ? "admin" : "database");
  }, [canNext, mode]);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-4 py-10">
      <section className="w-full max-w-2xl rounded-2xl border border-gray-200/70 bg-white/90 p-6 shadow-sm sm:p-10">
        {/* 品牌与标题 */}
        <header className="mb-8 text-center">
          <h1 className="font-display text-3xl font-bold text-ink">月言 · 安装向导</h1>
          <p className="mt-1.5 text-sm text-ink-2">首次部署引导：几分钟完成初始化</p>
        </header>

        {/* 步骤进度（完成页隐藏） */}
        {step !== "done" && (
          <div className="mb-8">
            <StepIndicator current={progressKey} mode={mode} />
          </div>
        )}

        {/* 主体内容 */}
        <div className="min-h-[220px]">{renderBody()}</div>

        {/* 错误提示 */}
        {error && (
          <p className="mt-4 rounded-lg bg-red-500/10 px-4 py-2.5 text-sm text-red-600">{error}</p>
        )}

        {/* 操作按钮区（安装中/完成页隐藏） */}
        {step !== "done" && step !== "installing" && (
          <footer className="mt-8 flex items-center justify-between">
            <div className="flex gap-2">
              {step === "check" && (
                <button
                  type="button"
                  onClick={() => void handleRecheck()}
                  disabled={busy || fixing}
                  className="rounded-lg border border-gray-300/80 px-4 py-2.5 text-sm text-ink-2 transition-colors hover:border-ink/30 hover:text-ink disabled:opacity-50"
                >
                  重新检查
                </button>
              )}
              {step !== "check" && (
                <button
                  type="button"
                  onClick={() => setStep(step === "admin" && mode === "manual" ? "database" : "check")}
                  disabled={busy}
                  className="rounded-lg border border-gray-300/80 px-4 py-2.5 text-sm text-ink-2 transition-colors hover:border-ink/30 hover:text-ink disabled:opacity-50"
                >
                  上一步
                </button>
              )}
            </div>
            <div className="flex gap-2">
              {step === "check" && (
                <button
                  type="button"
                  onClick={handleNext}
                  disabled={!canNext}
                  className="rounded-lg bg-ink px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-ink/85 disabled:opacity-50"
                >
                  下一步
                </button>
              )}
              {step === "database" && (
                <button
                  type="button"
                  onClick={() => void handleDatabase()}
                  disabled={busy}
                  className="rounded-lg bg-ink px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-ink/85 disabled:opacity-50"
                >
                  {busy ? "验证中…" : "验证并继续"}
                </button>
              )}
              {step === "admin" && (
                <button
                  type="button"
                  onClick={() => void handleInstall()}
                  disabled={busy}
                  className="rounded-lg bg-ink px-6 py-2.5 text-sm font-medium text-white transition-colors hover:bg-ink/85 disabled:opacity-50"
                >
                  {busy ? "安装中…" : "开始安装"}
                </button>
              )}
            </div>
          </footer>
        )}
      </section>
    </main>
  );
}
