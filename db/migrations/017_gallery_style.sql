-- ============================================================
-- 迁移 017：帖子图片展示风格（发帖编辑器可选，发布后按风格渲染）
-- 说明：
--   gallery_style 取值：''=默认网格 grid / carousel 轮播 / flip 卡片翻转 /
--   stack 堆叠 / masonry 瀑布流 / polaroid 拍立得。
-- 幂等：可重复执行。
-- ============================================================

ALTER TABLE posts ADD COLUMN IF NOT EXISTS gallery_style VARCHAR(20) NOT NULL DEFAULT '';
COMMENT ON COLUMN posts.gallery_style IS '图片展示风格：grid/carousel/flip/stack/masonry/polaroid';
