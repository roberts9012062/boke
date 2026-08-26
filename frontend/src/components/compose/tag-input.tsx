// src/components/compose/tag-input.tsx
// 标签输入（发帖中心「标签 #月色」）：# 前缀识别，回车/空格提交，失焦兜底提交。
// 从 compose 页抽出（说说/文章两种表单共用；受控组件，状态由父级管理）。
"use client";

import { useState } from "react";

import { MAX_TAGS } from "@/components/compose/config";

// TagInput 标签输入（受控：tags 当前标签列表；onChange 追加/删除后的新列表）。
export function TagInput({ tags, onChange }: { tags: string[]; onChange: (tags: string[]) => void }) {
  const [tagInput, setTagInput] = useState<string>("");
  const [hint, setHint] = useState<string>(""); // 超限提示（局部短提示，不打断输入）

  // 添加标签（去 # 前缀、去重、超上限提示）
  const addTag = (raw: string) => {
    const name = raw.trim().replace(/^#/, "");
    if (!name || tags.includes(name)) {
      setTagInput("");
      return;
    }
    if (tags.length >= MAX_TAGS) {
      setHint(`标签最多 ${MAX_TAGS} 个`);
      return;
    }
    onChange([...tags, name]);
    setTagInput("");
    setHint("");
  };

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        {tags.map((tag) => (
          <span
            key={tag}
            className="flex items-center gap-1 rounded-full bg-accent-soft px-3 py-1 text-xs text-glow"
          >
            #{tag}
            <button
              type="button"
              onClick={() => onChange(tags.filter((t) => t !== tag))}
              className="text-ink-3 hover:text-like"
              aria-label={`删除标签 ${tag}`}
            >
              ✕
            </button>
          </span>
        ))}
        <input
          type="text"
          value={tagInput}
          onChange={(e) => setTagInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              addTag(tagInput);
            }
          }}
          onBlur={() => {
            if (tagInput.trim()) {
              addTag(tagInput);
            }
          }}
          placeholder="标签 #月色"
          className="h-8 flex-1 rounded-full border border-line bg-muted px-4 text-sm text-ink placeholder:text-ink-3 focus:border-accent focus:outline-none"
        />
      </div>
      <p className="mt-1 text-xs text-ink-3">{hint || `最多 ${MAX_TAGS} 个标签，回车添加`}</p>
    </div>
  );
}
