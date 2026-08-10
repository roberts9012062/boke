// postcss.config.mjs
// PostCSS 配置：Tailwind CSS v4 使用官方 PostCSS 插件（CSS-first 配置）。
// 注意：Tailwind v4 不再需要 tailwind.config.js，设计令牌直接在 CSS 中 @theme 定义。
export default {
  plugins: {
    "@tailwindcss/postcss": {},
  },
};
