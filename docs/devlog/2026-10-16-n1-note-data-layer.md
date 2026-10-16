# 2026-10-16 N1 信息记录 v2 数据/服务层（M0.5）

## 做了什么

（分支 `feature/n1-note-data`；PRD v2.1 §3.5 + ui-baseline v3 §5.4a–c）

- **迁移 `0003_notes_evolution.sql`**：notes + kind（'note'|'spark'，默认 note）、project_id、archived_at；两个索引。既有数据行为不变。migrate_test 版本断言升至 3。
- **store/notes_v2.go**：NoteV2/NoteSummaryV2、CreateNoteV2（kind 校验）、GetNoteV2、ListNotesV2（kind/项目/归档过滤，未处理灵感 = `archived_at IS NULL`）、SaveNoteV2（正文/标题/关联全量保存）、ArchiveNote（幂等归档）、RestoreNote、SweepSparks（30 天自动归档含满 30 天整 `<=` 边界、90 天清除，now 注入可测）、PromoteSparkToNote（保留原灵感=用户 2026-10-16 裁定）。
- **DeleteProject 补丁**：删除项目时 notes.project_id 一并置空（原只清 todos）。
- **service/note_v2.go**：CreateSpark/ListSparks/ListNotesV2(tag 过滤, #token 解析含中文标点 rune 边界)/PromoteSpark(note|todo, **跨域组合 TodoService**——经 `AttachTodoPeer` 注入 peer，避免 IPC 参数传服务)/ArchiveSpark/RestoreSpark/SaveNoteV2/LinkProject（校验项目存在）/SweepSparks（广播 note.changed）/CopyRichText。
- **IPC 单值约束**：mygo generate panic 教我——typed IPC 方法最多 (value, error)；PromoteSpark/SweepSparks 分别改返回 `PromoteSparkResult{Note,Todo}`/`SweepResult{Archived,Purged}` 单结构。
- **markdown_html.go**（零依赖 markdown→HTML 轻量转换，覆盖 GFM 常用子集）+ 剪贴板双格式写入（transfer.HTML + transfer.Text）。mygo Clipboard API 签名按 0.4.8 实测：`transfer.New(NewItem(Bytes(HTML,…), Bytes(Text,…)))`。
- **main.go**：当 ready 后 AttachTodoPeer + 启动 sweep（失败仅打日志不阻塞启动）。
- **generate**：TS 客户端已更新（NoteV2/ListSparks/PromoteSpark/Sweep 等全部类型生成）。

## 为什么这样做（含被否决的备选方案）

- v2 字段用独立 NoteV2 而非扩展 v1 Note 结构：保持 v1 JSON 契约不破坏（已生成 TS Note 类型继续有效）；列表摘要 v2 带_kind/projectId 供灵感流/最近笔记卡直用。
- 转待办走 TodoService peer 而非 store 直写：todo.changed 广播必须复用 TodoService.broadcast（事件一致性）；被否决的备选：store 层双写双事件（事件广播逻辑重复）。
- 归档/清除 SQL `updated_at <= cutoff`（满 30 天整即归档）：契约验收边界按 30 天整达标执行。
- 剪贴板：mygo Clipboard 原生支持多 flavor，直接双格式写入（比 earlier 草稿的「纯文本兜底」更符合验收 4 富文本诉求）。
- containsTag 按 rune 边界判定 #tag 后缀（中文标点跟边）——初版 byte 比较在含 中文标点时编译失败，已修。

## 验证结果

- `CGO_ENABLED=0 go test -race ./...` 全绿（含新 store/notes_v2_test.go：sweep 29/30/31/89/91 天边界 + v2 CRUD/过滤/转笔记保留原灵感/删除项目联动；service/note_v2_test.go：灵感 CRUD/Promote 双目标/Archive 幂等/LinkProject 校验/标签过滤/富文本剪贴板注入断言）。
- `go vet` ✓、`go build` ✓、`npm run generate` ✓（panic 修复后类型生成成功）、`tsc -p web --noEmit` ✓。

## 遗留问题 / TODO

- N2 灵感速记 UI / N3 编辑器文档头 / N4 辅助区换肤：前端消费新类型（NoteV2）。
- note.changed 广播 payload 尚不含 v2 字段——前端以「触发重拉」策略（与 todo.changed 同链路），如后续需要 payload 扩字段再改。
