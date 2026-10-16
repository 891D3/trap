// Topbar —— v3 设计基线（ui-baseline §二）：64px 白底顶栏 = 页面标题 + 面包屑
// + 全局搜索框（样式占位，M0 不做检索后端）+ 操作组（通知/筛选占位、主题切换）。
//
// 主题切换按钮循环 system → light → dark（与设置页选项一致；持久化经 settings）。
import type { ThemeSetting, ViewId } from '../state/app-store';
import { useStore } from '../state/app-store';
import { IconBell, IconFilter, IconMonitor, IconMoon, IconPanelLeftOpen, IconSearch, IconSun } from '../components/icons';

export const VIEW_TITLE: Record<ViewId, { title: string; crumb: string }> = {
  dashboard: { title: '数据看板', crumb: '个人工作台 / 数据看板' },
  todo: { title: '待办事项', crumb: '个人工作台 / 待办事项' },
  note: { title: '信息记录', crumb: '个人工作台 / 信息记录' },
  stock: { title: '股票看台', crumb: '个人工作台 / 股票看台' },
  link: { title: '快捷入口', crumb: '个人工作台 / 快捷入口' },
  settings: { title: '设置', crumb: '个人工作台 / 设置' },
};

const THEME_ORDER: ThemeSetting[] = ['system', 'light', 'dark'];

const THEME_ICON: Record<ThemeSetting, React.ReactNode> = {
  system: <IconMonitor />,
  light: <IconSun />,
  dark: <IconMoon />,
};

const THEME_LABEL: Record<ThemeSetting, string> = {
  system: '跟随系统',
  light: '浅色',
  dark: '深色',
};

type TopbarProps = {
  view: ViewId;
  /** 辅助区收起后出示展开钮（v3 目检：展开入口放顶栏）。 */
  auxExpand?: { visible: boolean; onExpand: () => void };
};

export function Topbar({ view, auxExpand }: TopbarProps) {
  const meta = VIEW_TITLE[view] ?? VIEW_TITLE.todo;
  const { theme, setTheme } = useStore();

  const nextTheme = (): void => {
    const index = THEME_ORDER.indexOf(theme);
    const next = THEME_ORDER[(index + 1) % THEME_ORDER.length] ?? 'system';
    void setTheme(next);
  };

  return (
    <header className="topbar">
      <div className="topbar-title-group">
        <h1 className="main-title">{meta.title}</h1>
        <span className="topbar-crumb">{meta.crumb}</span>
      </div>

      {/* M0 样式占位：全局检索后端不做（PRD v2.0 §1 范围外）。 */}
      <div className="topbar-search">
        <IconSearch />
        <input
          aria-label="全局搜索"
          placeholder="搜索待办、记录、项目…"
          disabled
          title="全局搜索（后续版本开放）"
        />
      </div>

      <div className="topbar-actions">
        <button className="topbar-icon-btn has-dot" aria-label="通知" title="通知（占位）" disabled>
          <IconBell />
        </button>
        <button className="topbar-icon-btn" aria-label="筛选" title="筛选（占位）" disabled>
          <IconFilter />
        </button>
        <button
          className="topbar-icon-btn"
          aria-label="切换主题"
          title={`主题：${THEME_LABEL[theme]}（点击切换）`}
          onClick={nextTheme}
        >
          {THEME_ICON[theme]}
        </button>
        {auxExpand?.visible && (
          <button
            className="topbar-icon-btn"
            aria-label="展开辅助区"
            title="展开辅助区"
            onClick={auxExpand.onExpand}
          >
            <IconPanelLeftOpen />
          </button>
        )}
      </div>
    </header>
  );
}
