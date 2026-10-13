-- 0001_init.sql：Trap M0 初始结构（对应 docs/prd/M0-工作台MVP.md §3）。
--
-- 约束（AGENTS.md §4.4）：
--   - 迁移文件版本化、编号递增；本文件一经发布禁止修改，结构变更一律追加 0002_*.sql。
--   - 每个迁移由迁移器在单个事务内执行（见 internal/store/store.go 的 migrate），
--     schema_migrations 表同样由迁移器创建，此文件不包含它。
--   - 时间字段一律存 UTC RFC3339 文本（AGENTS.md §4.1），前端负责本地化展示。

-- 键值设置（用户资料、主题、导出目录等）
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,          -- 如 user.name / user.avatar_path / app.theme / app.export_dir
  value      TEXT NOT NULL,
  updated_at TEXT NOT NULL              -- UTC RFC3339
);

-- 待办（M0 最小属性集：title + done；优先级/截止日/项目关联留 M1）
CREATE TABLE todos (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL,
  done         INTEGER NOT NULL DEFAULT 0,   -- 0/1
  created_at   TEXT NOT NULL,
  completed_at TEXT,                          -- 完成时写 UTC 时间，取消勾选置 NULL
  updated_at   TEXT NOT NULL
);

-- 信息记录（正文即 GFM markdown 纯文本，零转换）
CREATE TABLE notes (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  title      TEXT NOT NULL DEFAULT '',
  content    TEXT NOT NULL DEFAULT '',         -- markdown 文本
  archived   INTEGER NOT NULL DEFAULT 0,      -- 0/1，M0 归档=简单标记
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- 快捷入口
CREATE TABLE quick_links (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  target     TEXT NOT NULL,                    -- URL（http/https）或本地工具路径
  kind       TEXT NOT NULL,                    -- 'url' | 'path'（录入时判定，store 校验取值）
  group_name TEXT NOT NULL DEFAULT '',        -- 分组（空=未分组）
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
