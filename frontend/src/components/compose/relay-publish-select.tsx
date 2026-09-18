// src/components/compose/relay-publish-select.tsx
// 中继站发布选择（发帖/说说表单）：是否公开发送到中继站（大世界）开关，
// 勾选后选择分类与标签（标签按分类规范表多选，≤5 个）。
// 受控组件：状态由父级管理；仅当中继站已启用时由父级渲染本组件。
"use client";

import type { RelayPublishOptions } from "@/lib/api-relay";
import type { RelayPublishInput } from "@/types/api";

// relayMaxTags 中继站标签数量上限（与后端协议一致）。
const relayMaxTags = 5;

// RelayPublishSelect 中继站发布选择器。
// disabled：帖子可见性非公开时置灰（中继站是全网公开聚合，仅公开帖可发送）。
export function RelayPublishSelect({
  value,
  onChange,
  options,
  disabled,
}: {
  value: RelayPublishInput;
  onChange: (value: RelayPublishInput) => void;
  options: RelayPublishOptions;
  disabled: boolean;
}) {
  // 候选标签：当前分类的规范标签（远端自定义分类不在规范表内时为空，仅展示已选）
  const tagCandidates: string[] = options.taxonomy[value.category] ?? [];

  // togglePublish 开关切换：首次开启且未选分类时预选（意向分类 → 第一个分类）。
  const togglePublish = () => {
    if (disabled) {
      return;
    }
    if (!value.publish && value.category === "") {
      const preset = options.default_category !== "" && options.categories.includes(options.default_category)
        ? options.default_category
        : (options.categories[0] ?? "");
      onChange({ ...value, publish: true, category: preset });
      return;
    }
    onChange({ ...value, publish: !value.publish });
  };

  // selectCategory 选择分类：切换分类时清空标签（标签归属分类）。
  const selectCategory = (category: string) => {
    onChange({ ...value, category, tags: [] });
  };

  // toggleTag 标签多选切换（≤5 个）。
  const toggleTag = (tag: string) => {
    if (value.tags.includes(tag)) {
      onChange({ ...value, tags: value.tags.filter((t) => t !== tag) });
      return;
    }
    if (value.tags.length >= relayMaxTags) {
      return;
    }
    onChange({ ...value, tags: [...value.tags, tag] });
  };

  return (
    <div className={`rounded-xl border p-4 transition-colors ${value.publish ? "border-accent bg-accent-soft/40" : "border-line bg-card"} ${disabled ? "opacity-60" : ""}`}>
      {/* 开关行 */}
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-ink">🌐 公开发送到中继站（大世界）</p>
          <p className="mt-0.5 text-xs text-ink-3">
            {disabled
              ? "仅公开可见的帖子可发送到中继站"
              : "发送后将在全网博客的大世界聚合流中公开展示"}
          </p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={value.publish && !disabled}
          disabled={disabled}
          onClick={togglePublish}
          className={`h-6 w-11 shrink-0 rounded-full transition-colors ${value.publish && !disabled ? "bg-accent" : "bg-muted"}`}
        >
          <span
            className={`block h-5 w-5 rounded-full bg-white shadow transition-transform ${value.publish && !disabled ? "translate-x-[22px]" : "translate-x-0.5"}`}
          />
        </button>
      </div>

      {/* 展开区：分类 + 标签（勾选发送后显示） */}
      {value.publish && (
        <div className="mt-4 space-y-4 border-t border-line pt-4">
          {/* 分类选择（单选 chips） */}
          <div>
            <p className="text-xs font-medium text-ink-2">
              选择分类 <span className="text-ink-3">（必选，发布到中继站的哪个分类）</span>
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              {options.categories.map((category) => (
                <button
                  key={category}
                  type="button"
                  onClick={() => selectCategory(category)}
                  className={`rounded-full px-3 py-1 text-sm transition-colors ${
                    value.category === category
                      ? "bg-accent font-medium text-on-accent"
                      : "bg-muted text-ink-2 hover:text-ink"
                  }`}
                >
                  {category}
                </button>
              ))}
              {options.categories.length === 0 && (
                <span className="text-xs text-ink-3">暂无可用分类（站长未完成中继站对接）</span>
              )}
            </div>
          </div>

          {/* 标签选择（多选 chips，按分类规范表） */}
          <div>
            <p className="text-xs font-medium text-ink-2">
              选择标签 <span className="text-ink-3">（可选，最多 {relayMaxTags} 个）</span>
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              {tagCandidates.map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  className={`rounded-full px-3 py-1 text-sm transition-colors ${
                    value.tags.includes(tag)
                      ? "bg-glow/15 font-medium text-glow ring-1 ring-glow/40"
                      : "bg-muted text-ink-2 hover:text-ink"
                  }`}
                >
                  #{tag}
                </button>
              ))}
              {/* 已选标签不在候选列表（如远端自定义分类）：保留展示便于取消 */}
              {value.tags
                .filter((tag) => !tagCandidates.includes(tag))
                .map((tag) => (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => toggleTag(tag)}
                    className="rounded-full bg-glow/15 px-3 py-1 text-sm font-medium text-glow ring-1 ring-glow/40"
                  >
                    #{tag}
                  </button>
                ))}
              {tagCandidates.length === 0 && value.tags.length === 0 && (
                <span className="text-xs text-ink-3">该分类暂无规范标签，可不选</span>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
