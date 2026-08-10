// src/lib/auth.tsx
// 用户认证状态管理（React 19 客户端组件）：token 持久化、静默刷新、登录态。
//
// 实现（需求 3.1 会话约定）：
//   - access（15min）+ refresh（7d）存 localStorage
//   - 401 时用 refresh 静默刷新一次（api.ts 的 request 内自动触发）
//   - 登录/注册后拉取 /me 资料；登出调用后端撤销 refresh
//   - 页面加载时用 refresh 恢复会话（refresh 有效 → 换新令牌对）
"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

import {
  apiLogin,
  apiLogout,
  apiMe,
  apiRefresh,
  apiRegister,
  setTokenProvider,
} from "@/lib/api";
import type { AuthTokens, UserProfile } from "@/types/api";

// localStorage 键名（令牌对与当前用户资料）。
const TOKENS_KEY = "yueyan-tokens";
const PROFILE_KEY = "yueyan-profile";

// 认证上下文值：登录态、当前用户、加载状态、操作方法。
interface AuthContextValue {
  user: UserProfile | null; // 当前登录用户（未登录为 null）
  loading: boolean; // 会话恢复中（首次挂载拉取资料）
  login: (account: string, password: string) => Promise<void>; // 登录
  register: (nickname: string, email: string, password: string) => Promise<void>; // 注册
  logout: () => Promise<void>; // 登出
  updateUser: (updates: Partial<UserProfile>) => void; // 本地更新当前用户（头像/昵称等，M1.7）
}

// 创建上下文（默认值兜底）。
const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  login: async () => {},
  register: async () => {},
  logout: async () => {},
  updateUser: () => {},
});

// readTokens 从 localStorage 读取令牌对（可能为 null）。
function readTokens(): AuthTokens | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = localStorage.getItem(TOKENS_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as AuthTokens;
  } catch {
    return null;
  }
}

// persistTokens 持久化令牌对。
function persistTokens(tokens: AuthTokens): void {
  localStorage.setItem(TOKENS_KEY, JSON.stringify(tokens));
}

// clearAuth 清理全部本地认证数据（登出/会话失效）。
function clearAuth(): void {
  localStorage.removeItem(TOKENS_KEY);
  localStorage.removeItem(PROFILE_KEY);
}

// AuthProvider 认证状态提供者（客户端组件，包裹全局）。
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserProfile | null>(null);
  const [loading, setLoading] = useState<boolean>(true);

  // 令牌对内存引用（供 api.ts 同步读取，避免每次读 localStorage）
  const tokensRef = useRef<AuthTokens | null>(null);

  // ---------- 凭证存取实现（注入 api.ts） ----------
  useEffect(() => {
    setTokenProvider({
      getAccessToken: () => tokensRef.current?.access_token ?? "",
      getRefreshToken: () => tokensRef.current?.refresh_token ?? "",
      refreshTokens: async () => {
        const refresh = tokensRef.current?.refresh_token;
        if (!refresh) {
          return false;
        }
        try {
          const tokens = await apiRefresh(refresh);
          tokensRef.current = tokens;
          persistTokens(tokens);
          return true;
        } catch {
          // 刷新失败：清理会话（refresh 过期或被撤销）
          tokensRef.current = null;
          clearAuth();
          setUser(null);
          return false;
        }
      },
    });
  }, []);

  // ---------- 首次挂载：恢复会话（本地令牌 → 拉取资料） ----------
  useEffect(() => {
    const tokens = readTokens();
    if (!tokens) {
      setLoading(false);
      return;
    }
    tokensRef.current = tokens;

    // 尝试用已有 access 拉取资料；失败时由 refreshTokens 静默恢复
    apiMe()
      .then((profile) => {
        setUser(profile);
        localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
      })
      .catch(() => {
        // api.ts 内部已尝试静默刷新；仍失败则保持未登录
        setUser(null);
      })
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ---------- 登录 ----------
  const login = useCallback(async (account: string, password: string) => {
    const tokens = await apiLogin(account, password);
    tokensRef.current = tokens;
    persistTokens(tokens);
    // 拉取当前用户资料
    const profile = await apiMe();
    setUser(profile);
    localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
  }, []);

  // ---------- 注册（注册即登录） ----------
  const register = useCallback(async (nickname: string, email: string, password: string) => {
    const tokens = await apiRegister(nickname, email, password);
    tokensRef.current = tokens;
    persistTokens(tokens);
    // 拉取当前用户资料
    const profile = await apiMe();
    setUser(profile);
    localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
  }, []);

  // ---------- 登出 ----------
  const logout = useCallback(async () => {
    // 通知后端撤销 refresh token（失败不阻断本地登出）
    const refresh = tokensRef.current?.refresh_token;
    if (refresh) {
      try {
        await apiLogout(refresh);
      } catch {
        // 网络异常也继续本地登出
      }
    }
    tokensRef.current = null;
    clearAuth();
    setUser(null);
  }, []);

  // ---------- 本地更新当前用户（M1.7：头像上传/编辑资料后即时同步） ----------
  const updateUser = useCallback((updates: Partial<UserProfile>) => {
    setUser((prev) => {
      if (!prev) {
        return prev;
      }
      const next = { ...prev, ...updates };
      localStorage.setItem(PROFILE_KEY, JSON.stringify(next));
      return next;
    });
  }, []);

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout, updateUser }}>
      {children}
    </AuthContext.Provider>
  );
}

// useAuth 读取认证上下文（组件内使用）。
export function useAuth(): AuthContextValue {
  return useContext(AuthContext);
}
