// src/components/admin/plugin-market/labels.ts
// 插件商城共享常量与格式化函数（卡片/详情弹窗/页面共用）。
// 类别标签（设计稿二级 Tab：SEO·增长/内容增强/安全/性能/分析/写作/运维）
export const CATEGORY_LABEL: Record<string, string> = {
  seo: "SEO / 增长",
  enhancement: "内容增强",
  security: "安全",
  performance: "性能",
  analytics: "分析",
  writing: "写作",
  ops: "运维",
};

// 二级类别列表（设计稿顺序）
export const CATEGORIES = ["seo", "enhancement", "security", "performance", "analytics", "writing", "ops"] as const;

// formatInstalls 安装量缩写（设计稿：12.4k）。
export function formatInstalls(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}

// categoryLabel 类别文案（未知类别回退原文）。
export function categoryLabel(category: string): string {
  return CATEGORY_LABEL[category] ?? category;
}
