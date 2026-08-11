-- 007_tags_category.sql
-- 标签分类列（M2.9 设计稿纠偏：《后台标签》画板「分类」列：情绪/栏目/体裁/临时）。
-- 幂等，可重复执行。
ALTER TABLE tags ADD COLUMN IF NOT EXISTS category VARCHAR(50) NOT NULL DEFAULT '';
