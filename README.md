# Trap · 个人工作台 / Personal Workbench

[English](#english) | [中文](#chinese)

---

<a id="english"></a>
## English

**Trap** is a local-first, AI-native personal workbench for everyday work, study, and hobbies. It runs as a desktop app that keeps everything — todos, notes, projects, quick links — in one always-available place, with 100% local data.

### Core Ideas

1. **Everything is a "thing", and things relate** — todos, notes, projects, and quick links live in one interface and connect through attributes.
2. **AI-native** — a conversational agent entry point, plus MCP support so external AI clients can read and operate on your workbench data.
3. **Local-first** — all data stays on your machine (SQLite + local files); network access happens only when you explicitly trigger it.
4. **Observable & traceable** — a dashboard shows your efficiency and growth; a message stream aggregates every event.

### Current Features (v0.3.0)

- **First-run onboarding** — the first launch shows a setup screen (username required, optional export directory & theme). Data is unified under `~/.trap` (with `copies/`, `exports/`); existing data is migrated automatically with a backup kept, and falls back safely on failure.
- **Dashboard** — 4 KPI cards + today's todo card + today's info timeline + "Today's Focus" side panel.
- **Todos** — quick capture (multi-line, `#project` mentions, natural date parsing like "tomorrow 15:00", category picker), date-grouped list, inline editing, monthly calendar / by-attribute / by-project / upcoming.
- **Notes v2** — two modes:
  - *Spark capture*: jot down short ideas as cards; promote to a note document or a todo; archive (30-day auto-archive / 90-day purge).
  - *Note documents*: Markdown editor (Milkdown), combined title "Project · Title", save status, code/preview toggle, project linking, word count.
- **Quick links** — grouped card grid; opens URLs/paths via a platform whitelist command.
- **Projects** — create/rename/recolor/delete; links todos & notes; project tabs at the top.
- **Dark mode** — follow system / light / dark.

### Tech Stack

- **Host**: [mygo](https://github.com/egoist/mygo) (Go + WebView)
- **Backend**: Go — typed IPC services, workflow-ready store layer
- **Frontend**: React 19 + TypeScript + Vite + Milkdown (markdown)
- **Storage**: SQLite (versioned migrations) + local files
- **Data directory**: `~/.trap`

### Development

```bash
# install frontend dependencies
npm install

# start the app with HMR (Go + Vite)
npm run dev

# typecheck / build frontend
npm run typecheck
npm run build:web

# Go build / vet / test
go build ./...
go vet ./...
go test ./...

# regenerate the typed IPC client
npm run generate

# full build (app + dmg)
npm run build

# real WebView end-to-end tests (macOS)
npm run test:e2e
```

### Install

Download the `.dmg` from [Releases](https://github.com/891D3/trap/releases) (macOS arm64), open it, and drag **Trap** into Applications.

> Requires macOS (Apple Silicon). First launch walks you through setup.

---

<a id="chinese"></a>
## 中文

**Trap（个人工作台）** 是一个本地优先、AI Native 的个人效率与成长工作台，以桌面应用形态常驻，把工作、学习、兴趣爱好相关的事务——待办、信息记录、项目、快捷入口——沉淀到一个统一入口，数据 100% 本地。

### 核心理念

1. **一切皆事务，事务可关联**：待办、信息、项目、快捷入口统一在一个界面内，用属性相互关联。
2. **AI Native**：对话式 Agent 入口 + MCP 接入能力，外部 AI 客户端可经 MCP 直接调用和操作工作台数据。
3. **本地优先**：所有数据保存在本机（SQLite + 本地文件）；仅在用户显式触发时出网。
4. **可观察、可回溯**：数据看板呈现效率与成长轨迹；动态消息流汇聚全部事件。

### 当前功能（v0.3.0）

- **首次安装初始化**：首次打开显示初始化页（用户名必填 + 可选导出目录/主题）；数据统一存放 `~/.trap`（含 copies/、exports/），存量数据自动迁移（保留备份，失败安全回退）。
- **数据看板**：4 张 KPI 卡 + 今日待办卡 + 今日信息时间轴 + 「今日聚焦」辅助区。
- **待办事项**：快速创建（多行、`#项目` 联想、时间短语解析如「明天 15:00」、分类单选）、按日期分组、行内编辑、月历 / 按属性看 / 按项目查看 / 即将到来。
- **信息记录 v2**（双模式）：
  - *灵感速记*：随手记短灵感成卡片；可转笔记、转待办；归档（30 天自动归档 / 90 天清除）。
  - *笔记文档*：Markdown 编辑器（Milkdown）、组合标题「项目 · 标题」、保存状态、代码/预览切换、项目关联、字数统计。
- **快捷入口**：分组卡片网格，经平台白名单命令打开 URL/本地路径。
- **项目**：新建/重命名/改色/删除，关联待办与笔记，顶部项目标签页。
- **暗色模式**：跟随系统 / 浅色 / 深色。

### 技术栈

- **宿主**：[mygo](https://github.com/egoist/mygo)（Go + WebView）
- **后端**：Go —— typed IPC 服务 + store 层（工作流引擎预留）
- **前端**：React 19 + TypeScript + Vite + Milkdown（Markdown）
- **存储**：SQLite（版本化迁移）+ 本地文件
- **数据目录**：`~/.trap`

### 开发

```bash
# 安装前端依赖
npm install

# 启动应用（Go + Vite 热重载）
npm run dev

# 类型检查 / 前端构建
npm run typecheck
npm run build:web

# Go 构建 / vet / 测试
go build ./...
go vet ./...
go test ./...

# 重新生成 typed IPC 客户端
npm run generate

# 完整打包（app + dmg）
npm run build

# 真实 WebView 端到端测试（macOS）
npm run test:e2e
```

### 安装

从 [Releases](https://github.com/891D3/trap/releases) 下载 `.dmg`（macOS arm64），打开后把 **Trap** 拖入「应用程序」。

> 需要 macOS（Apple Silicon）。首次启动会引导完成初始化设置。
