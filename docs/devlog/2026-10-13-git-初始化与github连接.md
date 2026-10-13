# 2026-10-13 git 初始化与 GitHub 连接

## 做了什么

- `git init -b main` 初始化仓库（main 分支）。
- 编写 `.gitignore`：排除 macOS（`.DS_Store`）、Go 产物（`*.test`/`build/`）、前端（`node_modules/`/`dist/`/`.vite/`）、**运行时数据（`*.db` 及 wal/journal/shm、`data/`）**、环境与密钥（`.env*`/`*.key`/`*.pem`/`secrets/`）、编辑器文件。
- 初始提交 `99ac550`（AGENTS.md、BRD v0.2、M0 需求评估、devlog、提示词记录，共 6 文件 610 行）。
- 配置 remote `origin → https://github.com/891D3/trap.git` 并推送，`main` 与 `origin/main` 已同步（远端 HEAD `99ac550`，已验证）。

## 为什么这样做（含被否决的备选方案）

- 提交范围只含文档与 `.gitignore`：`.agents/`（审批上下文配置）属本地会话环境文件，暂不入库；数据文件被 `.gitignore` 硬排除，符合「数据本地化」硬约束（SQLite 绝不入库）。
- main 分支仅作初始化基线；后续开发一律走 feature 分支（AGENTS.md 硬约束）。
- 被否决：用 gh CLI 建仓（gh 未安装，且用户已自行建好仓库）；SSH remote（未确认用户 SSH 配置，HTTPS + 现有凭证先跑通）。

## 验证结果（跑了哪些测试、结果如何）

- 只读验证：`git status --short --branch` 显示 `## main...origin/main` 无 ahead/behind；`git ls-remote --heads origin` 返回 `99ac550` 与本地一致。
- 无代码改动，无测试执行。

## 遗留问题 / TODO

1. **会话保护层拦截 git 写操作**：agent 的 shell 对 `.git` 目录的写入（add/commit/remote add/push）被硬性拦截，对话授权无法解除；本次提交/推送由用户手动执行。后续涉及 git 提交的工作单元需预留此协作方式，或由用户调整保护规则。
2. `gh` CLI 未安装；后续如需建 PR/发 Release（M2 自动更新）建议 `brew install gh`。
3. 仓库 `891D3/trap` 当前为 **public**：BRD/devlog 对外可见；如介意可在 GitHub 设置转 private（M2 前需定，涉及 Releases 与签名发布策略）。
4. `.agents/` 仍未入库（有意为之），如后续决定纳入需用户确认。