#!/usr/bin/env bash
# build.sh —— 带编译号的打包脚本。
#
# 版本号规则：v<base>-<build code>，build code 取当前 git 短 hash
# （如 0.3.1-826e3ac）。每次构建自动携带可追溯的编译标识，无需手工维护。
#
# 原理：临时把 mygo.config.ts 的 version 与 internal/service/app.go 的
# appVersion 改成 <base>-<hash>，npm run build（mygo build 读取两者），
# 构建完成后还原工作区（版本号不入库，发布 tag 仍用 base）。
set -euo pipefail
cd "$(dirname "$0")/.."

BASE=$(sed -n 's/.*version: "\([^"]*\)".*/\1/p' mygo.config.ts | head -1)
CODE=$(git rev-parse --short HEAD)
VERSION="${BASE}-${CODE}"

echo "==> 构建版本: v${VERSION}"

# 1) 临时改版本号（mygo.config.ts + Go 侧 appVersion）
sed -i '' "s|version: \"${BASE}\"|version: \"${VERSION}\"|" mygo.config.ts
sed -i '' "s|const appVersion = \"${BASE}\"|const appVersion = \"${VERSION}\"|" internal/service/app.go

# 2) 无论如何退出都还原文件（版本号不入库）
restore() {
  git checkout -- mygo.config.ts internal/service/app.go
}
trap restore EXIT

# 3) 打包
npm run build
echo "==> 完成: build/ 下产物版本 v${VERSION}"
