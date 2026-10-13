import type { ViewId } from '../state/app-store';
import type { ReactNode } from 'react';
import {
  IconDashboard,
  IconLink,
  IconNote,
  IconPanelLeftClose,
  IconPanelLeftOpen,
  IconSettings,
  IconTodo,
} from '../components/icons';
import { useStore } from '../state/app-store';

type SidebarProps = {
  view: ViewId;
  collapsed: boolean;
  onSelectView: (v: ViewId) => void;
  onToggleCollapsed: () => void;
};

const NAV_ITEMS: { id: ViewId; label: string; icon: ReactNode }[] = [
  { id: 'dashboard', label: '看板', icon: <IconDashboard /> },
  { id: 'todo', label: '待办事项', icon: <IconTodo /> },
  { id: 'note', label: '信息记录', icon: <IconNote /> },
  { id: 'link', label: '快捷入口', icon: <IconLink /> },
];

export function Sidebar({ view, collapsed, onSelectView, onToggleCollapsed }: SidebarProps) {
  const { user } = useStore();

  return (
    <div className={`side-rail${collapsed ? ' is-collapsed' : ''}`}>
      <aside className="sidebar">
        <div className="side-user">
          <div className="avatar">{user.name.charAt(0)}</div>
          <div className="side-label side-user-line">
            <span className="side-user-name">{user.name}</span>
            <span className="side-user-sub">的工作台</span>
          </div>
        </div>

        <nav className="side-nav">
          {NAV_ITEMS.map((item) => (
            <button
              key={item.id}
              className={`side-item${view === item.id ? ' is-active' : ''}`}
              onClick={() => onSelectView(item.id)}
              title={collapsed ? item.label : undefined}
            >
              {item.icon}
              <span className="side-label">{item.label}</span>
            </button>
          ))}
        </nav>

        <div className="side-group side-label">项目</div>
        <button className="side-item" disabled title="项目工作 M1 开放">
          <span
            style={{ width: 8, height: 8, borderRadius: 4, background: 'var(--text-3)', flex: 'none' }}
          />
          <span className="side-label">项目（M1 开放）</span>
        </button>

        <div className="side-spacer" />

        <div className="side-bottom">
          <button
            className={`side-item${view === 'settings' ? ' is-active' : ''}`}
            onClick={() => onSelectView('settings')}
            title={collapsed ? '设置' : undefined}
          >
            <IconSettings />
            <span className="side-label">设置</span>
          </button>
          <button
            className="side-item"
            onClick={onToggleCollapsed}
            title={collapsed ? '展开侧栏' : '收起侧栏'}
            aria-label={collapsed ? '展开侧栏' : '收起侧栏'}
          >
            {collapsed ? <IconPanelLeftOpen /> : <IconPanelLeftClose />}
            <span className="side-label">{collapsed ? '展开侧栏' : '收起侧栏'}</span>
          </button>
        </div>
      </aside>
    </div>
  );
}