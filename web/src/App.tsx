// App —— v3 应用壳（ui-baseline §二）：
//   侧边导航（216px 可收起 64px）| 主区域列（顶栏 64 + 内容区）| 辅助区（256px 可收起）。
//   默认视图 = 数据看板（PRD v2.0 拟定决策 1；深链接 e2eView 仍可用）。
//   <1200px：侧栏自动折叠、辅助区折叠为抽屉（ui-baseline §七）。
import { useEffect, useRef, useState } from 'react';
import { Auxiliary } from './panels/Auxiliary';
import { MainArea } from './panels/MainArea';
import { Sidebar } from './panels/Sidebar';
import { DashboardView } from './panels/views/DashboardView';
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
  const { view, setView } = useStore();

  // 深链接初始视图（E2E 用）：仅首次挂载时覆盖 store 默认值（数据看板）。
  const deepLinkRef = useRef(false);
  useEffect(() => {
    if (deepLinkRef.current) return;
    deepLinkRef.current = true;
    const requested = new URLSearchParams(window.location.search).get('e2eView');
    if (requested && VIEW_IDS.has(requested as ViewId)) setView(requested as ViewId);
  }, [setView]);

  const [collapsed, setCollapsed] = useState(false);
  const [auxOpen, setAuxOpen] = useState(false);
  // v3 修正③：辅助区可收起/展开（会话内状态，不跨会话持久——与侧栏收起一致）。
  const [auxCollapsed, setAuxCollapsed] = useState(false);
  const narrow = useNarrow();
  const showAux = auxEnabled(view) && !auxCollapsed;

  return (
    <div className="app">
      {/* 标题栏条（红绿灯之右）：项目标签页 + 可拖动窗口（TitleBarStyle=hidden） */}
      <TitleStrip view={view} />
      <div className="app-body">
      <Sidebar
        view={view}
        collapsed={narrow ? true : collapsed}
        onSelectView={setView}
        onToggleCollapsed={() => setCollapsed((c) => !c)}
      />

      <MainArea
        view={view}
        auxExpand={{ visible: auxEnabled(view) && auxCollapsed && !narrow, onExpand: () => setAuxCollapsed(false) }}
      >
        {view === 'dashboard' && <DashboardView />}
        {view === 'todo' && <TodoView />}
        {view === 'note' && <NoteView />}
        {view === 'link' && <LinkView />}
        {view === 'settings' && <SettingsView />}
        {view === 'stock' && <StockView />}
      </MainArea>
      </div>

      {/* 宽屏：辅助区常驻挂载以支持与侧栏同套宽度动画 */}
      {auxEnabled(view) && !narrow && (
        <Auxiliary
          view={view}
          collapsed={auxCollapsed}
          onCollapse={() => setAuxCollapsed(true)}
        />
      )}
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

/** 标题栏条：红绿灯右侧为项目标签页；其余区域负责拖动窗口（--app-region: drag）。 */
function TitleStrip({ view }: { view: ViewId }): React.JSX.Element {
  const { projects, projectTabs, projectFilter, openProject, closeProjectTab } = useStore();
  const tabs = projectTabs
    .map((id) => projects.find((p) => p.id === id))
    .filter((p): p is NonNullable<typeof p> => p != null);
  void view;
  return (
    <div className="titlebar">
      <div className="titlebar-tabs">
        {tabs.length === 0 && <span className="titlebar-hint">Trap</span>}
        {tabs.map((p) => (
          <button
            key={p.id}
            className={`topbar-tab${projectFilter === p.id ? ' is-active' : ''}`}
            onClick={() => openProject(p.id)}
            title={p.name}
          >
            <span className={`side-dot proj-${p.color}`} />
            <span className="topbar-tab-name">{p.name}</span>
            <span
              className="topbar-tab-close"
              role="button"
              aria-label={`关闭标签 ${p.name}`}
              onClick={(e) => {
                e.stopPropagation();
                closeProjectTab(p.id);
              }}
            >
              ×
            </span>
          </button>
        ))}
      </div>
      <div className="titlebar-drag" style={{ flex: 1, height: '100%' }} />
    </div>
  );
}
