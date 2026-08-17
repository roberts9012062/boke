// src/components/admin/page-builder/ai-chat-panel.tsx
// AI 页面构建器右侧对话框（两阶段流程）：
//   第一阶段「制定方案」：用户描述需求 → AI 输出设计方案卡片（PlanCard）；
//   第二阶段「执行实现」：用户点击「按此方案生成页面」→ AI 按方案生成完整 HTML
//   文档并应用到左侧预览。用户也可继续输入调整方案（修订后再执行）。
"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { PlanCard } from "@/components/admin/page-builder/plan-card";
import { apiAiGenerateChatStream, apiAiProviders } from "@/lib/api-ai";
import type { ChatMsg } from "@/lib/api-ai";
import type { AiProviderDTO } from "@/lib/api-ai";
import { extractHtmlDocument } from "@/lib/page-html";
import { BUILD_SYSTEM_PROMPT, EXECUTE_COMMAND, PLAN_SYSTEM_PROMPT } from "@/lib/page-builder-prompts";

// BubbleKind 消息种类：普通对话 / 设计方案（第一阶段）/ 代码生成（第二阶段）。
type BubbleKind = "chat" | "plan" | "build";

// ChatBubble 对话展示条目。
interface ChatBubble {
  role: "user" | "assistant"; // 角色
  content: string; // 文本（方案为 markdown）
  kind: BubbleKind; // 种类（plan 以方案卡片渲染）
  streaming: boolean; // 是否流式生成中
  actionable: boolean; // 方案是否待执行（显示执行按钮，仅最新一条）
}

// Stage 流程阶段：空闲 → 规划中(planning) → 待确认方案(awaiting) → 生成中(building)。
type Stage = "idle" | "planning" | "awaiting" | "building";

// AiChatPanelProps 对话面板参数。
interface AiChatPanelProps {
  currentHtml: string; // 当前页面代码（手动编辑后与历史不一致时注入上下文）
  onApply: (html: string) => void; // AI 生成完成（提取出 HTML 文档后回调）
  onBusyChange?: (busy: boolean) => void; // 生成状态变化（预览遮罩提示用）
}

// AiChatPanel AI 对话面板（先计划后执行）。
export function AiChatPanel({ currentHtml, onApply, onBusyChange }: AiChatPanelProps) {
  const [models, setModels] = useState<string[]>([]);
  const [model, setModel] = useState<string>("");
  const [bubbles, setBubbles] = useState<ChatBubble[]>([]);
  const [input, setInput] = useState<string>("");
  const [stage, setStage] = useState<Stage>("idle");
  const [error, setError] = useState<string>("");
  // 对话历史（完整文本，两阶段共用；不含 system）
  const historyRef = useRef<ChatMsg[]>([]);
  const scrollRef = useRef<HTMLDivElement>(null);

  const busy = stage === "planning" || stage === "building";

  // 流式更新最后一条气泡内容
  const patchLastBubble = (patch: Partial<ChatBubble>): void => {
    setBubbles((prev) => {
      const next = [...prev];
      next[next.length - 1] = { ...next[next.length - 1], ...patch };
      return next;
    });
  };

  // 加载可用模型（启用且配置 Key 的供应商模型拍平去重）
  useEffect(() => {
    apiAiProviders()
      .then((r) => {
        const ready = (r.items ?? []).filter((p: AiProviderDTO) => p.enabled && p.api_key_set);
        const names = Array.from(new Set(ready.flatMap((p: AiProviderDTO) => p.models)));
        setModels(names);
        setModel((prev) => prev || names[0] || "");
      })
      .catch(() => undefined);
  }, []);

  // 新消息后滚动到底部
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight });
  }, [bubbles]);

  // 组装多轮消息：指定系统提示词 + 历史；手动改过代码时把当前代码作为最新 assistant 上下文
  const buildMessages = (systemPrompt: string): ChatMsg[] => {
    const messages: ChatMsg[] = [{ role: "system", content: systemPrompt }];
    const lastCode = (() => {
      for (let i = historyRef.current.length - 1; i >= 0; i--) {
        if (historyRef.current[i].role === "assistant") {
          return extractHtmlDocument(historyRef.current[i].content);
        }
      }
      return "";
    })();
    if (currentHtml && currentHtml !== lastCode) {
      messages.push({ role: "assistant", content: `当前页面代码：\n\`\`\`html\n${currentHtml}\n\`\`\`` });
    }
    messages.push(...historyRef.current);
    return messages;
  };

  // 流式调用公共封装：返回完整输出文本（错误时抛出）
  const streamOnce = async (messages: ChatMsg[], onChunk: (full: string) => void): Promise<string> => {
    let full = "";
    await apiAiGenerateChatStream({ model, messages, max_tokens: 8192 }, (chunk) => {
      full += chunk;
      onChunk(full);
    });
    return full;
  };

  // ---------- 第一阶段：制定方案 ----------
  const runPlanning = async (question: string) => {
    setError("");
    setInput("");
    setBubbles((prev) => [
      // 旧方案（若有）的执行按钮失效——始终只有最新方案可执行
      ...prev.map((b) => (b.actionable ? { ...b, actionable: false } : b)),
      { role: "user", content: question, kind: "chat", streaming: false, actionable: false },
      { role: "assistant", content: "", kind: "plan", streaming: true, actionable: false },
    ]);
    setStage("planning");
    onBusyChange?.(true);
    try {
      const full = await streamOnce(
        [...buildMessages(PLAN_SYSTEM_PROMPT), { role: "user" as const, content: question }],
        (text) => patchLastBubble({ content: text }),
      );
      historyRef.current = [
        ...historyRef.current,
        { role: "user", content: question },
        { role: "assistant", content: full },
      ];
      patchLastBubble({ content: full, streaming: false, actionable: true });
      setStage("awaiting");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "生成失败";
      setError(msg.includes("AI 设置") || msg.includes("API Key") ? `${msg}（可前往侧栏「AI 设置」配置）` : msg);
      patchLastBubble({ streaming: false }); // 停止流式状态
      setBubbles((prev) => prev.filter((b) => b.kind !== "plan" || b.content !== "")); // 清掉空方案卡片
      setStage("idle");
    } finally {
      onBusyChange?.(false);
    }
  };

  // ---------- 第二阶段：执行实现 ----------
  const runBuild = async () => {
    setError("");
    // 最新待执行方案转为已确认（不再显示按钮）
    setBubbles((prev) => {
      const next = [...prev];
      for (let i = next.length - 1; i >= 0; i--) {
        if (next[i].kind === "plan" && next[i].actionable) {
          next[i] = { ...next[i], actionable: false };
          break;
        }
      }
      next.push({ role: "assistant", content: "", kind: "build", streaming: true, actionable: false });
      return next;
    });
    setStage("building");
    onBusyChange?.(true);
    try {
      const messages = [...buildMessages(BUILD_SYSTEM_PROMPT), { role: "user" as const, content: EXECUTE_COMMAND }];
      const full = await streamOnce(messages, (text) => patchLastBubble({ content: text }));
      historyRef.current = [
        ...historyRef.current,
        { role: "user", content: EXECUTE_COMMAND },
        { role: "assistant", content: full },
      ];
      patchLastBubble({ content: full, streaming: false });
      // 提取 HTML 文档应用到预览（提取不到时保留原页面并提示）
      const html = extractHtmlDocument(full);
      if (html) {
        onApply(html);
      } else {
        setError("AI 未按约定输出 HTML 代码块，请重试或调整方案");
      }
      setStage("idle");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "生成失败";
      setError(msg.includes("AI 设置") || msg.includes("API Key") ? `${msg}（可前往侧栏「AI 设置」配置）` : msg);
      patchLastBubble({ streaming: false }); // 停止流式状态
      setBubbles((prev) => prev.filter((b) => b.kind !== "build" || b.content !== "")); // 清掉空生成提示
      setStage("idle");
    } finally {
      onBusyChange?.(false);
    }
  };

  // 发送（Enter）：进入方案规划
  const handleSend = async () => {
    const question = input.trim();
    if (!question || busy) {
      return;
    }
    if (!model) {
      setError("请先在「AI 设置」中启用供应商并配置 API Key");
      return;
    }
    await runPlanning(question);
  };

  return (
    <div className="flex h-full w-full flex-col rounded-lg border border-line bg-elevated">
      {/* 头部：流程提示 + 模型选择 */}
      <div className="flex items-center gap-2 border-b border-line px-4 py-2.5">
        <span className="text-sm font-medium text-ink">AI 对话</span>
        <span className="text-[10px] text-ink-3">先出方案 · 确认后生成</span>
        <select
          value={model}
          onChange={(e) => setModel(e.target.value)}
          className="ml-auto h-8 max-w-[170px] rounded-lg border border-line bg-muted px-2 text-xs text-ink-2 focus:border-accent focus:outline-none"
        >
          {models.length === 0 && <option value="">未配置模型</option>}
          {models.map((m) => (
            <option key={m} value={m}>
              {m}
            </option>
          ))}
        </select>
      </div>

      {/* 消息列表 */}
      <div ref={scrollRef} className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
        {bubbles.length === 0 && (
          <div className="rounded-lg bg-muted/60 px-3 py-4 text-xs leading-relaxed text-ink-3">
            <p className="mb-2 text-ink-2">告诉 AI 你想要什么页面，它会先给出设计方案，确认后再生成：</p>
            <p>· 「做一个友情链接页，卡片网格布局，展示名称/简介/头像占位」</p>
            <p>· 「生成一个关于页：个人介绍 + 时间线 + 联系方式」</p>
            <p>· 「把当前页面的配色改成冷色调」</p>
            {models.length === 0 && (
              <p className="mt-3 text-like">
                未检测到可用的 AI 模型，
                <Link href="/admin/ai" className="underline">
                  前往 AI 设置
                </Link>
                启用供应商并配置 API Key 后即可使用。
              </p>
            )}
          </div>
        )}
        {bubbles.map((b, i) =>
          b.role === "user" ? (
            <div key={i} className="ml-auto max-w-[92%] rounded-lg bg-accent-soft px-3 py-2 text-sm leading-relaxed text-ink">
              {b.content}
            </div>
          ) : b.kind === "plan" ? (
            <PlanCard
              key={i}
              content={b.content}
              streaming={b.streaming}
              actionable={b.actionable}
              executing={stage === "building"}
              onExecute={() => void runBuild()}
            />
          ) : (
            /* build 阶段：代码流不刷屏，提示语引导看左侧预览 */
            <div key={i} className="max-w-[92%] rounded-lg bg-muted px-3 py-2 text-sm leading-relaxed text-ink-2">
              {b.streaming
                ? "正在按方案生成代码，完整效果将实时更新到左侧预览…"
                : "已按方案生成页面 ✓ 效果见左侧预览，可继续输入调整（会先更新方案）"}
            </div>
          ),
        )}
      </div>

      {error && (
        <p className="mx-4 mb-2 rounded-md bg-like/10 px-3 py-2 text-xs text-like" role="alert">
          {error}
        </p>
      )}

      {/* 输入区 */}
      <div className="border-t border-line p-3">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            // Enter 发送 / Shift+Enter 换行
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              void handleSend();
            }
          }}
          rows={3}
          placeholder="描述你想要的页面…（AI 会先给出设计方案）"
          className="w-full resize-none rounded-lg border border-line bg-muted px-3 py-2 text-sm text-ink focus:border-accent focus:outline-none"
        />
        <div className="mt-2 flex justify-end">
          <button
            type="button"
            onClick={() => void handleSend()}
            disabled={busy || !input.trim()}
            className="rounded-full bg-accent px-6 py-1.5 text-sm font-medium text-on-accent transition-opacity hover:opacity-90 disabled:opacity-50"
          >
            {stage === "planning" ? "规划中…" : busy ? "生成中…" : "发送"}
          </button>
        </div>
      </div>
    </div>
  );
}
