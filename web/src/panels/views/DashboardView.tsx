// DashboardView —— v3 数据看板视图（ui-baseline §5.1，PRD v2.0 §4.2）：
//   KPI 行（4 等宽白卡）→ 左「待办事项卡」（380px）+ 右「今日信息卡」（flex）。
// 数据全部来自真实服务：TodoService.Stats 聚合 + store 的 todos 列表。
import { useMemo, useState } from 'react';
import { IconNote, IconPlus, IconRefresh, IconTrash } from '../../components/icons';
import { ConfirmDialog } from '../../components/dialogs';
import { useStore } from '../../state/app-store';
import type { Todo } from '../../state/app-store';
import { fetchNews, NEWS_CHANNELS, type NewsChannelKey } from '../../api/news';
import type { NewsItem } from '../../mygo';
import { subscribeNewsChanged } from '../../realtime/news-events';
import { useEffect } from 'react';

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

  // ---- 热点资讯标签页（今日信息卡内）----
  // tab = 'today'（今日信息）/ 渠道 key（weibo…）；多渠道在 NEWS_CHANNELS 扩展。
  const [infoTab, setInfoTab] = useState<'today' | NewsChannelKey>('today');
  const [newsItems, setNewsItems] = useState<Record<string, NewsItem[]>>({});
  const [newsLoading, setNewsLoading] = useState(false);
  const [newsError, setNewsError] = useState<string | null>(null);

  const loadNews = async (platform: NewsChannelKey, force = false): Promise<void> => {
    setNewsLoading(true);
    setNewsError(null);
    try {
      const items = await fetchNews(platform, force);
      setNewsItems((cur) => ({ ...cur, [platform]: items }));
    } catch (error: unknown) {
      setNewsError(error instanceof Error ? error.message : '资讯加载失败');
    } finally {
      setNewsLoading(false);
    }
  };

  // 切到资讯 tab 时懒加载（无缓存才拉；有缓存直接展示）。
  useEffect(() => {
    if (infoTab !== 'today' && !newsItems[infoTab]) void loadNews(infoTab);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [infoTab]);

  // 后台定时刷新广播：刷新当前渠道列表（静默，不转 loading）。
  useEffect(
    () =>
      subscribeNewsChanged((payload) => {
        if (payload.platform === infoTab) {
          void fetchNews(payload.platform as NewsChannelKey).then((items) =>
            setNewsItems((cur) => ({ ...cur, [payload.platform]: items })),
          );
        }
      }),
    [infoTab],
  );

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

          {/* bug #11：去掉「进行中/已完成」假 Tab（点击只是跳转、无筛选作用，用户已裁定移除）；
              行项末尾保留「查看全部」跳转。 */}

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

        {/* 今日信息 + 热点资讯卡（标签页：今日信息/微博…，右侧手动刷新） */}
        <section className="dash-card dash-today" aria-label="今日信息">
          <header className="dash-card-head">
            <div className="dash-info-tabs">
              <button
                className={`info-tab is-title${infoTab === 'today' ? ' is-active' : ''}`}
                onClick={() => setInfoTab('today')}
              >
                今日信息
              </button>
              {NEWS_CHANNELS.map((ch) => (
                <button
                  key={ch.key}
                  className={`info-tab${infoTab === ch.key ? ' is-active' : ''}`}
                  onClick={() => setInfoTab(ch.key)}
                >
                  {ch.label}
                </button>
              ))}
            </div>
            <div className="dash-head-right">
              {infoTab !== 'today' && (
                <button
                  className="btn-icon"
                  aria-label="手动刷新资讯"
                  title="刷新"
                  disabled={newsLoading}
                  onClick={() => void loadNews(infoTab, true)}
                >
                  <IconRefresh size={14} />
                </button>
              )}
              <span className="dash-today-date">{todayLabel()}</span>
            </div>
          </header>

          {infoTab === 'today' ? (
            <>
              {/* 今日信息 tab 仅作标题页：待办内容与侧栏「待办事项」重复，已取消展示。
                  这里只保留底部项目进行中统计。 */}
              <div className="today-empty" />

              <footer className="today-foot">
                <span>项目进行中 {projects.filter((p) => p.openTodos > 0).length} 个</span>
              </footer>
            </>
          ) : (
            <ul className="news-list">
              {newsLoading && !(newsItems[infoTab]?.length) && <li className="dash-empty">正在获取资讯…</li>}
              {!newsLoading && newsError && (
                <li className="dash-empty">{newsError}</li>
              )}
              {(newsItems[infoTab] ?? []).map((item, i) => (
                <li key={`${item.url}-${i}`} className="news-item">
                  <span className="news-rank">{i + 1}</span>
                  <a href={item.url} target="_blank" rel="noreferrer" className="news-title">
                    {item.title}
                  </a>
                </li>
              ))}
              {!newsLoading && !newsError && (newsItems[infoTab]?.length ?? 0) === 0 && (
                <li className="dash-empty">暂无资讯</li>
              )}
            </ul>
          )}
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


