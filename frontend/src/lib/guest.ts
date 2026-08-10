// src/lib/guest.ts
// 匿名评论身份管理（需求 3.5：开放评论无需登录，访客自填昵称 + 匿名 token）。
//
// 流程：访客首次评论时弹出昵称输入（可选，默认「匿名访客」+ 随机后缀），
// 调 /guest-identity 签发 token，localStorage 持久化（7 天有效，服务端内存管理）。
"use client";

import { apiGuestIdentity } from "@/lib/api";
import type { GuestIdentity } from "@/types/api";

// localStorage 键名。
const GUEST_KEY = "yueyan-guest";

// readGuest 读取本地匿名身份（可能为 null）。
export function readGuest(): GuestIdentity | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = localStorage.getItem(GUEST_KEY);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as GuestIdentity;
  } catch {
    return null;
  }
}

// persistGuest 持久化匿名身份。
export function persistGuest(guest: GuestIdentity): void {
  localStorage.setItem(GUEST_KEY, JSON.stringify(guest));
}

// clearGuest 清理匿名身份。
export function clearGuest(): void {
  localStorage.removeItem(GUEST_KEY);
}

// ensureGuest 确保存在匿名身份：已存在直接返回；否则签发新身份。
// 参数：nickname 自填昵称（空 = 默认匿名访客 + 随机后缀）。
// 返回：匿名身份（签发失败时返回 null，由调用方提示）。
export async function ensureGuest(nickname: string): Promise<GuestIdentity | null> {
  // 已有本地身份直接复用（防刷限频以 token 为维度）
  const existing = readGuest();
  if (existing) {
    return existing;
  }
  try {
    const guest = await apiGuestIdentity(nickname);
    persistGuest(guest);
    return guest;
  } catch {
    return null;
  }
}
