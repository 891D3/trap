-- 0002_projects_todos.sql：PRD v2.0（Ardot 设计稿驱动重构）数据层扩展。
--
-- 变更内容（docs/prd/M0-工作台MVP.md §3 v2.0）：
--   1. 新增 projects 表：项目最小实体（仅做待办关联与统计，不做项目工作视图）；
--      color 存前端令牌名（slate/amber/green/purple，映射 ui-baseline v3 §三色板）。
--   2. todos 扩展三列：
--      - category     分类（'' | personal | work | study，对应设计稿 个人/工作/学习）；
--      - due_date     截止日期时间，UTC RFC3339；NULL = 未安排；
--      - project_id   关联项目；项目删除时由 store 层在事务内显式置 NULL（见
--        projects.go DeleteProject），不依赖 SQLite FK 级联 pragma 的开启状态。
--
-- 约束沿用 0001：版本化迁移一经发布禁止修改；时间字段一律 UTC RFC3339 文本。

CREATE TABLE projects (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT 'slate',   -- 前端色令牌名（store 校验取值）
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- 已有数据行为保持不变：所有现存待办 = 未分类、未安排、无项目。
ALTER TABLE todos ADD COLUMN category   TEXT NOT NULL DEFAULT '';
ALTER TABLE todos ADD COLUMN due_date   TEXT;
ALTER TABLE todos ADD COLUMN project_id INTEGER REFERENCES projects(id);
