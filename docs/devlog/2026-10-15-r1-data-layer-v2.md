# 2026-10-15 R1 数据层 v2（Ardot 设计驱动重构）

## 做了什么

- 文档先行（用户要求「先设计定样式与功能再开发」）：
  - `docs/design/ui-baseline.md` 重写为 **v3**：以 Ardot 设计稿 `WorkDesktop Project v1.0`（文件 736400687124033）为准，固化设计原则/布局骨架/全部设计 token（Slate 主色 #566C8C、Zinc 中性、语义涨红跌绿、双模式 Surface）/组件规范/两功能页（Dashboard、待办）的功能结构。视觉截图存 `docs/design/ardot-{dashboard,todo}-2x.png`。
  - `docs/prd/M0-工作台MVP.md` 升级 **v2.0**：新增 F0 数据看板、F2 扩展（项目关联/日期/分类/行内编辑）、projects 表、服务接口（ProjectService/TodoInput/TodoFilter/Stats）、里程碑 R1–R7；旧原 T6（快捷入口）并入 R6。经用户 Work Contract 确认（8 条拟定决策随契约一并确认）。
  - `docs/brd/BRD.md` 功能状态表同步。
- R1 数据层实现（分支 `feature/r1-data-layer-v2`）：
  - 迁移 `internal/store/migrations/0002_projects_todos.sql`：projects 表 + todos 增 category/due_date/project_id 三列（已有数据行为不变）。
  - `internal/store/projects.go`：项目 CRUD + LEFT JOIN 计数（total/open）；颜色存 token 名（slate/amber/green/purple，校验取值）；DeleteProject 在事务内显式把关联待办 project_id 置 NULL（不依赖 FK pragma）。
  - `internal/store/todos.go` 重写：TodoInput（title/category/dueDate UTC RFC3339/projectId）+ UpdateTodo + TodoFilter（done/category/project/due range/noDue）全量参数化；默认排序 done ASC → 已安排升序 → 未安排 → id DESC；due_date 格式由 Go 校验（解析在前端完成，PRD 决策 2）。
  - `internal/store/stats.go`：DashboardStats 聚合（今日待办含过期未完成、昨日同口径差值、进行中项目数、记录总数/近7日新增、今日时间轴 = 当日 due 待办 + 当日新建记录、项目进度）。时间边界按调用方传入 now/时区折算 UTC 文本比较（测试可注入固定时间）。
  - `internal/service/project.go`：ProjectService（Add/List/Update/Delete + `project.changed` 广播）；`internal/service/todo.go`：Add 改 TodoInput、新增 Update/List(filter)/Stats；main.go Bind + Attach。
  - 前端：`npm run generate` 重新生成 TS 客户端；`web/src/api/todos.ts` 归一化新字段（category/dueDate/projectId）、listTodos(filter)、createTodo(TodoInput)、updateTodo、dashboardStats；新增 `web/src/api/projects.ts`；app-store 适配。

## 为什么这样做（含被否决的备选方案）

- 一开始 R1 内又写了 project CRUD（时间链：用户最初让继续 T6 快捷入口→拒绝其继续，用户提出新工作流），发现设计稿驱动重构下项目管理方式不可控，改为：**先设计定档（ui-baseline v3 + PRD v2.0 + Work Contract）→ 用户确认 → 再实现**。被否决的备选：直接按旧 PRD 继续做快捷入口 T6（与设计稿的功能结构冲突）。
- TodoFilter.Category 空串语义定为「不过滤」而非「未分类」：M0 的「按属性看」只有 三个具名分类，不存在「仅未分类」需求；哨兵值方案（catAny="-"）多余，首版实现曾因此引入 bug（空 filter 误过滤 category=''，测试捕获后已修）。
- 项目删除不用 FK ON DELETE 级联：mygo sqlite 不保证 foreign_keys pragma 开启，行为必须显式可测；DeleteProject 在同一事务内先 UPDATE todos 置 NULL 再 DELETE projects。
- Stats 的「今日待办」口径 = due <= 今日末的未完成（含过期，PRD 拟定决策 3 的数据层口径）；昨日口径 = due <= 昨日末的未完成（历史快照语义）。KPI 迷你柱 M0 静态不落库。
- 非法项目关联在 Go 侧显式校验（GetProject 存在性检查），与 FK 叠加双保险。

## 验证结果（跑了哪些测试、结果如何）

- `CGO_ENABLED=0 go test -race ./...` ✓；`go build ./...`/`go vet ./...` ✓；`npm run generate` ✓（Todo/Project/Stats 类型生成）；`npx tsc -p web --noEmit` ✓；`npm run build:web` ✓（>500KB chunk 警告沿用 T5 现状，非本次引入）。
- 新增测试：migrate_test 更新至版本 2（含 0002 回滚/幂等）、projects CRUD+计数+删除置空、todos 扩展字段与 7 种过滤条件默认排序、Stats 聚合全口径（UTC+8 固定时区）、service 层 project CRUD/守卫/事件名与 Stats 零值。

## 遗留问题 / TODO

- R2–R7 按契约推进；E2E 用例（新增 5–7 条 + 回归）留 R7；暗色 token 属 R2。
- `notes.created_at` 无合法的更新入口，Stats 测试回填 created_at 用的是 store 包内测试直连事务——业务路径不受影响，如未来需要「修时间」需走正规迁移/服务。
