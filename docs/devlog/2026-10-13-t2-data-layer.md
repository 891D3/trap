# 2026-10-13 T2 数据层（迁移 + 四表 + store + AppService 设置读写）

## 做了什么

### 范围与分支

- 前置：T3 样式改动经用户目检通过后提交 `e8e6bb6`（`feature/m0-scaffold`），随后建立 `feature/m0-data-layer`。
- 工作契约：用户已接受（范围=迁移机制 + settings/todos/notes/quick_links 四表 + internal/store + AppService 设置读写 + Go 单测；不含 Todo/Note/QuickLink 服务与前端接入）。

### 实现

- `internal/store/migrations/0001_init.sql`：按 PRD §3 建四表（UTF8 文本时间、UTC RFC3339）；发布后不可修改（AGENTS.md §4.4）。
- `internal/store/store.go`：
  - `store.Open(ctx, path)` 打开连接并执行迁移；`migrate` 从 `//go:embed migrations/*.sql` 枚举文件，按编号升序应用；
  - 迁移器自建 `schema_migrations(version, applied_at)`；每个迁移的语句拆分后连同版本记录写入**单个事务**（mygo `Transaction` = BEGIN IMMEDIATE，任一失败整体回滚）；已应用版本不重跑（幂等）；
  - `splitSQL` 文本级按 `;` 拆分并剥离 `--` 注释（遍历调用点保持简单，当前迁移不含含分号的字符串字面量；文件头注释已写明约束）；
  - `migrateFS(ctx, fs.FS)` 内部抽出供测试注入坏迁移。
- `internal/store/settings.go|todos.go|notes.go|quicklinks.go|conv.go`：
  - 四表 CRUD（Add/Get/List/Set/Save/Archive/Update/Delete），所有写操作走 `Transaction`（AGENTS.md §4.4）；
  - 勾选写 `completed_at`、取消置 NULL；notes 列表走 `NoteSummary`（不携 content）；quick_links `sort_order` 取 max+1、`kind` 校验只收 url/path（判定归 T6 service）；
  - `ErrNotFound` 统一 NotFound 语义；时间一律 `time.Now().UTC().Format(time.RFC3339)`；`conv.go` 集中结果列断言。
- `internal/service/app.go`：`Settings` 结构（json 标签）+ `SettingsChanged = mygo.NewEvent[Settings]("settings.changed")` 事件；`GetSettings/UpdateProfile/SetTheme/SetExportDir` 经 store 读写；主题校验 system/light/dark；姓名非空校验；写后 `Broadcast`（广播失败显式忽略不阻断——数据已落库）；`Ping` 保留；`SetStore` 在 `WhenReady` 中注入（未注入时显式报错）。
- `main.go`：`WhenReady` 内 `mygo.App.Path(PathUserData)` → 打开 `trap.db` → 注入 AppService → 开窗；窗口创建晚于数据层就绪。数据库迁移必须放在 `WhenReady`：mygo 文档明确 `mygo generate` 会运行 main 到 `App.Run` 就退出，过早迁移会在生成期跑库。
- `web/src/mygo.ts` 经 `npm run generate` 重新生成（含 Settings、AppService 方法与 settings.changed 事件）。

## 为什么这样做（含被否决的备选方案）

- **驱动选 mygo 内置 `plugins/sqlite`**（purego、无 cgo）：避免新增第三方依赖（AGENTS.md 硬约束），且 mygo 已是 go.mod 现有依赖。被否决：`modernc.org/sqlite` / `mattn/go-sqlite3`（新依赖 + cgo 环境问题）。
- **迁移 SQL 从根级 `migrations/` 改为 `internal/store/migrations/`**：`go:embed` 无法引用包目录之外文件，嵌入是打包后可用的唯一稳妥方式。根级方案被否决（打出的二进制要另搭 resources 加载路径，复杂化；「接口如无文档必须查证」已查 mygo 文档确认 embed 限制）。PRD §9 目录表的偏差已在 PRD §8 执行状态中标注；AGENTS.md §1.3 图的修改需用户确认后另行处理。
- **service 层不写 SQL、store 层不判业务**：`ErrNotFound` 语义在 store 定义但业务校验（kind 判定、姓名/主题规则）按 PRD §5 放 service。
- **事件在 T2 即定义并 Broadcast**：PRD §2 要求设置变化由 Go 侧类型化事件广播（`settings.changed`），前端接入时事件已在；`Broadcast` 无窗口时安全（查证 ipc.go 实现为空循环）。
- **接口签名按 mygo 文档实际约定**（context.Context 可选首参、返回值+error、Bind 指针注册先于 App.Run、注释生成 TSDoc）——均查证 `docs/bindings.md` 与源码，无杜撰。
- 被否决：在 `main` 顶层提前打开数据库（与 MYGO_GENERATE 生成流程冲突，文档明确建议 `WhenReady`）。

## 验证结果（跑了哪些测试、结果如何）

- `go build ./...` ✓、`go vet ./...` ✓（0 警告）。
- `CGO_ENABLED=0 go test ./...` ✓：
  - store：`TestMigrateFreshReplay`（四表+版本 1）、`TestMigrateIdempotent`（重开不重跑、数据无损）、`TestMigrateRollbackOnFailure`（fstest.MapFS 注入坏迁移，版本记录与先执行语句一并回滚）、`TestSettingsRoundtrip/Upsert/Rollback`、`TestTodoCRUDAndCompletedAt`、`TestNoteCRUDArchiveAndList`、`TestQuickLinkCRUDAndOrder`；
  - service：ready 守卫、GetSettings 键值映射（含 DataDir 透传）、UpdateProfile 迁移与校验、SetTheme 校验；
  - 全部时间字段断言 UTC RFC3339（`mustParseTime`）。
- SQL 归属检查：`grep -E "SELECT |INSERT INTO|UPDATE .* SET|DELETE FROM|CREATE TABLE"` 非测试 `.go` 文件仅命中 `internal/store/*.go` ✓。
- 前端：`npx tsc -p web --noEmit` ✓（mygo.ts 重新生成后）；`npm run build:web` ✓（前端行为零改动）。
- 首轮 `go test` 下载 mygo 预编译 SQLite 库（约 31s）——契约中用户已知悉的测试期依赖获取，运行时不联网。
- 未覆盖：`WhenReady` 打库→注入→开窗的真实 GUI 冒烟（无头环境不可见；generate 流程已跑通 Bind 链路不炸），真实启动冒烟建议与 T7 E2E 一并做或用户 `npm run dev` 目检设置页。

## 偏差与遗留问题 / TODO

1. **目录偏差（已确认）**：用户已确认接受迁移 SQL 位于 `internal/store/migrations/`（非 PRD §9 根级 `migrations/`），原因见上。
2. `0001_init.sql` 字段已按 PRD §3 完成复核；后续结构变更只能追加新迁移。
3. T2 已获用户确认提交；`AGENTS.md` 的一处未提交改动（「提交前确认」→「代码需测试」，108 行）仍保留在工作区，不纳入本次提交。
4. 服务层测试为白盒直接注入 store；`settings.changed` 事件的端到端接收测试归前端接入（T4+）与 T7 E2E。
5. 顺手修正 `0001_init.sql` 中一处笔误注释（多余的括号）。

## 交接（下一步）

- T4 待办：`TodoService`（Add/List/SetDone/Delete + 事件 todo.changed）+ 生成客户端 + 前端 mock 替换。
- 按顺序 T5（Milkdown 冒烟先行）→ T6 → T7（E2E + 打包冒烟）。

## 完成功能复核（2026-10-13）

- `CGO_ENABLED=0 go test -race ./...` 通过；`go test ./...`、`go vet ./...`、`npm run generate`、`npm run typecheck`、`npm run build:web` 均再次通过。
- 复核发现并已修复高优先级缺陷：原 `AppService.SetStore(*store.DB, string)` 会被 mygo 自动绑定，生成不应暴露的 `AppService.setStore` 与空 `DB` 接口；现改为包级 `service.AttachStore` 调用非导出 `setStore`，重新生成后 `web/src/mygo.ts` 已不再包含 `DB/setStore/SetStore`。
- 修复后复验：`CGO_ENABLED=0 go test -race ./...`、`go build ./...`、`go vet ./...`、SQL 归属检查、`npx tsc -p web --noEmit`、`npm run build:web` 全部通过。
- `go tool mygo vet .` 未执行成功：当前环境报告 `go: no such tool "mygo"`；不影响已通过的 `go vet ./...`，但应在具备该工具的环境补跑。
