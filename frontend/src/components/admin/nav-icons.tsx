// src/components/admin/nav-icons.tsx
// 后台侧栏导航图标（内联 SVG 线框图标，stroke 风格，对齐设计稿侧栏图标形态）。
// 说明：设计稿图标为线框风格（非 emoji），统一 1.5 描边、currentColor 着色。
"use client";

// 图标映射：key → SVG path（24×24 视口，stroke 线框）。
const ICON_PATHS: Record<string, string> = {
  dashboard: '<rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/>',
  posts: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>',
  comments: '<path d="M21 12a8 8 0 0 1-8 8H4l2.5-2.5A8 8 0 1 1 21 12z"/>',
  users: '<circle cx="9" cy="8" r="3.5"/><path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6"/><path d="M16 5a3.5 3.5 0 0 1 0 7"/><path d="M18.5 14.5c2 1 3.5 3 3.5 5.5"/>',
  media: '<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="9" cy="10" r="1.5"/><path d="M3 17l5-5 4 4 3-3 6 6"/>',
  tags: '<path d="M3 11V5a2 2 0 0 1 2-2h6l10 10-8 8z"/><circle cx="7.5" cy="7.5" r="1"/>',
  roles: '<path d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6z"/><path d="M9 12l2 2 4-4"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1"/>',
  plugins: '<path d="M9 4a2 2 0 1 1 3 1.7V9h6a2 2 0 1 1 0 4h-6v6a2 2 0 1 1-3 0v-6H4a2 2 0 1 1 0-4h5V5.7A2 2 0 0 1 9 4z"/>',
  market: '<path d="M6 8h12l-1 12a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2z"/><path d="M8 8V6a4 4 0 0 1 8 0v2"/>',
  seo: '<circle cx="11" cy="11" r="7"/><path d="M16.5 16.5L21 21"/><path d="M8 11l2 2 3-4"/>',
  health: '<path d="M3 12h4l2-6 4 12 2-6h6"/>',
  serp: '<path d="M4 6h16M4 10h16M4 14h16M4 18h10"/>',
  ai: '<path d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M18.4 5.6L17 7M7 17l-1.4 1.4"/><path d="M8 12a4 4 0 0 1 8 0"/><path d="M12 12l2.5 4.5h-5z"/>',
};

// NavIcon 侧栏图标组件。
// 参数：name 图标键；className 附加类（尺寸/颜色由父级控制）。
export function NavIcon({ name, className = "h-4.5 w-4.5" }: { name: string; className?: string }) {
  const path = ICON_PATHS[name] ?? ICON_PATHS.dashboard;
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden
      dangerouslySetInnerHTML={{ __html: path }}
    />
  );
}
