import { useEffect, useState } from 'react';
import { IconDashboard } from './components/icons';
import { Auxiliary } from './panels/Auxiliary';
import { MainArea } from './panels/MainArea';
import { Sidebar } from './panels/Sidebar';
import { LinkView } from './panels/views/LinkView';
import { NoteView } from './panels/views/NoteView';
import { SettingsView } from './panels/views/SettingsView';
import { TodoView } from './panels/views/TodoView';
import { useStore, useTheme, type ViewId } from './state/app-store';

const VIEW_META: Record<ViewId, { title: string; desc: string }> = {
  dashboard: { title: '看板', desc: '效率与成长轨迹（M2 开放）' },
  todo: { title: '待办事项', desc: '随手记、一键完成' },
  note: { title: '信息记录', desc: '灵感与重要事项，markdown 沉淀' },
  link: { title: '快捷入口', desc: '常用工具与网站，一键直达' },
  settings: { title: '设置', desc: '资料、主题与数据' },
};

/** 窄屏（<1200px）标记：右栏折叠为抽屉、侧栏自动收起（ui-baseline 通用规则）。 */
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

export function App() {
  const { doneToday } = useStore();
  useTheme(); // 解析主题设置并写入 <html data-theme>

  const [view, setView] = useState<ViewId>(() => {
    // 支持深链接初始视图；正常启动没有参数时仍默认进入待办。
    const requested = new URLSearchParams(window.location.search).get('e2eView');
    return requested && requested in VIEW_META ? (requested as ViewId) : 'todo';
  });
  const [collapsed, setCollapsed] = useState(false);
  const [auxOpen, setAuxOpen] = useState(false);
  const narrow = useNarrow();

  return (
    <div className="app">
      <Sidebar
        view={view}
        collapsed={narrow ? true : collapsed}
        onSelectView={setView}
        onToggleCollapsed={() => setCollapsed((c) => !c)}
      />

      <MainArea
        meta={VIEW_META[view] ?? VIEW_META.todo}
        doneToday={doneToday}
        showStat={view === 'todo'}
        showAuxToggle={narrow}
        auxOpen={auxOpen}
        onToggleAux={() => setAuxOpen((o) => !o)}
      >
        {view === 'dashboard' && (
          <div className="aux-placeholder" style={{ marginTop: 24 }}>
            <IconDashboard />
            <span>看板 · M2 开放</span>
          </div>
        )}
        {view === 'todo' && <TodoView />}
        {view === 'note' && <NoteView />}
        {view === 'link' && <LinkView />}
        {view === 'settings' && <SettingsView />}
      </MainArea>

      {!narrow && <Auxiliary />}
      {narrow && (
        <>
          {auxOpen && <div className="overlay" style={{ alignItems: 'stretch', justifyContent: 'flex-end' }} onClick={() => setAuxOpen(false)} />}
          {auxOpen && <Auxiliary />}
        </>
      )}
    </div>
  );
}