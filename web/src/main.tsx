import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { AppStoreProvider } from './state/app-store';
import './style.css';

// T3 壳：React 挂载点。typed IPC 链路已在 T1 以 AppService.Ping 验证；
// T2 起 UI 经 web/src/api/ 调用 Go 服务，事件经 mygo Events 同步。
const root = document.querySelector<HTMLDivElement>('#root');
if (!root) throw new Error('#root 挂载点缺失');

createRoot(root).render(
  <StrictMode>
    <AppStoreProvider>
      <App />
    </AppStoreProvider>
  </StrictMode>,
);