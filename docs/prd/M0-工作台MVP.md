# PRD — M0 工作台 MVP

> 状态：**已确认 v1.0**（2026-10-13 用户逐条确认 §10 五项拟定决策；依据 BRD v0.2 与 `docs/requirements/2026-10-13-M0-需求评估.md`）
> 覆盖范围：F1 工作台壳、F2 待办列表、F3 信息记录、F4 快捷入口 + 数据层约定。
> 文内标注「拟定」的决策项，评审确认后固化为本文定稿。

---

## 1. 目标与范围

**目标**：交付可日常使用的三栏工作台 MVP——待办、信息记录、快捷入口三大功能完整走通，壳与数据层为 M1+ 功能预留扩展点。

**范围内**：
- F1 三栏壳与导航、设置页（基础项）
- F2 待办列表（快速添加/完成/删除）
- F3 信息记录（Milkdown 编辑、归档、导出 .md）
- F4 快捷入口（CRUD/分组/跳转）
- 数据层：SQLite schema、版本化迁移、store 层、typed IPC 服务

**范围外（明确不做）**：
- F5 项目工作、F6 辅助区功能实现（日历/便签/消息流仅占位，见 §4.4）
- F7 数据看板、F8 自动更新、F9 AI 助手、F10 工作流引擎
- 待办属性（优先级/截止日期/项目关联）→ M1（BRD F5 关联打通后一并定）
- 数据目录迁移功能（M0 仅展示数据目录位置）

## 2. 技术架构约定

沿用 BRD §4.2/4.3 已确认的架构，M0 落地要点：

```
mygo 宿主（窗口/生命周期/WebView）
  └─ Go 应用层：Bind 服务（Todo/Note/QuickLink/App） + typed Events
       └─ store 层（唯一 SQL 所在）：SQLite + 版本化迁移
  └─ WebView 前端（React + TS 严格模式 + Vite）
       └─ mygo generate 生成的 TS 客户端（类型单一事实源，禁止手改）
```

- **服务边界**：前端不得直接访问 SQLite（不注册 mygo sqlite 的前端插件面），全部数据操作经 Bind 服务；符合 AGENTS.md「引擎边界」与「store 层唯一 SQL」约束。
- **事件约定**：数据变更后由 Go 侧 `Emit`/`Broadcast` 类型化事件（如 `todo.changed`，payload 带变更后实体或列表版本），前端据此刷新状态；事件命名 `<域>.<动作>`。
- **时间**：一律 UTC RFC3339 存储，前端本地化展示（AGENTS.md 4.1）。
- **目录结构**：按 AGENTS.md §1.3 适配为 mygo 应用本体（见 §9 脚手架约定）。

## 3. 数据模型（拟定，实现时可微调字段名，不改语义）

> 迁移文件版本化编号递增（`0001_init.sql` 起），禁止改已发布迁移；所有写操作在事务内。

```sql
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
  kind       TEXT NOT NULL,                    -- 'url' | 'path'（录入时判定）
  group_name TEXT NOT NULL DEFAULT '',        -- 分组（空=未分组）
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
```

## 4. 功能需求

### 4.1 F1 工作台壳

- 三栏布局：左侧导航（可收起/展开，记住收起状态）｜中间主功能区（65–80%）｜右侧辅助区（约 20%）。
- 左侧栏自上而下：用户信息区（头像+姓名+「{姓名}的工作台」）→ 工作台功能区（待办事项/信息记录/快捷入口）→ 项目列表区（**M0 占位**：显示「项目（M1 开放）」置灰项）→ 设置按钮。
- 主功能区随左侧选中项切换，选中状态联动；M0 默认选中「待办事项」。
- 辅助区占位：三个占位卡片（日历/便签/动态消息流，标注「M1 开放」），不做实际功能。
- 设置页（主功能区内的视图）：
  - 用户资料：姓名（文本）、头像（选择本地图片，存数据目录 copies/ 下）；
  - 主题：浅色/深色/跟随系统（拟定：M0 实现浅/深两套 CSS 变量，跟随系统）；
  - 数据目录：展示当前 SQLite 与附件所在目录 + 「打开目录」按钮（调系统文件管理器）；
  - 导出目录：选择导出目标目录（F3 用）；
  - AI 配置/自动更新：仅显示入口占位（M3/M2 开放）。

**验收标准**：
1. 三栏布局正常渲染，左栏收起/展开且刷新后保持；
2. 导航切换三个功能视图，选中态联动正确；
3. 修改姓名/头像后，左栏用户信息区即时更新（事件驱动，非整页刷新）；
4. 主题切换生效；「打开目录」能打开数据目录。

### 4.2 F2 待办列表

- 快速添加：顶部输入框，输入标题回车即添加（`Enter` 提交、`Esc` 清空）；支持快捷键（拟定 `⌘/Ctrl+K`）从任意视图聚焦到待办添加框。
- 列表：未完成在前按创建倒序，已完成折叠置底；点击复选框切换完成/恢复。
- 删除：单条删除，**二次确认**（拟定为确认对话框；软删/回收站留 P1，评审可改）。
- 空态与条目数展示。

**验收标准**：
1. 输入标题回车，新待办出现在列表且落库（重启应用后仍在）；
2. 勾选完成 → 移入已完成区、`completed_at` 写入；取消勾选恢复；
3. 删除有二次确认，确认后条目消失且不再出现在列表；
4. 多端状态（若开多窗口）经事件同步，无手动刷新依赖。

### 4.3 F3 信息记录

- 列表视图：条目卡片（标题+摘要行+更新时间），支持「全部/已归档」筛选；新建进入编辑视图。
- 编辑器：Milkdown Crepe（`@milkdown/kit` + `@milkdown/crepe` 7.22.3），`/` 菜单插入 markdown 语法；内容即 GFM markdown 纯文本，**存取零转换**。
- 自动保存：编辑停顿（拟定 2s 防抖）或失焦时保存，显示保存状态（已保存/未保存）。
- 标题：默认取正文首行（截断），可手动改名。
- 归档：条目级归档/取消归档；归档不删除。
- 导出：单条与批量导出为 `.md` 文件，写入设置中的导出目录；文件名 `{标题}-{id}.md`（重复时追加时间戳）；内容与库内 markdown 完全一致（无损）。
- 表格边界：支持 GFM 表格可视化编辑（`/` 插入、增删行列、对齐）；不支持合并单元格/单元格内嵌套块（markdown 固有限制，BRD F3 已声明）。
- 删除：单条删除 + 二次确认（与 F2 策略一致，拟定）。

**验收标准**：
1. 新建记录，`/` 菜单可插入标题/列表/表格/代码块，内容以 markdown 文本保存，重启后原样恢复；
2. 含表格的记录在编辑器内可视化编辑，导出 `.md` 后用任意编辑器打开为合法 GFM；
3. 自动保存生效且无丢失（编辑中强制退出应用，重启后最多丢失最后一个防抖窗口的内容——接受并在 UI 显示未保存态）；
4. 归档/取消归档正常，导出目录不存在时给出明确错误提示而非静默失败。

### 4.4 F4 快捷入口

- 条目字段：名称、目标（URL 或本地路径，录入时自动判定 `kind`）、分组、排序。
- 操作：添加（表单）/编辑/删除（二次确认）；分组以分隔或标签呈现（拟定：分组名内联标签 + 折叠分组头）。
- 跳转：URL → 调 mygo 桌面 API 用系统默认浏览器打开；本地路径 → 用系统「打开方式」打开。**安全边界**：仅打开用户显式录入的 `target`，禁止任何动态拼接命令；Go 侧打开实现固定为平台标准方式（mygo API 优先，兜底白名单命令 `open`/`xdg-open`/`explorer`）。

**验收标准**：
1. 添加 URL 条目后点击，系统默认浏览器打开对应网址；
2. 添加本地路径条目后点击，系统以默认应用打开（无效路径给出错误 toast，不崩溃）；
3. 分组增删与排序在重启后保持。

### 4.5 全局横切

- 错误处理：所有 RPC 调用统一经 `api/` 封装，失败 toast（含可读错误信息）；Go 侧错误经 mygo 错误机制返回结构化信息。
- 性能：常规操作（加待办、保存记录、点击入口）端到端响应 < 100ms（本地 SQLite 写入量级完全满足）。

## 5. 服务接口（Bind 服务签名，拟定）

> 以下为 Go 侧服务方法草案，`mygo generate` 生成 TS 客户端后以此为准演进；两端类型同步修改，禁止前端私自加字段（AGENTS.md 4.3）。

```go
type TodoService struct{}
func (TodoService) Add(ctx, title string) (Todo, error)
func (TodoService) List(ctx) ([]Todo, error)
func (TodoService) SetDone(ctx, id int64, done bool) (Todo, error)
func (TodoService) Delete(ctx, id int64) error
// 事件：TodoChanged（payload: 变更后的 Todo 或全量列表指纹）

type NoteService struct{}
func (NoteService) Create(ctx, title string) (Note, error)
func (NoteService) List(ctx, includeArchived bool) ([]NoteSummary, error)
func (NoteService) Get(ctx, id int64) (Note, error)
func (NoteService) Save(ctx, id int64, title, content string) (Note, error)
func (NoteService) SetArchived(ctx, id int64, archived bool) (Note, error)
func (NoteService) Delete(ctx, id int64) error
func (NoteService) Export(ctx, ids []int64) (ExportResult, error)  // 写入 settings 的导出目录
// 事件：NoteChanged

type QuickLinkService struct{}
func (QuickLinkService) Add(ctx, name, target, groupName string) (QuickLink, error)
func (QuickLinkService) List(ctx) ([]QuickLink, error)
func (QuickLinkService) Update(ctx, id int64, name, target, groupName string, sortOrder int) (QuickLink, error)
func (QuickLinkService) Delete(ctx, id int64) error
func (QuickLinkService) Open(ctx, id int64) error
// 事件：QuickLinkChanged

type AppService struct{}
func (AppService) GetSettings(ctx) (Settings, error)              // 姓名/头像/主题/导出目录/数据目录
func (AppService) UpdateProfile(ctx, name, avatarPath string) (Settings, error)
func (AppService) SetTheme(ctx, theme string) error
func (AppService) SetExportDir(ctx, dir string) error
func (AppService) OpenDataDir(ctx) error
// 事件：SettingsChanged
```

## 6. 非功能需求（M0 适用项）

| 维度 | 要求 |
|---|---|
| 数据本地化 | 全部数据存本地 SQLite + 本地文件（头像 copy、导出 .md）；无遥测、无网络请求（M0 无 AI/无更新检查） |
| 可靠性 | SQLite 事务写；应用异常退出数据无损；导出内容与库内一致 |
| 安全 | 删除/打开外部目标均有明确边界（§4.2–4.4）；密钥类内容 M0 不存在 |
| 可维护性 | TS 严格模式禁 `any`；SQL 仅 store 层；迁移版本化可从零重放 |

## 7. 测试要求

- **Go 单测**（`go test ./...`）：
  - store 层：CRUD 与迁移从零重放（内存库或临时目录）；
  - 服务层：待办完成/恢复、记录保存/导出（临时目录断言文件内容与库内一致）、快捷入口 kind 判定与非法 target 拒绝；
- **前端单测**（vitest + testing-library，选型沿用评估建议）：导航切换联动、待办增删改状态驱动、事件驱动的列表刷新；
- **E2E 主链路**（覆盖 AGENTS.md §5）：
  1. 添加待办 → 勾选完成 → 删除（含确认）；
  2. 新建记录 → 编辑含表格内容 → 归档 → 导出 .md；
  3. 添加 URL 快捷入口 → 点击跳转；
  4. 改姓名 → 左栏即时更新。

## 8. 里程碑与任务分解（对应评估工作量 11–15.5 人日）

| # | 任务 | 依赖 | 预估 |
|---|---|---|---|
| T1 | 脚手架：git（已完成基线）、`mygo-cli init`、目录结构、dev 脚本、TS 严格配置 | — | 1 |
| T2 | 数据层：迁移机制 + settings/todos/notes/quick_links 表 + store 层 + 单测 | T1 | 1.5–2 |
| T3.0 | 设计基线：`docs/design/ui-baseline.md`（用户提供的设计提示词 + M0 适用性映射）定稿 | — | 0.5 |
| T3 | F1 壳：三栏布局/导航/主题/设置页 + 辅助区占位，**按 ui-baseline 实现，先出可看界面交用户过目** | T1、T3.0 | 2–3 |
| T4 | F2 待办：服务 + 生成客户端 + UI + 单测 | T2、T3 | 1.5–2 |
| T5 | F3 信息记录：Milkdown 集成（**先行集成冒烟**）+ CRUD/归档/导出 | T2 | 3–4 |
| T6 | F4 快捷入口：CRUD/分组/跳转 | T2 | 1–1.5 |
| T7 | E2E 主链路 + `mygo build` 打包冒烟（macOS） | T3–T6 | 1–2 |

> 执行顺序（用户确认，2026-10-13，壳优先）：T1 → T3.0 → T3（空壳界面先给用户过目）→ T2 → T4–T6 → T7。每个任务一个 feature 分支；T5 的 Milkdown 集成冒烟在 T5 开始当天先做（评估风险项 3）。

## 9. 脚手架与目录约定（拟定）

`mygo-cli init` 生成结构基础上对齐 AGENTS.md §1.3 精神（Trap 业务即 mygo 应用本体，engine/ 不再单列 Go module）：

```
Trap/
├── docs/                    # 已有
├── go.mod                   # mygo 应用本体
├── main.go                  # Bind 服务 + 窗口创建
├── internal/
│   ├── service/             # Bind 服务（Todo/Note/QuickLink/App）
│   └── store/               # 唯一 SQL：SQLite 访问 + 迁移
├── migrations/              # 0001_init.sql ...
└── web/                     # React + TS 前端
    └── src/
        ├── api/             # mygo generate 产物 + 调用封装
        ├── canvas/          # （M5+ 工作流画布预留）
        └── panels/          # 待办/信息记录/快捷入口/设置/辅助区组件
```

> 与 AGENTS.md §1.3 的差异（engine/ → 应用本体、rpc/ → mygo typed IPC）源于架构决策升级（BRD 4.3），脚手架完成后同步更新 AGENTS.md §1.3（需用户确认后修改）。

## 10. 已确认决策（原「待评审确认项」，2026-10-13 用户确认）

1. 删除策略：待办/记录/入口均用**二次确认**，软删留 P1。✅
2. 辅助区 M0 仅占位、动态消息流 M0 不做 UI（仅内部事件通道）。✅
3. 自动保存防抖 2s、`⌘/Ctrl+K` 聚焦待办。✅
4. 归档 M0 为单条简单标记（无分组/合集）。✅
5. 目录结构差异（§9）与 AGENTS.md §1.3 的同步修改——T1 脚手架后确认。✅

---

> 已确认，随即进入 T1 脚手架。