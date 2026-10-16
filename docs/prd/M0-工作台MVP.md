# PRD — M0 工作台 MVP

> 状态：**v2.1（含 M0.5 信息记录 v2 章节，设计驱动 2026-10-16）**（2026-10-15，按 Ardot 设计稿 `WorkDesktop Project` v1.0 重订范围；样式与功能结构以 `docs/design/ui-baseline.md` v3 为准）。v1.1 记录见本文 §11 历史版本。
> 覆盖范围：F1 工作台壳（重构）、F0 数据看板（新）、F2 待办列表（扩展：项目关联/日期/分类）、F3 信息记录（换肤）、F4 快捷入口（换肤）+ 数据层约定。
> v2.0 变更依据：用户 2026-10-15 决定「先通过 Web 页面设计确定样式和功能，再进入实质性开发」；设计稿含 Dashboard 与待办两张功能页 + 全套设计规范（8 个 Section）。
> 文内标注「拟定」的决策项，评审确认后固化为本文定稿。

---

## 1. 目标与范围

**目标**（v2.0）：重构至 Ardot 设计稿形态——数据看板（Dashboard）、扩展型待办（项目关联/日期/分类）、信息记录、快捷入口四视图完整走通，项目列表入侧边栏，暗色可用。

**范围内**：
- F1 三栏壳重构：侧边栏（用户区/菜单5项/项目列表/设置）+ 顶栏（标题面包屑/搜索框/操作钮）+ 辅助区（随视图切换内容）+ 暗色模式
- F0 数据看板（新视图，v2 新增）：KPI 卡 × 4 + 待办事项卡 + 今日信息卡；辅助区映射「今日聚焦」
- F2 待办列表扩展：快速创建卡（多行输入 + #项目联想 + 时间解析 chip + 分类单选）、按日期分组列表、行内编辑态；辅助区映射「月历/按属性看/按项目查看/即将到来」
- 项目概念（v2 新增，最小化）：项目实体（名称/颜色/排序），待办可关联项目；侧边栏项目列表（色点+名称+计数+新建按钮）；项目仅做关联与统计，不做项目工作视图（F5 实体功能仍留 M1）
- F3 信息记录：功能沿用 v1.1（Milkdown 编辑/归档/导出），视觉按 v3 基线换肤
- F4 快捷入口：功能沿用 v1.1（CRUD/分组/跳转），视觉按 v3 基线换肤
- 数据层：SQLite schema（新增 projects 表与 todos 扩展字段）、版本化迁移、store 层、typed IPC 服务

**范围外（明确不做）**：
- F5 项目工作视图（事项明细/进度跟踪仍留 M1；v2 仅提供项目实体与关联）
- F6 辅助区旧占位形态取消：辅助区改为随视图映射的真实内容（日历组件仅服务于待办未月历过滤）；独立便签/消息流仍不做
- F8 自动更新、F9 AI 助手、F10 工作流引擎
- F11 MCP 接入能力（M3 阶段实施，M0 仅在设置页保留入口占位）
- 待办优先级字段（设计稿未出现，留 M1）；循环待办/提醒推送
- 数据目录迁移功能（M0 仅展示数据目录位置）
- 股票看台：导航占位项，视图 M2+ 开放
- 全局搜索后端（顶栏搜索框仅样式占位）

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

> 迁移文件版本化编号递增（`0001_init.sql` 起，已在发布了 0001；增量修订用新迁移文件如 `0002_projects_todos.sql`），禁止改已发布迁移；所有写操作在事务内。

```sql
-- v2 新增：项目（最小实体：仅做关联与统计，无事项明细/进度）
CREATE TABLE projects (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL,                  -- 令牌名如 'slate'/'warning'/'green'/'purple'（UI 端映射色值）
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

-- 键值设置（用户资料、主题、导出目录等）
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,          -- 如 user.name / user.avatar_path / app.theme / app.export_dir
  value      TEXT NOT NULL,
  updated_at TEXT NOT NULL              -- UTC RFC3339
);

-- 待办（v2 扩展：+日期/时间/分类/项目关联；优先级仍留 M1）
CREATE TABLE todos (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL,
  done         INTEGER NOT NULL DEFAULT 0,   -- 0/1
  category     TEXT NOT NULL DEFAULT '',      -- '' | 'personal' | 'work' | 'study'（设计稿：个人/工作/学习）
  due_date     TEXT,                           -- UTC RFC3339 日期或日期时间；NULL=未安排
  project_id   INTEGER REFERENCES projects(id) ON DELETE SET NULL,
  created_at   TEXT NOT NULL,
  completed_at TEXT,                          -- 完成时写 UTC 时间，取消勾选置 NULL
  updated_at   TEXT NOT NULL                  -- 颜色分配：新建项目时按主色/语义色梯度顺序循环分配，可修改
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

## 3.5 M0.5 信息记录 v2（灵感速记 × 笔记文档，2026-10-16 新增）

> 设计来源：Ardot 35:1（笔记文档态）/ 39:41（灵感速记态）；样式规格见 ui-baseline v3 §5.4a–c。

### 3.5.1 数据模型（新迁移 0003_notes_evolution.sql）

```sql
ALTER TABLE notes ADD COLUMN kind        TEXT NOT NULL DEFAULT 'note';  -- 'note' | 'spark'
ALTER TABLE notes ADD COLUMN project_id  INTEGER REFERENCES projects(id);
ALTER TABLE notes ADD COLUMN archived_at TEXT;                         -- 灵感归档时间（笔记沿用原 archived 标记）
-- 索引：kind/archived 查询
```

- 灵感（spark）属性集：`content`（摘要正文，纯文本）+ `project_id`；进「未处理」流。
- 自动归档口径：未处理灵感 `updated_at` 距今 > 30 天 → 自动归档（`archived_at` 写入）；归档保留 90 天后可清除（M0.5 仅实现归档与恢复，90 天后清除由启动 sweep 顺带删除）。
- 标签：不落库，从笔记正文解析 `#标签` token 实时统计（保持零迁移成本，PRD v2.0 §3 决策 4 口径延续）。

### 3.5.2 服务接口（NoteService 扩展）

```go
func (NoteService) CreateSpark(ctx, content string, projectID *int64) (Note, error)
func (NoteService) PromoteSpark(ctx, id int64, as string) (Note or Todo seed, error) // as: "note" | "todo"
func (NoteService) ListSparks(ctx, includeArchived bool) ([]NoteSummary, error)
func (NoteService) ArchiveSpark(ctx, id int64) / RestoreSpark(ctx, id int64) error
func (NoteService) ListNotes(ctx, projectID *int64, tag string) ([]NoteSummary, error)  // 过滤扩展
func (NoteService) LinkProject(ctx, id int64, projectID *int64) (Note, error)           // 项目关联
func (NoteService) CopyRichText(ctx, id int64) error   // markdown → HTML 到剪贴板（富文本复制）
// 事件：note.changed 补 kind 字段；自动归档由启动 sweep + 灵感 Tab 激活时触发
```

### 3.5.3 功能验收（M0.5）

1. 灵感速记 Tab：输入卡回车保存 → 卡片入「未处理灵感」流（计数、卡内容 13px、项目色点+时间、转笔记/转待办/删除钮）；⌘⏎ 同样保存；
2. 「转笔记」→ 新建笔记文档（标题=灵感首行截 20，正文=引用灵感），原灵感从流中移除；「转待办」→ 新建待办（title=灵感内容）；删除=二次确认；
3. 归档：归档胶囊开浮层（30 天/90 天规则说明 + 归档项 + 恢复/删除）；归档 90 天后被 sweep 清除；
4. 笔记文档 Tab：文档头（21 Bold 组合标题「项目 · 标题」+ 保存状态 + 代码/预览切换 + 删除；项目 chip + 更新时间 + 字数）；正文 Milkdown GFM 按 v3 排版规格渲染（列表/任务/引用/代码块含复制钮/分割线）；`/` 菜单可用；
5. 辅助区：最近笔记卡（当前项高亮 EE F1F6、项目色点+名称+时间、新建快捷钮）、常用标签卡（正文 #标签 Top5，点击过滤）、导出卡（Markdown 导出复用 §4.4；**复制为富文本**经剪贴板 text/html；PDF 按钮置灰 tooltip「后续版本开放」，不误导）；
6. 笔记项目关联：笔记编辑态可选项目（项目 chip 点击弹选择浮层），保存后色点/标题组合/最近笔记 meta 同步；
7. 迁移 0003 从零重放 + kind/归档计数 Go 单测；E2E 新增 runNoteV2Flow（灵感保存/转笔记/转待办/归档浮层 + 笔记文档双模式切换 + 标签过滤）。

## 4. 功能需求

### 4.1 F1 工作台壳（v2.0 重构）

布局与样式严格按 `docs/design/ui-baseline.md` v3 §二/三/四实现，要点：

- **侧边导航栏（216px，可折叠至 64px，记住收起状态）**：
  1. 用户信息区：头像 44 圆形 + 姓名 + 「个人工作台」副标 + 收起按钮；
  2. 菜单（分组标题「工作台」）：数据看板 / 待办事项（带未完成数字角标） / 信息记录 / 股票看台（占位） / 快捷入口；active 态 = 左侧 3px 主色指示条 + 主色加粗文字，**禁止色块选中高亮**；
  3. 项目列表区：标题「项目」+ 新建按钮（ minimalist 弹层：名称+色盘选择，或默认色循环）；项目项 = 色点 + 名称 + 未完成计数；点击项目 → 跳转待办视图并按该项目过滤；
  4. 底部：设置项。
- **顶栏（64px 白底）**：页面标题 + 面包屑（随视图切换）；全局搜索框（样式占位，不做后端）；通知/筛选按钮占位、**主题切换按钮（真实生效，循环浅/深/跟随系统）**。
- **辅助区（256px，可收起，收起后主区扩展）**：内容随视图映射——
  - 数据看板 & 其他视图：现「今日聚焦」布局（日期卡+周日历条/快速灵感/最近记录/项目进度；设置页时隐藏或不相关内容留白）；
  - 待办事项：月历卡（事件点过滤）+ 按属性看（分类计数筛选）+ 按项目查看（项目筛选） + 即将到来（未来 3 个有日期的未完成待办）；
  - 占位内容一律删除，不留「M1 开放」卡片。
- 默认进入「数据看板」（设计稿首屏即 Dashboard；v1 默认待办，随 v2 变更）。
- 暗色模式：v3 双模式 token（§三）落地，跟随系统 + 手动覆盖并持久化（Settings 已有 theme key，扩展 `light/dark/system`）。

**验收标准**：
1. v3 基线视觉对照用户目检通过（主色/中性色/圆角/间距/导航 active 指示条）;
2. 侧边栏收起/展开且刷新后保持；辅助区收起/展开；窄窗 <1200px 自动折叠；
3. 五个导航项 + 项目列表 + 设置渲染正确；待办角标计数与数据一致且事件驱动更新；
4. 主题切换（顶栏按钮与设置页）即时生效并持久化，暗色下无对比度缺陷；「打开目录」能用；
5. 新建项目出现在侧边栏，修改后计数/名称/颜色即时更新（事件驱动）。

### 4.2 F0 数据看板（v2.0 新增）

设计稿「工作台 Dashboard 页」直接映射：

- **KPI 行**（456px 宽度等分 4 张白卡 r16，高 120）：
  1. 今日待办：今日应做未完成计数（今天与前所有未完成，拟定取 `due_date < 今日 24:00` 且未完成数）+ 较昨日徽章（红涨/绿跌）；
  2. 进行中项目：定义未定（设计稿示例 4，即有未完成待办的项目数）；
  3. 灵感记录：notes 总数 + 本周（近 7 日）新增；
  4. 本周专注：M0 **无数据源**，显示占位（下划0h，M2 补）；样式完整但标「占位」语义。
  - 迷你趋势柱：8 柱主色蓝灰梯度（静态，数据源待 M2；不做空轴交互）。
- **待办事项卡**（左，380px）：标题「待办事项」+ 计数「N / M」（未完成/全部）；快速添加输入框（回车创建，同 F2）；筛选 Tab 全部/进行中/已完成；列表同 F2 条目样式（chew列1022+分类徽章+删除）；已完成沉底灰划；底部「查看全部 N 项 →」→ 跳待办视图。
- **今日信息卡**（右 flex）：标题「今日信息」+ 右侧日期「10月N日 · 星期N」；时间轴 = 当日有 `due_date` 的待办（完成/未完成状态映射绿点/蓝点/灰点 + 状态徽章 已完成/进行中/待开始）+ 当日记录（「已记录」徽章）；无数据时段留白。图表规范 M2 才启用。

**验收标准**：
1. 四张 KPI 数字与真实数据一致且事件驱动更新；
2. 待办卡增删改完成后即时刷新（与待办视图同源同事件）；
3. 今日信息按 `due_date` 当日待办 + 当日新建记录聚合，建议完成创作待办后徽章状态正确；
4. 点击「查看全部」跳待办视图并落地同筛选态。

### 4.3 F2 待办列表（v2.0 扩展）

v1.1 的完成/删除/删除确认段落仍有效；新增：

- **快速创建卡**（待办视图顶部）：
  - 多行输入（默认 5 行高，随内容自适应）：`Enter` 仅换行，提交用「添加待办」按钮或 `⌘/Ctrl+Enter`（拟定，避免多行文本与快捷创建冲突）；
  - `#` 唤起项目联想浮层：选择后在文本中插入 `#项目名` token（主色）；输入时按名称过滤项目；
  - 时间解析 chip：识别「今天/明天/后天/周X」+「HH:mm」或「日期」短语，展示为日历图标 chip 可删除；未识别则不创建 `due_date`（进「未安排」组）；
  - 操作行：分类单选（个人/工作/学习，12 圆点）+ Primary「添加待办」。
- **列表**（待办视图主卡）：
  - 筛选 Tab：全部 / 今天 / 已完成（数字计数，active 主色下划线）；「今天」口径：`due_date` 为今日，**过期未完成项并入「今天」组**并以红色组点警示（拟定）；
  - **按日期分组**：今天 / 明天 / 未安排（组头色点：今天 #D97706、明天 #566C8C、未安排 #A1A1AB）+ 组计；组间 1px 分隔线；同组内未完成按 `due_date` 升序，已完成沉底；
  - 条目行：复选框 + 标题 13 + 右侧 = 分类徽章 / #项目名（项目色文字） / 时间 tabular / hover 删除；
  - **行内编辑态**（点击条目进入）：标题输入 + 日期 chip + 项目 chip（可移除） + 分类单选 + 取消（Ghost）/保存（Primary）；仅提交时写入，Esc 等价取消；
  - 点击条目右侧 #项目名跳出至按该项目过滤的列表；
- **月历**（辅助区）：月历 6 周、周一开头；今日 = 主色圆座白字；有件日期下方 4px 圆点（#9DB0C4 为普通，项目色可后定）；点击日期 → 主列表聚焦到该日（过滤态，可取消）；今日以前未完成事件点显示在对应未来日的前一天位置（拟定：仅提醒，不自动顺延）。
- **状态口径**：`due_date` 仅区分「有日期/无日期」与「今天前/今日/今天后」；过期未完成将在列表顶部以「今天」组红色组点警示（拟定）。

**验收标准**：
1. 多行输入 + `#` 联想选择项目后，`#项目名` token 保存至待办 `project_id`（重启后仍在，项目色/名称正确）；
2. 时间解析：输入「明天 15:00」创建，`due_date` 落库正确（重开 UTC→本地化展示对齐）；无法解析时退化为纯文本不建错日期；
3. 分类单选保存后，徽章与「按属性看」计数一致；
4. 分组：待办项落在正确日期组（含过期并入今天、未安排独立组），勾选完成后沉底灰划；
5. 行内编辑态：修改标题/日期/项目/分类保存后即时生效；取消不改变数据；
6. 月历：今日圈、事件点点位正确；点击日期过滤生效且可取消；
7. 辅助区「按项目查看」「按属性看」计数与筛选联动正确；「即将到来」显示未来有日期未完成待办按日期升序前 3 条；
8. 多窗口经 todo.changed 同步；`⌘/Ctrl+K` 从任意视图跳到待办视图并聚焦输入（v1 确认项保留）。

### 4.4 F3 信息记录

> v2.0：功能需求本节不变，视觉按 ui-baseline v3 换肤（卡片 r16、记录列表样式统一）；v2 已确认的「信息记录大编辑器 + 可收起列表」布局保留。

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

### 4.5 F4 快捷入口

> v2.0：功能需求本节不变（原 T6 未实施部分一并纳入），视觉按 ui-baseline v3 换肤。

- 条目字段：名称、目标（URL 或本地路径，录入时自动判定 `kind`）、分组、排序。
- 操作：添加（表单）/编辑/删除（二次确认）；分组以分隔或标签呈现（拟定：分组名内联标签 + 折叠分组头）。
- 跳转：URL → 调 mygo 桌面 API 用系统默认浏览器打开；本地路径 → 用系统「打开方式」打开。**安全边界**：仅打开用户显式录入的 `target`，禁止任何动态拼接命令；Go 侧打开实现固定为平台标准方式（mygo API 优先，兜底白名单命令 `open`/`xdg-open`/`explorer`）。

**验收标准**：
1. 添加 URL 条目后点击，系统默认浏览器打开对应网址；
2. 添加本地路径条目后点击，系统以默认应用打开（无效路径给出错误 toast，不崩溃）；
3. 分组增删与排序在重启后保持。

### 4.6 全局横切

- 错误处理：所有 RPC 调用统一经 `api/` 封装，失败 toast（含可读错误信息）；Go 侧错误经 mygo 错误机制返回结构化信息。
- 性能：常规操作（加待办、保存记录、点击入口）端到端响应 < 100ms（本地 SQLite 写入量级完全满足）。

## 5. 服务接口（Bind 服务签名，拟定 v2.0）

> 以下为 Go 侧服务方法草案，`mygo generate` 生成 TS 客户端后以此为准演进；两端类型同步修改，禁止前端私自加字段（AGENTS.md 4.3）。

```go
type ProjectService struct{}                        // v2 新增
func (ProjectService) Add(ctx, name string, color string) (Project, error)
func (ProjectService) List(ctx) ([]Project, error) // 携带未完成计数
func (ProjectService) Update(ctx, id int64, name, color string, sortOrder int) (Project, error)
func (ProjectService) Delete(ctx, id int64) error   // 关联待办 project_id 置 NULL
// 事件：ProjectChanged

type TodoService struct{}
func (TodoService) Add(ctx, in TodoInput) (Todo, error)  // TodoInput: title/category/dueDate(UTC)/projectId；解析由前端完成后传值（Go 侧校验字段合法性）
func (TodoService) List(ctx, f TodoFilter) ([]Todo, error)   // f: category/projectId/dateRange/done
func (TodoService) Update(ctx, id int64, in TodoInput) (Todo, error)
func (TodoService) SetDone(ctx, id int64, done bool) (Todo, error)
func (TodoService) Delete(ctx, id int64) error
func (TodoService) Stats(ctx) (DashboardStats, error)   // v2 新增：KPI、今日信息、项目进度等看板聚合
// 事件：TodoChanged（payload: 变更后的 Todo 或全量列表指纹）

// DashboardStats（拟定）：今日待办计数与较昨差、进行中项目数、记录总数与周增、
// 今日时间轴（当日 due 的 todo + 当日新建 note、状态）、项目进度（未完成/全部）。
// KPI 迷你趋势柱 M0 返回静态零值，M2 补真实序列。

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
  - store 层：CRUD 与迁移从零重放（含 0002 增量迁移，内存库或临时目录）；
  - 服务层：待办完成/恢复、记录保存/导出（临时目录断言文件内容与库内一致）、快捷入口 kind 判定与非法 target 拒绝；
  - 项目：删除项目后关联待办 `project_id` 置 NULL；计数正确；
  - Stats：今日/昨日口径边界（UTC 边界）回归；
- **前端单测**（vitest + testing-library，选型沿用评估建议）：导航切换联动、待办增删改状态驱动、事件驱动刷新、日期分组与筛选、主题 token 切换；
- **E2E 主链路**（覆盖 AGENTS.md §5，真实 WKWebView）：
  1. 添加待办 → 勾选完成 → 删除（含确认）；
  2. 新建记录 → 编辑含表格内容 → 归档 → 导出 .md；
  3. 添加 URL 快捷入口 → 点击跳转；
  4. 改姓名 → 左栏即时更新；
  5. （v2）新建项目 → 待办关联项目 → 分析日期分组/角标/侧栏计数/看板 KPI 同步；
  6. （v2）时间解析「明天 15:00」创建后 `due_date` 正确；
  7. （v2）主题切换亮/暗生效且刷新保持。

## 8. 里程碑与任务分解（v2.0，对应评估工作量在原 T7 后接续）

### v1.1 已完成任务（保留）

| # | 任务 | 状态 |
|---|---|---|
| T1 | 脚手架 | 完成 |
| T2 | 数据层 v1：settings/todos/notes/quick_links + store + 单测 | 完成 |
| T3/T3.0/T3.1 | 设计基线 v1 + 壳 + 设置真实接入 | 完成 |
| T4 | F2 待办 v1（增删改查 + E2E） | 完成 |
| T5 | F3 信息记录（Milkdown、CRUD/归档/导出 + E2E） | 完成 |
| UI-aux | 辅助区仅看板可见 + 信息记录大编辑器布局 | 完成待合并 |

### v2.0 重构任务（本次新增）

| # | 任务 | 依赖 | 预估 |
|---|---|---|---|
| R1 | 数据层 v2：`projects` 表 + `todos` 扩展字段（`category/due_date/project_id`）增量迁移 + ProjectService + TodoService 扩展 + Stats + 单测 | T5 | 1.5–2 |
| R2 | 设计令牌 v3 落地：tokens.css 重写（Slate/Zinc/语义色/双模式 Surface）+ 暗色跟随系统 + 设置页主题三项 | R1 | 1 |
| R3 | 壳重构：侧边栏（用户区/菜单5项/项目列表/设置）+ 顶栏（标题面包屑/搜索占位/主题钮）+ 收起态 | R2 | 2–2.5 |
| R4 | F0 数据看板视图：KPI 卡 + 待办卡 + 今日信息卡 + 「今日聚焦」辅助区 | R1、R3 | 2–2.5 |
| R5 | F2 待办重构：快速创建卡（#/时间解析/分类）+ 日期分组列表 + 行内编辑态 + 月历/按属性/按项目/即将到来辅助区 | R1、R3 | 2.5–3 |
| R6 | F3 信息记录 & F4 快捷入口换肤（功能不动，视觉对齐 v3） | R2 | 1–1.5 |
| R7 | E2E 主链路 v2（含 3–4 新用例）+ `mygo build` 打包冒烟 + 用户目检 | R4–R6 | 1.5–2 |

> 执行顺序：R1 → R2 → R3（壳先出可看界面交用户过目，与 v1 同惯例）→ R4/R5 可并行 → R6 → R7。每个任务一个 feature 分支（基于重构基线分支）。

### 执行状态（随开发更新）

- v1.1 状态：T1–T5 与 UI-aux 调整均完成（详见 git 历史）；T6（F4 快捷入口）未实施，被 v2.0 重构吸收进 R6。
- 2026-10-15：v2.0 重构启动（用户确定「先设计定样式与功能再开发」；设计基线 ui-baseline v3 + PRD v2.0 + Work Contract 确认）。
- 2026-10-15：R1 数据层 v2 完成（6b1c093）：projects 表 + todos 扩展 + ProjectService/TodoInput/Filter/Stats + 全量单测。
- 2026-10-15：R2 令牌 v3 完成（a7c9328）：Slate/Zinc/语义双模式 token + 三态主题。
- 2026-10-15：R3 壳重构完成（9907bf5 + 目检修复 cabc0b2）：侧边栏/顶栏/辅助区随视图映射/项目实体接入；用户目检通过（含收起态居中/指示条/顶栏 48px/窄屏贴顶 4 处修复）。
- 2026-10-15：R4 数据看板完成（675b605）：KPI×4 + 待办卡 + 今日信息卡 + 今日聚焦辅助区（实数据）。
- 2026-10-15：R5 待办重构完成（b340625）：快速创建（⌘Enter/#联想/时间解析/分类）+ 日期分组（过期并入今天）+ 行内编辑 + 月历/按属性/按项目/即将到来；新增 runTodoV2Flow E2E。
- 2026-10-15：R6 信息记录换肤 + 快捷入口真实接入完成（ee37339）：QuickLinkService（白名单 Open）+ LinkView v3 + 新增 runLinkFlow E2E。
- 2026-10-15：R7 收尾：全量门禁（go test -race/vet/generate/types/build:web）✓、E2E 全链路 ✓、`mygo build` 打包冒烟（Trap.app 启动稳定）✓、CHANGELOG 创建、待用户最终验收。
- 2026-10-13：T1 完成；T3.0 完成；T3 壳完成、样式重构完成并提交（e8e6bb6，用户目检通过）。
- 2026-10-13：T2 数据层完成（分支 `feature/m0-data-layer`）：迁移机制 + 四表 + `internal/store` + AppService 设置读写 + Go 单测全绿；用户已确认接受迁移 SQL 位于 `internal/store/migrations/`（go:embed 无法引包外目录）。
- 2026-10-13：T3.1 设置真实接入完成（分支 `feature/m0-settings-integration`）：设置初始化、持久化、settings.changed 同步、中文错误提示与导出目录手动保存已接入；真实 macOS WKWebView E2E 已通过并提交。
- 2026-10-14：T4 待办真实接入完成（分支 `feature/m0-todo`）：TodoService、todo.changed、SQLite 持久化、前端状态接入与双窗口真实 WKWebView E2E 已通过并提交（1a19a25）。
- 2026-10-14：T5 信息记录完成（分支 `feature/m0-notes`）：Milkdown 7.22.3 编辑器、NoteService CRUD/导出、note.changed、2 秒防抖自动保存与双窗口真实 WKWebView E2E 已通过并提交（4b81a28）。
- 2026-10-14：UI 调整完成（分支 `feature/m0-ui-aux-note`，用户指定）：辅助区（日历/便签/消息流）仅看板视图显示，其他视图移除；信息记录改为中间默认大编辑器 + 右侧可收起/展开记录列表（自动打开最近记录）；真实 E2E 已覆盖，待用户确认提交。

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

## 10. 已确认决策

### v1.1（2026-10-13 用户确认，v2.0 继承）

1. 删除策略：待办/记录/入口均用**二次确认**，软删留 P1。✅
2. ~~辅助区 M0 仅占位~~（被 v2.0 取消：辅助区改为随视图映射真实内容）。其他 v1.1 意图（动态消息流不做 UI）维持。✅/🔄
3. 自动保存防抖 2s 待办（待办多行制创将量键由「添加」承担）、`⌘/Ctrl+K` 跳视图并聚焦待办输入。✅
4. 归档 M0 为单条简单标记（无分组/合集）。✅
5. 目录结构差异（§9）与 AGENTS.md §1.3 的同步修改——T1 脚手架后确认。✅

### v2.0 拟定决策（待用户逐条确认后固化为定稿）

| # | 拟定决策 | 说明 |
|---|---|---|
| 1 | 默认视图改为「数据看板」 | 设计稿首屏即 Dashboard；旧会话中用户记忆的默认待办不再优先 |
| 2 | 时间解析仅前端短语匹配（今天/明天/后天/周X/HH:mm/日期）| 不引 NLP 依赖；无法解析不建 due_date，宁缺勿错 |
| 3 | 过期未完成并入「今天」组（红色组点警示） | 不新增「过期」独立组，避免口径复杂 |
| 4 | 今日信息卡 = 当日 due 待办 + 当日新建记录聚合 | 无独立消息流实体 |
| 5 | KPI 迷你趋势柱静态、本周专注卡占位 | 真实数据源待 M2 |
| 6 | 项目颜色由前端色盘顺序循环分配，可选改 | 不做后端颜色字段枚举限制（存 token 名） |
| 7 | 待办多行输入，`⌘/Ctrl+Enter` 提交为主、按钮兜底 | 与快捷创建不冲突 |
| 8 | 项目仅做关联与统计；点侧栏项目名跳待办视图过滤 | 不做项目工作视图（F5 仍属 M1） |

---

## 11. 历史版本

- v1.0–v1.1（2026-10-13–14）：见 git 历史；范围 = 壳/待办/信息记录/快捷入口 + 辅助区占位；全部 T1–T5 落实。
- v2.0（2026-10-15）：按 Ardot 设计稿重订范围（本文）；未确认前以拟定状态存在，确认后替换 v1.1 成为定稿。