-- 0003_notes_evolution.sql：信息记录 v2（PRD v2.1 §3.5，灵感速记 × 笔记文档）。
--
-- 变更内容：
--   1. notes 增加 kind 列：'note'（笔记文档）| 'spark'（灵感速记）；
--      既有数据均为笔记（'note'），行为不变。
--   2. notes 增加 project_id：笔记/灵感都可关联项目（色点、组合标题、过滤用）。
--      项目删除的置空由 store 层事务显式处理（projects.go DeleteProject 同步 ——
--      见 DeleteProject v2 补丁），不依赖 SQLite FK pragma。
--   3. notes 增加 archived_at：灵感的自动归档时间（笔记文档为 NULL）；
--      sweep（见 notes_v2.go）负责 30 天自动归档、90 天清除。
--
-- 约束沿用 0001/0002：版本化迁移一经发布禁止修改；时间字段一律 UTC RFC3339。

ALTER TABLE notes ADD COLUMN kind        TEXT NOT NULL DEFAULT 'note';
ALTER TABLE notes ADD COLUMN project_id  INTEGER REFERENCES projects(id);
ALTER TABLE notes ADD COLUMN archived_at TEXT;

-- 灵感流按未处理优先查询；归档浮层按归档时间查询。
CREATE INDEX IF NOT EXISTS idx_notes_kind    ON notes (kind);
CREATE INDEX IF NOT EXISTS idx_notes_archived ON notes (archived_at);
