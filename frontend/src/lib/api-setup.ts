// frontend/src/lib/api-setup.ts
// 安装向导 API 客户端：封装 /api/setup/* 接口调用与强类型定义。
//
// 说明：请求走相对路径（浏览器同源），由 Next rewrites 代理到后端 Gin 服务。
//       响应遵循统一格式 { code, message, data }，code !== 0 时抛出 SetupApiError。

// SetupStatus 安装状态查询结果。
export interface SetupStatus {
  installed: boolean;
  mode: "docker" | "manual";
  version: string;
}

// CheckItem 单项依赖检查结果。
export interface CheckItem {
  id: string;
  name: string;
  status: "ok" | "fail" | "warn" | "pending";
  detail: string;
  fixable: boolean;
}

// CheckResult 环境检查汇总。
export interface CheckResult {
  mode: "docker" | "manual";
  checks: CheckItem[];
  pass: boolean;
}

// DatabaseForm 数据库连接表单（裸机模式）。
export interface DatabaseForm {
  host: string;
  port: string;
  user: string;
  password: string;
  database: string;
}

// AdminForm 管理员账号表单。
export interface AdminForm {
  username: string;
  email: string;
  password: string;
  nickname: string;
}

// InstallResult 安装执行结果。
export interface InstallResult {
  frontend_url: string;
  admin_url: string;
  admin_username: string;
  restart: "auto" | "manual";
}

// SetupApiError 安装接口业务错误（携带后端提示文案）。
export class SetupApiError extends Error {
  public readonly code: number;

  public constructor(code: number, message: string) {
    super(message);
    this.code = code;
  }
}

// request 统一请求封装：解析统一响应体，业务失败抛 SetupApiError。
async function request<TData>(path: string, init?: RequestInit): Promise<TData> {
  const res = await fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
    cache: "no-store",
  });
  if (res.status === 503) {
    throw new SetupApiError(503, "后端服务未就绪（安装模式未启动），请稍后重试");
  }
  const body: { code: number; message: string; data: TData } = await res.json();
  if (body.code !== 0) {
    throw new SetupApiError(body.code, body.message);
  }
  return body.data;
}

// fetchStatus 查询安装状态与模式。
export function fetchStatus(): Promise<SetupStatus> {
  return request<SetupStatus>("/api/setup/status");
}

// runChecks 执行环境依赖检查。
export function runChecks(): Promise<CheckResult> {
  return request<CheckResult>("/api/setup/check", { method: "POST" });
}

// runFix 自动配置缺失依赖（返回修复后的复查结果）。
export function runFix(): Promise<CheckResult> {
  return request<CheckResult>("/api/setup/fix", { method: "POST" });
}

// testDatabase 验证并暂存数据库连接（裸机模式）。
export function testDatabase(form: DatabaseForm): Promise<Record<string, string>> {
  return request<Record<string, string>>("/api/setup/database", {
    method: "POST",
    body: JSON.stringify(form),
  });
}

// runInstall 执行安装全流程（Docker 模式数据库由编排绑定；裸机模式随请求提交）。
// siteUrl 为浏览器当前访问地址（安装完成提示的前后台地址以此为准）。
export function runInstall(
  admin: AdminForm,
  database: DatabaseForm | null,
  siteUrl: string,
): Promise<InstallResult> {
  return request<InstallResult>("/api/setup/install", {
    method: "POST",
    body: JSON.stringify({ admin, database, site_url: siteUrl }),
  });
}
