# 2026-10-13 T3 样式重构（参考 stylekit file-manager 模板）

## 做了什么

- 依用户指定，参考 https://www.stylekit.top/zh/templates/file-manager 重构视觉样式（配色/圆角/边框/阴影/按钮/图标块/侧栏形态），三栏结构与交互不变。
- `docs/design/ui-baseline.md`：新增「〇、v2 样式方向」；§一 悬浮岛式侧栏条目标记 [SUPERSEDED by v2]；§二 新增「v2 生效令牌」表，原 v1 令牌保留为历史。
- `web/src/styles/tokens.css`：整体替换为 v2 令牌（页面底 `#f9fafb`、主色 `#3b82f6`/hover `#2563eb`、浅底 `#eff6ff`、边框 `#e5e7eb`、文字 gray-900/500/400；卡片圆角 12、控件 8；阴影默认 none，仅悬浮与弹层用；深色主题同步映射）。
- `web/src/style.css`：类名不变，值全部按 v2 重写。要点：侧栏改为贴边白底 + 右侧 1px 细边（去掉悬浮投影与外边距）；卡片白底细边、静态无投影、悬停蓝描边+轻阴影；按钮/输入框圆角 8；分段控件改为灰槽+白色浮起选中项（模板视图切换风格）；右栏卡片改为页面同底+细边，视觉权重低于中栏。
- `web/src/state/app-store.tsx`：`avatarColors` 改为模板图标色板（blue/green/orange/purple/pink-500）。
- `web/src/panels/views/LinkView.tsx`：快捷入口图标块改为 10% 透明度浅底 + 同色字（`${hex}1a`），对应模板 `bg-*-50 text-*-500`。

## 为什么这样做（含被否决的备选方案）

- 配色取模板的 Tailwind gray/blue 色板，而非保留原 `#4F6BF6`：用户要求按模板风格，色值整体替换才一致；保留旧主色会与模板观感不符。
- 侧栏改贴边、去投影：模板侧栏是贴边白底细边。悬浮岛式是 v1 设计点，已在 ui-baseline 标记为 SUPERSEDED，而非静默删除。收起/展开/悬停行为保持不变。
- 按钮高度沿用 26px 基准：用户此前多次反馈「按钮缩小」，本次只改颜色与圆角，不回退已确认的尺寸。
- 字体未引入模板的 Albert Sans：需要引入字体文件属于新依赖，AGENTS.md 要求先确认，故沿用系统字体栈。
- 快捷入口图标块未做模板式多色分类：保留「每个入口一个色」的 hash 映射（色板换为模板色），避免为模板多色重写数据模型。
- 被否决：直接引入 Tailwind 重写组件（引入新依赖且改动面过大，本次只需视觉对齐）。

## 验证结果（跑了哪些测试、结果如何）

- `npx tsc -p web --noEmit` ✓（exit 0）
- `npm run build:web` ✓（CSS 11.75KB，JS 240.31KB；web/dist 已被 .gitignore 忽略）
- 浏览器截图对照：`npm run dev:web`（端口 5199）+ 无头 Chrome 截图，待办视图与模板截图（/tmp/template.png）风格一致：细边白卡、灰底、蓝主色、右栏低权重。截图 /tmp/app-v2.png。
- 未覆盖：信息记录、快捷入口、设置、弹层、深色主题、<1200px 抽屉的视觉未做截图核验（无头截图无法点击切换视图），仅经代码审阅与构建通过。
- 未跑任何单元测试（仓库目前无测试文件，属 AGENTS.md §5 的既有缺口，非本次引入）。

## 遗留问题 / TODO

1. 上述未截图视图需用户目检，或后续补 E2E 截图。
2. 字体：如需模板的 Albert Sans，需先确认是否引入字体依赖。
3. 深色主题映射未经视觉验证。
4. 此前 T3 样式提交（18:54–19:43，`bd0244f`…`8aeb4e5`）未写 devlog，本文件不补记，建议用户 T3 合入前一并确认。
5. `AGENTS.md` 有未提交改动（第 108 行「提交前确认」→「代码需测试」），非本次改动，待用户确认。

## 追加：按钮按模板重做（用户反馈 2026-10-13）

- 字体：确认沿用苹方（`--font-sans` 首选 PingFang SC），不引入模板字体。
- 按钮颜色与形状对齐模板：
  - 主按钮 `.btn.is-primary`：蓝色实底白字（bg-blue-500 / hover bg-blue-600），圆角 8，字重 500。
  - 中性按钮 `.btn`：白底细边、灰字，悬停灰底 + 轻阴影（对应模板 hover:shadow-sm）。
  - 色系次要按钮 `.btn.is-blue` / `.is-green` / `.is-purple`：浅色底 + 同色系细边 + 同色字（对应模板 New Folder / Upload File / Share）。令牌见 `tokens.css` 的 `--blue-*`、`--green-*`、`--purple-*`，深色主题同步映射。
  - 图标按钮 `.btn-icon`：灰字，悬停灰底（对应模板 p-1.5 rounded-md hover:bg-gray-100），尺寸 26px。删除按钮的悬停由红色改为中性灰。
- 映射选择：信息记录「整理导出」（当前禁用，T2 后开放）改为 `is-green`，对应模板「Upload File」这类操作按钮。这是我的映射，可以改。
- 保留：按钮高度 26px（用户多次反馈要求缩小，模板 py-2 约 36px 未采用）；确认对话框的危险按钮仍为红色（模板无对应色，属语义保留）。
- 验证：`tsc -p web --noEmit` 通过；`npm run build:web` 通过（CSS 12.56KB）；临时测试页（/tmp/btn-test/index.html 引用项目 style.css）截图核对 6 种按钮变体，颜色与模板一致。
- 遗留：按钮高度是否改为模板尺寸待用户确认；危险确认按钮颜色待用户确认。

## 追加：蓝色按钮改用模板实际色值（用户反馈 2026-10-13）

- 发现：此前令牌用的是 Tailwind v3 色值（如 `#3b82f6`），而模板是 Tailwind v4，实际色值取自模板 CSS 变量 `--color-*`。
- 修正（`web/src/styles/tokens.css`，浅色）：
  - 主色 `--primary` `#3b82f6` → `#3080ff`（blue-500）；悬停 `--primary-hover` → `#155dfc`（blue-600）；激活字 `--primary-text` → `#1447e6`（blue-700）；聚焦环 `--ring` → `#90c5ff`（blue-300）。
  - 色系次要按钮：蓝字 `--blue-text` `#155dfc`、蓝边 `--blue-border` `#bedbff`；绿字 `#00a544`、绿边 `#b9f8cf`；紫字 `#9810fa`。
- 影响面：`--primary` 为全局强调色，同时作用于侧栏激活态、头像描边、待办勾选等，已一并变为模板蓝。
- 未改：中性灰（gray 系）与模板 v4 灰值差异极小，暂未替换；深色主题映射未变。
- 验证：`tsc -p web --noEmit` 通过；`npm run build:web` 通过（CSS 12.56KB）；测试页截图确认蓝色按钮与模板一致。

## 追加：按钮尺寸对齐模板（用户反馈 2026-10-13）

- 依据模板 Tailwind 类换算：文字按钮 `py-2 px-4 text-sm` → 高 36px、左右内边距 16px、字号 14px、间距 8px；图标按钮 `p-1.5` + `w-4 h-4` 图标 → 28px。
- 变更（`web/src/style.css`）：
  - `.btn`：高 36px（替代 26px）、padding `0 16px`、字号 14px、行高 20px、gap 8px、nowrap。
  - `.btn-icon`：28px（替代 26px，原 20px 已在上一轮改为 26）。
  - `.filter-tab` / `.seg-btn`：高 28px、padding `0 12px`、字号 14px（分段控件容器保持 2px 内边距，总高 32px）。
  - `.side-item`（导航项，也是按钮）：padding `10px 12px`、gap 12px，对应模板 `py-2.5 px-3 gap-3`，高 40px。
  - `.todo-input`：高 36px、padding `0 16px`、字号 14px，与「添加」按钮同高（沿用用户此前对「输入框与按钮等高」的要求）。
- 未改：弹层表单输入框 `.field input`（非按钮，仍为原尺寸）。
- 验证：`tsc -p web --noEmit` 通过；`npm run build:web` 通过（CSS 12.79KB）；测试页截图核对按钮/图标按钮/分段控件尺寸；主界面截图确认待办输入框与添加按钮同高。

## 追加：头像更新为本地图片（用户反馈 2026-10-13）

- 素材：`/Users/eric/Downloads/千图_Q版头像…（仅供交流学习非商用）/e6a84d142cea46679bb596dba6124559.png`（2048×2048，2.7MB）。已用 `sips -Z 256` 缩至 256×256（约 84KB）放入 `web/src/assets/avatar.png`。素材来源为千图（58pic），文件夹标注「仅供交流学习非商用」，仅用于本个人工作台；如要发布或商用需替换素材。
- 新增 `web/src/components/Avatar.tsx`：统一头像渲染（`<img>` + `alt="{name} 的头像"`），侧栏（`Sidebar.tsx`）与设置页（`SettingsView.tsx`）共用，替代原先的姓名首字。
- `web/src/style.css`：`.avatar` 改为 `overflow:hidden`，内部 `img` 圆形裁切 `object-fit: cover`，保留 44px 与 2px 主色描边。
- 设置页头像行提示文字由「当前显示姓名首字」改为「当前为默认头像」。
- 验证：`tsc -p web --noEmit` 通过；`npm run build:web` 通过（CSS 12.79KB，JS 240.41KB）；主界面截图确认头像显示正常。
- 遗留：头像当前为内置默认图，自定义上传仍属 T2（设置页提示文字已注明）。

## 交接（会话收尾，重启前记录）

### 当前状态
- 分支 `feature/m0-scaffold`；T1 脚手架、T3 三栏壳已完成，T3 样式重构已完成但**未提交**。
- 未提交改动：`web/src/style.css`、`web/src/styles/tokens.css`、`web/src/state/app-store.tsx`、`web/src/panels/Sidebar.tsx`、`web/src/panels/views/{LinkView,NoteView,SettingsView}.tsx`、`docs/design/ui-baseline.md`，新增 `web/src/components/Avatar.tsx`、`web/src/assets/avatar.png`、本 devlog。
- `AGENTS.md` 有一处未提交改动（第 108 行「提交前确认」→「代码需测试」），非本次改动，待用户确认。
- 无运行中的 dev 服务。

### 已定样式决策（用户确认）
- 字体：沿用苹方，不引入模板字体。
- 颜色：按钮与主色采用模板实测值（Tailwind v4），蓝 `#3080ff` / 悬停 `#155dfc`。
- 尺寸：文字按钮 36px、图标按钮 28px、分段控件 28px、导航项 40px；待办输入框与添加按钮同高。
- 头像：素材 `e6a84d142cea46679bb596dba6124559.png` 已缩至 256px 放入 `web/src/assets/avatar.png`（素材标注「仅供交流学习非商用」）。

### 待用户决定
1. 删除确认按钮（`web/src/components/dialogs.tsx` 的危险按钮）仍为红色，模板无对应色，是否保留？
2. `AGENTS.md` 未提交改动是否保留？
3. 目检：`npm run dev` 查看信息记录、快捷入口、设置、弹层、深色主题，以及 <1200px 抽屉（未核验项）。
4. 目检通过后，是否把 T3 样式改动提交到 `feature/m0-scaffold` 并合入 T3？（提交需你确认。）

### 下一步（按 PRD 顺序）
1. 目检 → 提交 T3 → 合入。
2. 补齐测试：仓库目前无任何 `*_test.go` / `*.test.ts(x)`，AGENTS.md §5 要求前端状态同步与 Go 侧单测，属既有缺口。
3. T2 数据层：迁移机制 + settings/todos/notes/quick_links 四表 + store 层 + AppService，替换 mock；头像上传、主题与姓名持久化同步接入。
4. T4 待办 → T5 信息记录（Milkdown 集成冒烟先行）→ T6 快捷入口。
5. T7：E2E 主链路 + `mygo build` 打包冒烟（macOS）。

### 恢复工作时的注意
- 先读本文件与 `docs/prd/M0-工作台MVP.md` §9–10，再继续。
- 验证命令：`npx tsc -p web --noEmit`、`npm run build:web`；截图可用无头 Chrome（`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome --headless=new --no-sandbox --disable-gpu --screenshot=…`，需后台运行并轮询输出文件，否则 bash 60 秒超时会被杀）。

## 目检结果（2026-10-13）

- 用户目检通过，同意提交 T3 样式改动到 `feature/m0-scaffold`。
- 未纳入本次提交：`AGENTS.md` 的「提交前确认」→「代码需测试」改动（待用户单独确认）、`.agents/`（mygo 技能目录）。
