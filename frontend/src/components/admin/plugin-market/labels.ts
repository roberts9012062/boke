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

// 能力枚举 → 展示文案（与后端 plugin_capability.go 枚举一一对应；
// 市场清单 capabilities 已统一为合法枚举——安装校验与运行时门控的单一事实源）
export const CAPABILITY_LABEL: Record<string, string> = {
  hooks: "钩子扩展",
  api: "自定义 API",
  frontend: "前端扩展",
  settings: "设置项",
  "data.read": "只读数据服务",
  "admin.page": "后台独立页面",
  ai: "AI 能力",
};

// capabilityLabel 能力文案（未知枚举回退原文，兼容第三方插件自定义展示）。
export function capabilityLabel(capability: string): string {
  return CAPABILITY_LABEL[capability] ?? capability;
}

// GitHub 加速代理预置列表（前缀拼接模式：实际请求 = {代理}/{完整 GitHub URL}）。
// 面向国内网络直连 api.github.com 失败（DNS 不存在/超时）的场景；
// 各公共代理的地区可达性不同，失败请切换候选或填自定义地址（需 https:// 开头）。
export const PROXY_PRESETS: ReadonlyArray<{ value: string; label: string }> = [
  { value: "https://cors.isteed.cc", label: "cors.isteed.cc（实测可用）" },
  { value: "https://gh-proxy.com", label: "gh-proxy.com" },
  { value: "https://ghfast.top", label: "ghfast.top" },
  { value: "https://ghproxy.net", label: "ghproxy.net" },
];
