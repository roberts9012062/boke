// src/components/world-filters.tsx
// 大世界（中继站聚合流）筛选器：分类 chips（单选）+ 标签 chips（单选）。
// 选中分类后展示该分类的规范标签；由 /world 独立页与首页「大世界」Tab 共用。
// 纯展示受控组件：筛选状态由父级管理（切换分类时父级负责清空标签）。
"use client";

// WorldFilters 大世界分类/标签筛选器。
export function WorldFilters({
  categories,
  taxonomy,
  category,
  tag,
  onCategoryChange,
  onTagChange,
}: {
  categories: string[]; // 分类候选（握手快照为权威，缺省回退规范表）
  taxonomy: Record<string, string[]>; // 分类 → 规范标签
  category: string; // 当前分类（空 = 全部）
  tag: string; // 当前标签（空 = 全部）
  onCategoryChange: (category: string) => void;
  onTagChange: (tag: string) => void;
}) {
  // 当前分类的标签候选（规范表；分类不在表内时为空则隐藏标签行）
  const tagCandidates: string[] = taxonomy[category] ?? [];

  return (
    <div className="space-y-2">
      {/* 分类筛选 */}
      <div className="flex flex-wrap gap-2" role="group" aria-label="按分类筛选">
        <FilterChip label="全部" active={category === ""} onClick={() => onCategoryChange("")} />
        {categories.map((c) => (
          <FilterChip key={c} label={c} active={category === c} onClick={() => onCategoryChange(c)} />
        ))}
      </div>

      {/* 标签筛选（选中分类后展示；无规范标签时隐藏） */}
      {category !== "" && tagCandidates.length > 0 && (
        <div className="flex flex-wrap gap-2 border-t border-line pt-2" role="group" aria-label="按标签筛选">
          <FilterChip label="全部标签" active={tag === ""} onClick={() => onTagChange("")} small />
          {tagCandidates.map((t) => (
            <FilterChip key={t} label={`#${t}`} active={tag === t} onClick={() => onTagChange(t)} small />
          ))}
        </div>
      )}
    </div>
  );
}

// FilterChip 单个筛选 chip（active 高亮；small 用于标签行的紧凑样式，缺省=false）。
function FilterChip({
  label,
  active,
  onClick,
  small,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  small?: boolean;
}) {
  const compact = small ?? false;
  return (
    <button
      type="button"
      onClick={onClick}
      className={`whitespace-nowrap rounded-full transition-colors ${
        compact ? "px-2.5 py-0.5 text-xs" : "px-3.5 py-1 text-sm"
      } ${
        active
          ? "bg-accent-soft font-medium text-glow ring-1 ring-accent/50"
          : "bg-muted text-ink-2 hover:text-ink"
      }`}
    >
      {label}
    </button>
  );
}
