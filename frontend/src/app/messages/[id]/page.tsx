// src/app/messages/[id]/page.tsx
// 私信会话详情页（设计稿 M/冷月/消息 390：会话气泡 + 写消息… 发送）：
// 移动端进入会话后的全屏对话视图；桌面直接访问同构。
"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesktopNav } from "@/components/desktop-nav";
import { ChatView } from "@/components/message/chat-view";
import { MobileTabbar } from "@/components/mobile-tabbar";
import { apiConversations } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import type { ConversationDTO } from "@/types/api";

// MessageDetailPage 私信会话详情（需登录；会话不存在显示提示）。
export default function MessageDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const conversationId = Number(params.id);
  const { user, loading } = useAuth();
  const [conversation, setConversation] = useState<ConversationDTO | null>(null);
  const [loaded, setLoaded] = useState<boolean>(false);

  // 从会话列表中找到当前会话（列表接口返回全部会话）
  useEffect(() => {
    if (loading) {
      return;
    }
    if (!user) {
      router.replace("/login");
      return;
    }
    apiConversations()
      .then((r) => {
        const found = r.items.find((c) => c.id === conversationId);
        setConversation(found ?? null);
      })
      .catch(() => setConversation(null))
      .finally(() => setLoaded(true));
  }, [loading, user, conversationId, router]);

  return (
    <div className="flex min-h-screen flex-col">
      <DesktopNav />
      <main className="mx-auto flex w-full max-w-[720px] flex-1 flex-col px-4 py-4 pb-20">
        {/* 返回（移动端会话页头部） */}
        <div className="mb-3 flex items-center gap-2">
          <Link href="/messages" className="text-sm text-ink-3 hover:text-ink" aria-label="返回消息列表">
            ← 消息
          </Link>
        </div>

        {!loaded && <div className="h-64 animate-pulse rounded-lg bg-muted" aria-hidden />}

        {loaded && !conversation && (
          <div className="py-16 text-center">
            <p className="text-sm text-ink-2">会话不存在或已删除</p>
            <Link href="/messages" className="mt-3 inline-block text-sm text-glow hover:underline">
              返回消息列表 →
            </Link>
          </div>
        )}

        {loaded && conversation && (
          <div className="h-[calc(100vh-11rem)] overflow-hidden rounded-lg border border-line bg-elevated">
            <ChatView conversation={conversation} />
          </div>
        )}
      </main>
      <MobileTabbar />
    </div>
  );
}
