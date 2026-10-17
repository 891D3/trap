import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "Trap",
  identifier: "app.trap.desktop",
  version: "0.3.1",
  // `mygo dev` 运行 devCommand 并加载 devUrl；`mygo build` 运行
  // buildCommand 并把 frontendDist 嵌入应用。
  devUrl: "http://localhost:5173",
  devCommand: "npm run dev:web",
  buildCommand: "npm run build:web",
  frontendDist: "web/dist",
  bindings: "web/src/mygo.ts",
  out: "build",
});