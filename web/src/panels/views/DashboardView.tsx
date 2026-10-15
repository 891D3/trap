// DashboardView —— v3 数据看板视图（ui-baseline §5.1，PRD v2.0 §4.2）：
//   KPI 行（4 等宽白卡）→ 左「待办事项卡」（380px）+ 右「今日信息卡」（flex）。
// 数据全部来自真实服务：TodoService.Stats 聚合 + store 的 todos 列表。
import { useMemo, useState } from 'react';
import { IconNote, IconPlus, IconTrash } from '../../components/icons';
import { ConfirmDialog } from '../../components/dialogs';
import { useStore } from '../../state/app-store';
import type { Todo } from '../../state/app-store';

/** 分类徽章文案与样式（个人/工作/学习；空 = 未分类不显示徽章）。 */
const CATEGORY_BADGE: Record<string, string> = {
  personal: '个人',
  work: '工作',
  study: '学习',
};

/** 迷你趋势柱：M0 静态占位（PRD v2.0 拟定决策 5，真实序列 M2 补）。 */
const MINI_BARS = [10, 14, 18, 12, 16, 22, 14];

function MiniBars(): React.JSX.Element {
  return (
    <div className="kpi-mini" aria-hidden="true">
      {MINI_BARS.map((h, i) => (
        <span key={i} style={{ height: h }} className={i === MINI_BARS.length - 2 ? 'is-peak' : ''} />
      ))}
    </div>
  );
}

/** 今日信息行的时间列：RFC3339 → HH:mm（本地时区）。 */
function timeLabel(at: string | null | undefined): string {
  if (!at) return '—';
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return '—';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  return `${hh}:${mm}`;
}

export function DashboardView(): React.JSX.Element {
  const { stats, todos, projects, addTodo, setTodoDone, deleteTodo, setView } = useStore();

  // 待办卡：默认显示前 8 条未完成（设计稿示意 8 条），已完成不进列表。
  const openTodos = useMemo(() => todos.filter((t) => !t.done).slice(0, 8), [todos]);
  const undoneCount = useMemo(() => todos.filter((t) => !t.done).length, [todos]);

  // 快速添加（回车快速创建，与设计稿占位一致）。
  const [draft, setDraft] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Todo | null>(null);

  const submitDraft = async (): Promise<void> => {
    const title = draft.trim();
    if (!title) return;
    setDraft('');
    try {
      await addTodo(title);
    } catch {
      // store 已有中文错误提示（todosError），这里静默恢复输入内容。
      setDraft(title);
    }
  };

  const todayTodo = stats?.todayTodoCount ?? 0;
  const diff = (stats?.todayTodoCount ?? 0) - (stats?.yesterdayTodoCount ?? 0);

  return (
    <div className="dashboard">
      {/* ---- KPI 行 ---- */}
      <div className="kpi-row">
        <div className="kpi-card">
          <div className="kpi-title">今日待办</div>
          <div className="kpi-num-row">
            <span className="kpi-num">{todayTodo}</span>
            <MiniBars />
          </div>
          <div className="kpi-trend">
            {diff === 0 ? (
              <span className="badge badge-flat">持平</span>
            ) : diff > 0 ? (
              <span className="badge badge-up">↑ {diff} 较昨日</span>
            ) : (
              <span className="badge badge-down">↓ {-diff} 较昨日</span>
            )}
          </div>
        </div>

        <div className="kpi-card">
          <div className="kpi-title">进行中项目</div>
          <div className="kpi-num-row">
            <span className="kpi-num">{stats?.activeProjects ?? 0}</span>
            <MiniBars />
          </div>
          <div className="kpi-trend">
            <span className="badge badge-flat">进行中</span>
          </div>
        </div>

        <div className="kpi-card">
          <div className="kpi-title">灵感记录</div>
          <div className="kpi-num-row">
            <span className="kpi-num">{stats?.noteTotal ?? 0}</span>
            <MiniBars />
          </div>
          <div className="kpi-trend">
            <span className="badge badge-up-neutral">↑ {stats?.noteWeekNew ?? 0} 本周</span>
          </div>
        </div>

        <div className="kpi-card">
          <div className="kpi-title">本周专注</div>
          <div className="kpi-num-row">
            <span className="kpi-num">0h</span>
            <MiniBars />
          </div>
          <div className="kpi-trend">
            <span className="badge badge-flat">数据接入后展示（M2）</span>
          </div>
        </div>
      </div>

      {/* ---- 待办 + 今日信息 ---- */}
      <div className="dashboard-row">
        {/* 待办事项卡 */}
        <section className="dash-card dash-todo" aria-label="待办事项">
          <header className="dash-card-head">
            <h2 className="dash-card-title">待办事项</h2>
            <span className="dash-count-badge">{undoneCount} / {todos.length}</span>
          </header>

          <div className="todo-add-row dash-quick-add">
            <IconPlus size={14} />
            <input
              className="dash-quick-input"
              placeholder="添加待办，回车快速创建"
              aria-label="快速添加待办"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  void submitDraft();
                }
              }}
            />
          </div>

          <div className="filter-tabs">
            <span className="filter-tab is-active">全部</span>
            <button className="filter-tab" onClick={() => setView('todo')}>进行中</button>
            <button className="filter-tab" onClick={() => setView('todo')}>已完成</button>
          </div>

          <ul className="dash-todo-list">
            {openTodos.map((todo) => (
              <li key={todo.id} className="dash-todo-item">
                <button
                  className="todo-check"
                  role="checkbox"
                  aria-checked="false"
                  aria-label={`完成 ${todo.title}`}
                  onClick={() => void setTodoDone(todo.id, true)}
                />
                <span className="dash-todo-text">{todo.title}</span>
                {todo.category && (
                  <span className={`note-badge cat-${todo.category}`}>{CATEGORY_BADGE[todo.category] ?? todo.category}</span>
                )}
                <button
                  className="btn-icon"
                  aria-label={`删除 ${todo.title}`}
                  title="删除待办"
                  onClick={() => setPendingDelete(todo)}
                >
                  <IconTrash size={14} />
                </button>
              </li>
            ))}
            {openTodos.length === 0 && (
              <li className="dash-empty">今天没有安排，去「待办事项」添加吧</li>
            )}
          </ul>

          <footer className="dash-todo-foot">
            <button className="dash-foot-btn" onClick={() => setView('todo')}>
              查看全部 {todos.length} 项 →
            </button>
          </footer>
        </section>

        {/* 今日信息卡 */}
        <section className="dash-card dash-today" aria-label="今日信息">
          <header className="dash-card-head">
            <h2 className="dash-card-title">今日信息</h2>
            <span className="dash-today-date">{todayLabel()}</span>
          </header>

          <ul className="today-list">
            {(stats?.todayAgenda ?? []).map((item) => (
              <li key={`${item.kind}-${item.refId}`} className="today-item">
                <span className="today-time">{timeLabel(item.at)}</span>
                <span className={`today-dot ${dotClass(item)}`} />
                <span className="today-title">{item.title}</span>
                <span className={`note-badge ${badgeClass(item)}`}>{badgeText(item)}</span>
              </li>
            ))}
            {(stats?.todayAgenda?.length ?? 0) === 0 && (
              <li className="dash-empty">今天暂时没有信息</li>
            )}
          </ul>

          <footer className="today-foot">
            <span>项目进行中 {projects.filter((p) => p.openTodos > 0).length} 个</span>
          </footer>
        </section>
      </div>

      {pendingDelete && (
        <ConfirmDialog
          title="删除待办"
          confirmText="删除"
          onConfirm={() => {
            void deleteTodo(pendingDelete.id);
            setPendingDelete(null);
          }}
          onCancel={() => setPendingDelete(null)}
        >
          确定删除「{pendingDelete.title}」？
        </ConfirmDialog>
      )}
    </div>
  );
}

/** 顶部日期「10月14日 · 星期三」。 */
function todayLabel(): string {
  const now = new Date();
  const week = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
  return `${now.getMonth() + 1}月${now.getDate()}日 · ${week[now.getDay()]}`;
}

type AgendaItem = NonNullable<ReturnType<typeof useStore>['stats']>['todayAgenda'][number];

/** 时间轴状态点：todo-done 绿（下跌绿浅底覆盖已完成的正向语义）、todo 未完成蓝（进行中）、note 灰蓝。 */
function dotClass(item: AgendaItem): string {
  if (item.kind === 'note') return 'is-note';
  return item.status === 'done' ? 'is-done' : 'is-open';
}

function badgeClass(item: AgendaItem): string {
  if (item.kind === 'note') return 'badge-note';
  return item.status === 'done' ? 'badge-done' : 'badge-open';
}

function badgeText(item: AgendaItem): string {
  if (item.kind === 'note') return '已记录';
  return item.status === 'done' ? '已完成' : '待开始';
}
