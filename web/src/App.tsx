// App —— v3 应用壳（ui-baseline §二）：
//   侧边导航（216px 可收起 64px）| 主区域列（顶栏 64 + 内容区）| 辅助区（256px 可收起）。
//   默认视图 = 数据看板（PRD v2.0 拟定决策 1；深链接 e2eView 仍可用）。
//   <1200px：侧栏自动折叠、辅助区折叠为抽屉（ui-baseline §七）。
import { useEffect, useState } from 'react';
import { Auxiliary } from './panels/Auxiliary';
import { MainArea } from './panels/MainArea';
import { Sidebar } from './panels/Sidebar';
import { LinkView } from './panels/views/LinkView';
import { NoteView } from './panels/views/NoteView';
import { SettingsView } from './panels/views/SettingsView';
import { StockView } from './panels/views/StockView';
import { TodoView } from './panels/views/TodoView';
import { useStore, useTheme, type ViewId } from './state/app-store';

/** 窄屏（<1200px）标记：右栏折叠为抽屉、侧栏自动收起。 */
function useNarrow(): boolean {
  const [narrow, setNarrow] = useState(() => window.matchMedia('(max-width: 1199px)').matches);
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 1199px)');
    const onChange = () => setNarrow(mq.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  }, []);
  return narrow;
}

/** 设置页不显示辅助区（ui-baseline §八映射表）。 */
function auxEnabled(view: ViewId): boolean {
  return view !== 'settings';
}

export function App() {
  useTheme(); // 解析主题设置并写入 <html data-theme>

  const [view, setView] = useState<ViewId>(() => {
    // 深链接初始视图（E2E 用）；默认 = 数据看板（PRD v2.0 拟定决策 1）。
    const requested = new URLSearchParams(window.location.search).get('e2eView');
    return requested && VIEW_IDS.has(requested as ViewId) ? (requested as ViewId) : 'dashboard';
  });
  const [collapsed, setCollapsed] = useState(false);
  const [auxOpen, setAuxOpen] = useState(false);
  const narrow = useNarrow();
  const showAux = auxEnabled(view);

  return (
    <div className="app">
      <Sidebar
        view={view}
        collapsed={narrow ? true : collapsed}
        onSelectView={setView}
        onToggleCollapsed={() => setCollapsed((c) => !c)}
      />

      <MainArea view={view}>
        {view === 'dashboard' && null /* R4 数据看板视图 */}
        {view === 'todo' && <TodoView />}
        {view === 'note' && <NoteView />}
        {view === 'link' && <LinkView />}
        {view === 'settings' && <SettingsView />}
        {view === 'stock' && <StockView />}
      </MainArea>

      {showAux && !narrow && <Auxiliary view={view} />}
      {showAux && narrow && (
        <>
          {auxOpen && (
            <div
              className="overlay"
              style={{ alignItems: 'stretch', justifyContent: 'flex-end' }}
              onClick={() => setAuxOpen(false)}
            />
          )}
          {auxOpen && <Auxiliary view={view} />}
        </>
      )}
    </div>
  );
}

const VIEW_IDS = new Set<ViewId>(['dashboard', 'todo', 'note', 'stock', 'link', 'settings']);
