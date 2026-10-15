# 2026-10-15 R6 信息记录换肤 + 快捷入口真实接入

## 做了什么

（分支 `feature/r6-skins`，基于 R5 提交 b340625；吸收原 T6——快捷入口此前未实施）

- **QuickLinkService（新，`internal/service/quicklink.go`）**：Add/List/Update/Delete/Open + `quick_link.changed`；kind 由 target 形态判定（http/https → url，其余 → path，写入 store 已有校验）；**Open 安全边界**按 PRD §4.4：平台白名单命令（darwin `open` / linux `xdg-open` / windows `explorer`），target 只作为单个参数传 `exec.Command`（不经 shell、无动态拼接），无效目标返回带上下文错误；main.go Bind + Attach。
- **前端 links 真实化**：`api/links.ts` 封装 + `realtime/quicklink-events.ts` 订阅；store 中 `initialLinks` mock 与 `nextId` 移除，`links/linksLoading/linksError + addLink/editLink/deleteLink/openLink` 全部走真实服务，`quick_link.changed` 驱动列表刷新。
- **LinkView 重写（v3 换肤，PRD §4.5）**：分组分节标题（空串=未分组，排序沿用 sort_order）；卡片网格 `auto-fill ≥220px`；卡片 = 首字母图标块（url→主色浅底、path→警告浅底；M0 不抓 favicon 保持「默认不主动联网」）+ 名称 + 域名/路径，悬停浮出编辑/删除；点击卡片整体打开目标（busy 态防抖）；添加/编辑共用 InputDialog；删除二次确认；错误统一 `[role=alert]`。
- **NoteView 换肤（功能不动，验收 6）**：编辑器面板 r16 白卡化；右侧记录列表改纯标题两行 + 徽章（摘要行隐藏），卡片 12px 圆角、hover 浅底无描边；其余交互（自动保存/归档/导出/删除确认）零改动。
- **E2E 新增 `runLinkFlow`**：UI 添加 URL 入口（弹层三字段）→ 列表显示 → 服务层断言（name/kind/group 持久化）→ 点击卡片真实打开（系统 open；断言无 JS 错误且无错误提示）→ 删除（二次确认）→ 列表与服务层消失。
- `setDialogFields` 提取为独立 helper（InputDialog 按 label 定位输入框）。

## 为什么这样做（含被否决的备选方案）

- Open 放服务层而非前端 `window.open`：AGENTS「子进程安全」与 PRD 打开固定平台方式；WebView 的 window.open 会尝试应用内导航，WKWebView 下不可控。被否决的备选：前端直接 `window.open(url)`——绕过安全边界且对本地路径无效。
- kind 判定后兼容 store 双 kind；`QuickLinkChangeAction` 直接复用（不设别名类型——初稿别名 Y 型声明反而混乱，已清理）。
- E2E 点击真实打开会弹出浏览器标签：这是「点击跳转」主链路的真实 E2E 验证（AGENTS §5 用户可见行为必须真实端到端）；替代方案（mock runner 注入）在包级变量可行但会弱化真实性，故保留真实 open 并在 devlog 记录该副作用。
- NoteView 摘要行隐藏：设计稿记录列表仅标题+时间；「点击打开 Markdown 编辑器」摘要无信息量。

## 验证结果（跑了哪些测试、结果如何）

- `go build/vet ./...` ✓；`CGO_ENABLED=0 go test -race ./...` ✓；`npm run generate` ✓（QuickLinkService 类型生成）；`npx tsc -p web --noEmit` ✓；`npm run build:web` ✓；
- `npm run test:e2e` 两轮 ✓（本轮全量含 runLinkFlow + 全部既有流程）。

## 遗留问题 / TODO

- 打开失败的「错误 toast」目前为 `[role=alert]` 文本条（v2 样式），与其他视图一致；toast 组件化留 R7 后统一评估。
- 排序 UI 暂未暴露（sort_order 服务端已有，M1 随分组折叠一起做）。
