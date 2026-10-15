// Sidebar —— v3 设计基线（ui-baseline §二）的侧边导航栏：
//   用户信息区（头像+姓名+副标+收起按钮）→「工作台」菜单（5 项，待办带角标，
//   股票看台占位）→「项目」列表（色点+名称+计数+新建/编辑）→ 底部「设置」。
// 导航 active 态用左侧 3px 主色指示条（CSS ::before），禁止色块高亮。
import { useState } from 'react';
import type { ReactNode } from 'react';
import {
  IconDashboard,
  IconDots,
  IconLink,
  IconNote,
  IconPanelLeftClose,
  IconPanelLeftOpen,
  IconPlus,
  IconSettings,
  IconStock,
  IconTodo,
} from '../components/icons';
import type { Project, ViewId } from '../state/app-store';
import { useStore } from '../state/app-store';
import { Avatar } from '../components/Avatar';
import { ConfirmDialog, CustomDialog } from '../components/dialogs';

type SidebarProps = {
  view: ViewId;
  collapsed: boolean;
  onSelectView: (v: ViewId) => void;
  onToggleCollapsed: () => void;
};

const NAV_ITEMS: { id: ViewId; label: string; icon: ReactNode; badge?: boolean }[] = [
  { id: 'dashboard', label: '数据看板', icon: <IconDashboard /> },
  { id: 'todo', label: '待办事项', icon: <IconTodo />, badge: true },
  { id: 'note', label: '信息记录', icon: <IconNote /> },
  { id: 'stock', label: '股票看台', icon: <IconStock /> }, // M2+ 占位视图
  { id: 'link', label: '快捷入口', icon: <IconLink /> },
];

/** 项目颜色循环分配顺序（设计稿实测四色；store 层校验同名 token）。 */
export const PROJECT_COLORS: { token: string; label: string }[] = [
  { token: 'slate', label: '主色蓝灰' },
  { token: 'amber', label: '琥珀' },
  { token: 'green', label: '绿' },
  { token: 'purple', label: '紫' },
];

/** 项目编辑弹层（新建/重命名/改色 + 删除确认）。 */
type ProjectDraft = {
  id: number | null; // null = 新建
  name: string;
  color: string;
};

export function Sidebar({ view, collapsed, onSelectView, onToggleCollapsed }: SidebarProps) {
  const { user, todos, projects, addProject, editProject, deleteProject, openProject, projectFilter } = useStore();

  // 新建按钮的迷你弹层（名称输入 + 色盘），局部 state 足够（契约：收起态不跨会话持久）。
  const [draft, setDraft] = useState<ProjectDraft | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Project | null>(null);

  const undoneCount = todos.filter((todo) => !todo.done).length;

  const submitDraft = async (): Promise<void> => {
    if (!draft) return;
    try {
      if (draft.id === null) {
        await addProject(draft.name, draft.color);
      } else {
        await editProject(draft.id, draft.name, draft.color);
      }
      setDraft(null);
    } catch {
      // 错误已在输入弹层顶部展示（InputDialog children 由调用方渲染）——保持弹层开启。
    }
  };

  return (
    <div className={`side-rail${collapsed ? ' is-collapsed' : ''}`}>
      <aside className="sidebar">
        {/* 用户信息区：头像 44 + 姓名/副标 + 收起按钮（v3 在顶部，不落底部）。 */}
        <div className="side-user">
          <Avatar name={user.name} />
          <div className="side-label side-user-line">
            <span className="side-user-name">{user.name}</span>
            <span className="side-user-sub">个人工作台</span>
          </div>
          <button
            className="side-collapse-btn"
            onClick={onToggleCollapsed}
            title={collapsed ? '展开侧栏' : '收起侧栏'}
            aria-label={collapsed ? '展开侧栏' : '收起侧栏'}
          >
            {collapsed ? <IconPanelLeftOpen /> : <IconPanelLeftClose />}
          </button>
        </div>

        <nav className="side-nav">
          <div className="side-group-title side-label">工作台</div>
          {NAV_ITEMS.map((item) => (
            <button
              key={item.id}
              className={`side-item${view === item.id ? ' is-active' : ''}`}
              onClick={() => onSelectView(item.id)}
              title={collapsed ? item.label : undefined}
            >
              {view === item.id && <span className="nav-ind" aria-hidden="true" />}
              {item.icon}
              <span className="side-label">{item.label}</span>
              {item.badge && undoneCount > 0 && (
                <span className="side-badge">{undoneCount > 99 ? '99+' : undoneCount}</span>
              )}
            </button>
          ))}
        </nav>

        {/* 项目列表区：标题行（项目 + 新建）+ 项目行（色点/名称/计数）。 */}
        <div className="side-projects">
          <div className="side-project-head side-label">
            <span>项目</span>
            <button
              className="side-mini-add"
              aria-label="新建项目"
              title="新建项目"
              onClick={() => setDraft({ id: null, name: '', color: PROJECT_COLORS[0]?.token ?? 'slate' })}
            >
              <IconPlus size={12} />
            </button>
          </div>
          {projects.map((project) => (
            <div key={project.id} className="side-project-row">
              <button
                className={`side-project${view === 'todo' && projectFilter === project.id ? ' is-active' : ''}`}
                onClick={() => openProject(project.id)}
                title={collapsed ? project.name : undefined}
              >
                <span className={`side-dot proj-${project.color}`} />
                <span className="side-label side-project-name">{project.name}</span>
                <span className="side-project-count side-label">{project.openTodos > 0 ? project.openTodos : ''}</span>
              </button>
              <button
                className="side-project-edit side-label"
                aria-label={`编辑项目 ${project.name}`}
                onClick={() => setDraft({ id: project.id, name: project.name, color: project.color })}
              >
                <IconDots size={14} />
              </button>
            </div>
          ))}
          {projects.length === 0 && (
            <div className="side-project-empty side-label">还没有项目</div>
          )}
        </div>

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
        </div>
      </aside>

      {/* 新建/编辑项目弹层。 */}
      {draft && (
        <CustomDialog
          title={draft.id === null ? '新建项目' : '编辑项目'}
          confirmText={draft.id === null ? '创建' : '保存'}
          onConfirm={() => void submitDraft()}
          onCancel={() => setDraft(null)}
        >
          <input
            className="dialog-input"
            aria-label="项目名称"
            value={draft.name}
            placeholder="项目名称"
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
          />
          <div className="color-dot-row">
            {PROJECT_COLORS.map((c) => (
              <button
                key={c.token}
                className={`color-dot proj-${c.token}${draft.color === c.token ? ' is-selected' : ''}`}
                aria-label={`颜色-${c.token}`}
                onClick={() => setDraft({ ...draft, color: c.token })}
              />
            ))}
          </div>
          {draft.id !== null && (
            <button
              type="button"
              className="dialog-danger-link"
              onClick={() => {
                const target = projects.find((p) => p.id === draft.id);
                if (target) setPendingDelete(target);
                setDraft(null);
              }}
            >
              删除项目…
            </button>
          )}
        </CustomDialog>
      )}

      {/* 项目删除对联：Confirm 保留 v1.1 安全边界，明示关联待办置空。 */}
      {pendingDelete && (
        <ConfirmDialog
          title="删除项目"
          confirmText="删除"
          onConfirm={() => {
            void deleteProject(pendingDelete.id);
            setPendingDelete(null);
          }}
          onCancel={() => setPendingDelete(null)}
        >
          确定删除「{pendingDelete.name}」？项目下的待办会保留，但不再关联该项目。
        </ConfirmDialog>
      )}
    </div>
  );
}
