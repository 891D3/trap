import { defineConfig } from "vite";

export default defineConfig({
  // 前端源码位于 web/（PRD §9 目录约定）；构建产物 web/dist 由 mygo build 嵌入。
  root: "web",
  server: {
    // mygo dev 期间应用加载此地址（mygo.config.ts 的 devUrl）。
    port: 5173,
    strictPort: true,
    // 开发期应用与打包产物目录不属于前端源码。
    watch: { ignored: ["**/.mygo/**", "**/build/**"] },
  },
});