# 2026-10-16 安装 Rust 工具链以修复 pi 图片附件失败

## 做了什么

- 定位 pi 警告 `Warning: Could not attach .../PixPin_2026-10-16_20-33-35.png: cargo was not found. Install Rust from https://rustup.rs to build native tools.` 的来源。
- 安装 Rust 工具链：`brew install rust` → 得到 `cargo 1.99.0 (5f94df478 2026-08-27) (Homebrew)` / `rustc 1.99.0 (b940084d7 2026-09-28) (Homebrew)`，位于 `/opt/homebrew/bin/{cargo,rustc}`。
- 预编译图片附件依赖的原生工具 `view-image`，复现 pi 扩展自身的编译逻辑：
  `cargo install --git https://github.com/luan/agents --rev dac9040e408c92aae5d375cd3218af31748d8146 --root ~/.pi/agent/native/view-image/dac9040e408c92aae5d375cd3218af31748d8146 --locked view-image`
- 验证产物可执行并成功解析此前失败的图片。

## 为什么这样做

- **根因**：pi 的图片附件扩展 `@luan.sh/pi-view-image`（`~/.pi/agent/npm/node_modules/@luan.sh/pi-view-image/src/extension.ts:94`）本身不读图，而是把路径交给 Rust 写的 `view-image` 二进制做读取/校验/缩放，再转成模型可消费的附件。该二进制无预编译分发，pi 设计为首次使用时经 `cargo install` 现场编译到 `~/.pi/agent/native/view-image/<rev>/bin/`（实现见 `@luan.sh/pi-libtui/src/native-binary.ts` 的 `ensureNativeBinary` / `runCargo`）。本机此前无 Rust，`spawn("cargo", ...)` 直接 `ENOENT`，故抛出该错误。
- **选择 brew 而非 rustup（用户指定方案 2）**：brew 由系统包管理器统一管理、升级与卸载路径清晰，不引入 `~/.rustup` 第二套工具链；代价是版本由 brew 决定而非 `rustup toolchain` 可切换。用户明确选择此项。
- **预编译而非等 pi 触发**：`~/.pi/agent/native/view-image/dac9040e.../` 此前已是空目录（创建过但无 `bin/view-image`），若不预热，用户下次贴图仍需现场等待约 40s 编译。预热后贴图即时可用，且能提前暴露编译失败。

## 验证结果

- `brew install rust` ✓：`cargo 1.99.0` / `rustc 1.99.0` 均在 PATH 中可用。
- `cargo install ... view-image` ✓：`Finished 'release' profile [optimized] target(s) in 39.87s`，安装至 `~/.pi/agent/native/view-image/dac9040e408c92aae5d375cd3218af31748d8146/bin/view-image`。
- 产物校验 ✓：`Mach-O 64-bit executable arm64`，1,819,872 字节，具备可执行位。
- 端到端复现 ✓：对报错涉及的 `PixPin_2026-10-16_20-33-35.png`（PNG 2142x292 RGBA）执行
  `view-image '{"path":"...","detail":"high"}'` → `exit=0`、stderr 为空、stdout 输出 83,286 字节 `{"image_url":"data:image/png;base64,..."}`。说明图片本身完好，此前失败纯属缺工具链。
- 未运行 Trap 仓库自身测试：本次改动不触及仓库代码（`git status` 干净），故无 Go/前端测试需重跑。

## 遗留问题 / TODO

- 需**重启 pi 会话**后扩展才会重新解析二进制路径；当前会话内该图片仍未附加进上下文。
- `brew` 的 rust 为独立工具链（非 rustup 管理）。若后续 `@luan.sh/pi-libtui` 提升 `native-revision`，需重新 `cargo install` 新 rev 目录；此过程可重复本节命令。
- 若将来需要多版本 Rust 工具链切换，需再补 `brew install rustup && rustup toolchain link system "$(brew --prefix rust)"`。
- 本次未修改 `AGENTS.md` 交付检查单；是否把「Rust 工具链」写入环境依赖清单需用户确认。
