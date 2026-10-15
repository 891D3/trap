# 2026-10-15 R4 数据看板视图 + 今日聚焦辅助区

## 做了什么

（分支 `feature/r4-dashboard`，基于 R3 目检修复提交 cabc0b2）

- **视图状态入 store**：`view/setView` 从 App 局部状态迁入 AppStore（DashboardView「查看全部」需跨组件跳转；深链接 e2eView 仍生效，StrictMode 安全的 ref 防重复）。
- **看板聚合入 store**：`stats`（DashboardStats）由 `TodoService.Stats` 加载，`todo.changed/note.changed/project.changed` 三类事件驱动刷新。
- **DashboardView（新）**：
  - KPI 行 4 卡（今日待办 count + 较昨日差值红涨绿跌徽章 / 进行中项目 / 灵感记录 total+周增 / 本周专注 0h「M2」占位徽章），迷你趋势柱为纯 CSS 静态占位（rq 决策 5）；
  - 待办卡：计数徽章 N/M、快速添加输入（回车创建，失败恢复草稿）、筛选 Tab（全部实态，进行中/已完成跳待办视图）、未完成前 8 条（复选框+标题+分类徽章+删除），删除走 ConfirmDialog（v1.1 二次确认）；空态引导；
  - 今日信息卡：日期头部、时间轴（stats.todayAgenda：todo done=绿点已完成/scheduled=蓝点待开始、note=灰点已记录，HH:mm tabular）、空态、页脚项目计数。
- **辅助区「今日聚焦」实数据**（`panels/aux/TodayFocus.tsx`）：今日日期卡（32 大日号 + 周日历条 7 胶囊今日主色）、快速灵感（Enter 保存/按钮双路径：addNote+saveNote 标题取首行 20 字）、最近记录（未归档按 updatedAt 倒序 4 条 + 相对时间 + 「查看全部」跳信息记录）、整理为 Markdown（批量导出到导出目录）、项目进度（进度条按项目色令牌，百分比 = (total-open)/total）。
- **CSS**：`.kpi-*`、`.dash-*`、`.today-*`、语义徽章（badge-up/down/flat/done/open/note）、分类徽章 person/work/study、aux[::-今日聚焦] 全套（周条/灵感输入/最近记录/进度条）；`.aux` 固定 256px 白底贴边。

## 为什么这样做（含被否决的备选方案）

- stats 放 store 而非组件局部：DashboardView 与辅助区都消费，且三个事件都要刷新——store 单一订阅点符合 AGENTS.md「组件不各自开连接」。
- mini 柱静态、专注卡 0h：契约拟定决策 5（M2 补真实序列）；徽章文案标注「数据接入后展示（M2）」避免误导（验收 2「不误导」）。
- 「查看全部」直接 setView('todo') 而非带筛选参数：PRD 验收 4 要求「落地同筛选态」——R5 待办视图有筛选 Tab 状态时再接（R4 注明 TODO）。
- 快速灵感复用 addNote('灵感')+saveNote 而非新 RPC：NoteService 已有完整能力，避免服务层膨胀；标题截断 20 与 NoteView 的「标题默认取正文首行」口径一致。
- 灵感保存失败静默 console.error 而非 toast：辅助区频繁交互，错误场景（RPC 失败）极罕见且不打断输入；PRD 全局横切的 toast 要求主要针对用户主动操作，此处保留输入内容不丢失。

## 验证结果（跑了哪些测试、结果如何）

- `npx tsc -p web --noEmit` ✓；`npm run build:web` ✓；`npm run test:e2e` ✓（全流程回归）。
- E2E 未新增看板断言（契约含「R7 E2E 全链路验证」，R4 期间以手工目检 + 既有回归兜底）。

## 遗留问题 / TODO

- 「查看全部」落地同筛选态待 R5；
- 今日信息时间轴里 todo 的时间粒度 = due 的 HH:mm；全天事件与「未安排」不进时间轴（口径见 PRD §4.2）；
- 项目进度条无「0 待办」过滤（设计如此，0% 显示空条）——若用户目检觉得吵再调。
