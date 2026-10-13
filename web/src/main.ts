// T1 脚手架占位入口；T3 将替换为 React 应用挂载（三栏壳）。
// AppService 为占位服务，验证 mygo typed IPC 端到端链路（Go Ping → 生成客户端 → 页面）。
import { AppService } from "./mygo";
import "./style.css";

const $ = <T extends HTMLElement>(selector: string) => document.querySelector<T>(selector)!;

$("#root").textContent = `Trap · typed IPC: ${await AppService.ping()}`;