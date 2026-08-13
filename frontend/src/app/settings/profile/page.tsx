// src/app/settings/profile/page.tsx
// 编辑资料页（设计稿 D/冷月/编辑资料 1400×900）：
// 编辑资料 → 这些信息会显示在你的个人主页 → 头像/显示名称/用户名(只读)/简介 → 保存更改。
// M1.7：头像真实上传（裁剪压缩 → /media → /me/avatar）、移除；保存后全局同步用户资料。
// 更换头像：选图后弹出圆形遮罩裁剪器（AvatarCropper），确认后上传裁剪结果。
"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { AvatarCropper } from "@/components/avatar-cropper";
import { DesktopNav } from "@/components/desktop-nav";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { Avatar } from "@/components/ui/avatar";
import { apiUpdateAvatar, apiUpdateProfile, apiUploadMedia, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

// SettingsProfilePage 编辑资料页（需登录）。
export default function SettingsProfilePage() {
  const router = useRouter();
  const { user, loading, updateUser } = useAuth();
  const [nickname, setNickname] = useState<string>("");
  const [bio, setBio] = useState<string>("");
  const [error, setError] = useState<string>("");
  const [saved, setSaved] = useState<boolean>(false);
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [avatarBusy, setAvatarBusy] = useState<boolean>(false);
  // 待裁剪的原图（非空时弹出圆形裁剪器）
  const [cropFile, setCropFile] = useState<File | null>(null);
  // 隐藏的文件选择框（更换头像触发）
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 表单回填
  useEffect(() => {
    if (user) {
      setNickname(user.nickname);
      setBio(user.bio);
    }
  }, [user]);

  // 未登录跳登录
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-6 w-32 animate-pulse rounded bg-muted" aria-hidden />
      </div>
    );
  }
  if (!user) {
    router.replace("/login");
    return null;
  }

  // 上传头像（裁剪完成后的文件）：上传媒体 → 写入用户头像 → 全局同步
  const uploadAvatar = async (croppedFile: File) => {
    setError("");
    setSaved(false);
    setAvatarBusy(true);
    try {
      const uploaded = await apiUploadMedia(croppedFile);
      await apiUpdateAvatar(uploaded.url);
      // 全局同步头像（导航/主页/评论等组件即时生效）
      updateUser({ avatar_url: uploaded.url });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "头像上传失败，请重试");
    } finally {
      setAvatarBusy(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  // 移除头像（空地址）
  const handleRemoveAvatar = async () => {
    setError("");
    setSaved(false);
    setAvatarBusy(true);
    try {
      await apiUpdateAvatar("");
      updateUser({ avatar_url: "" });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "移除头像失败，请重试");
    } finally {
      setAvatarBusy(false);
    }
  };

  // 保存（昵称 + 简介；成功后全局同步）
  const handleSave = async () => {
    setError("");
    setSaved(false);
    if (!nickname.trim()) {
      setError("昵称不能为空");
      return;
    }
    setSubmitting(true);
    try {
      await apiUpdateProfile(nickname.trim(), bio.trim());
      updateUser({ nickname: nickname.trim(), bio: bio.trim() });
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败，请稍后再试");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto w-full max-w-[640px] flex-1 px-4 py-6 pb-20">
        {/* 顶部 Tab（设计稿：资料/隐私/通知/外观/安全；M 后置修复：
            五页均已上线，由原先「M2 上线」灰按钮改为真实链接） */}
        <div className="flex gap-2">
          {[
            { key: "profile", label: "资料", href: "/settings/profile", active: true },
            { key: "privacy", label: "隐私", href: "/settings/privacy" },
            { key: "notify", label: "通知", href: "/settings/notifications" },
            { key: "appearance", label: "外观", href: "/settings/theme" },
            { key: "security", label: "安全", href: "/settings/security" },
          ].map((t) => (
            <Link
              key={t.key}
              href={t.href}
              className={`rounded-full px-4 py-1.5 text-sm transition-colors ${
                t.active
                  ? "bg-accent-soft font-medium text-glow"
                  : "text-ink-3/60 hover:text-ink"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </div>

        <h1 className="mt-4 font-display text-xl font-semibold text-ink">编辑资料</h1>
        <p className="mt-1 text-xs text-ink-3">这些信息会显示在你的个人主页</p>

        <div className="mt-6 rounded-lg border border-line bg-elevated p-6">
          {/* 头像（设计稿：建议正方形，不超过 5MB；M1.7 真实上传） */}
          <div className="flex items-center gap-4">
            <Avatar name={nickname} url={user.avatar_url} className="h-16 w-16 text-2xl" />
            <div>
              <p className="text-sm font-medium text-ink">头像</p>
              <p className="mt-0.5 text-xs text-ink-3">建议正方形，不超过 5MB</p>
              {/* 更换头像 / 移除（上传后全局生效） */}
              <div className="mt-2 flex gap-2">
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={avatarBusy}
                  className="rounded-full border border-line px-3 py-1 text-xs text-ink-2 hover:text-ink disabled:opacity-60"
                >
                  {avatarBusy ? "上传中…" : "更换头像"}
                </button>
                <button
                  type="button"
                  onClick={() => void handleRemoveAvatar()}
                  disabled={avatarBusy || !user.avatar_url}
                  className="rounded-full border border-line px-3 py-1 text-xs text-ink-3 hover:text-like disabled:opacity-50"
                >
                  移除
                </button>
                {/* 隐藏文件选择框（图片类型过滤） */}
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  aria-label="选择头像图片"
                  onChange={(e) => {
                    const file = e.target.files?.[0];
                    if (file) {
                      // 选图后弹出圆形裁剪器（确认后上传）
                      setCropFile(file);
                    }
                  }}
                />
              </div>
            </div>
          </div>

          {/* 显示名称（设计稿：林月） */}
          <div className="mt-6">
            <label htmlFor="nickname" className="mb-1.5 block text-sm text-ink-2">
              显示名称
            </label>
            <input
              id="nickname"
              type="text"
              value={nickname}
              onChange={(e) => setNickname(e.target.value)}
              maxLength={20}
              className="h-11 w-full rounded-lg border border-line bg-muted px-4 text-sm text-ink focus:border-accent focus:outline-none"
            />
          </div>

          {/* 用户名（设计稿：@linyue；MVP 只读） */}
          <div className="mt-4">
            <label htmlFor="username" className="mb-1.5 block text-sm text-ink-2">
              用户名
            </label>
            <input
              id="username"
              type="text"
              value={`@${user.username}`}
              readOnly
              className="h-11 w-full cursor-not-allowed rounded-lg border border-line bg-muted/50 px-4 text-sm text-ink-3"
            />
            <p className="mt-1 text-xs text-ink-3">用户名暂不支持修改（M2 开放）</p>
          </div>

          {/* 简介（设计稿：写一点夜里的声音…） */}
          <div className="mt-4">
            <label htmlFor="bio" className="mb-1.5 block text-sm text-ink-2">
              简介
            </label>
            <textarea
              id="bio"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              maxLength={100}
              rows={3}
              placeholder="写一点夜里的声音，和白天没说完的话。"
              className="w-full resize-none rounded-lg border border-line bg-muted px-4 py-3 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
            />
            <p className="mt-1 text-right text-xs text-ink-3">{Array.from(bio).length}/100</p>
          </div>

          {/* 提示与保存 */}
          {error && (
            <p className="mt-4 rounded-md bg-like/10 px-3 py-2 text-sm text-like" role="alert">
              {error}
            </p>
          )}
          {saved && (
            <p className="mt-4 rounded-md bg-accent-soft px-3 py-2 text-sm text-glow" role="status">
              保存成功
            </p>
          )}
          <div className="mt-6 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => router.back()}
              className="rounded-full border border-line px-6 py-2.5 text-sm text-ink-2 hover:text-ink"
            >
              取消
            </button>
            <button
              type="button"
              onClick={() => void handleSave()}
              disabled={submitting}
              className="rounded-full bg-accent px-7 py-2.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-60"
            >
              {submitting ? "保存中…" : "保存更改"}
            </button>
          </div>
        </div>
      </main>
      <MobileTabbar />

      {/* 圆形头像裁剪器（选图后弹出；确认后上传，取消关闭） */}
      {cropFile && (
        <AvatarCropper
          file={cropFile}
          onCancel={() => {
            setCropFile(null);
            if (fileInputRef.current) {
              fileInputRef.current.value = "";
            }
          }}
          onConfirm={(croppedFile) => {
            setCropFile(null);
            void uploadAvatar(croppedFile);
          }}
        />
      )}
    </div>
  );
}
