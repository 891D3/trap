// TodoView —— v3 待办视图（ui-baseline §5.3，PRD v2.0 §4.3）：
//   快速创建卡（多行输入 + # 项目联想 + 时间短语 chip + 分类单选 + ⌘/Ctrl+Enter）
//   → 待办列表卡（全部/今天/已完成 Tab + 日期分组 + 行内编辑态）。
//   辅助区（月历/按属性/按项目/即将到来）在 Auxiliary 侧联动本视图的过滤态。
import { useEffect, useMemo, useRef, useState } from 'react';
import { toChineseError } from '../../api/app-settings';
import { dueToRFC3339, extractDue, extractProjectToken, projectQuery } from '../../api/todo-parse';
import { ConfirmDialog } from '../../components/dialogs';
import { IconCalendar, IconPlus, IconTrash, IconX } from '../../components/icons';
import { useStore, type Todo } from '../../state/app-store';

const CATEGORY_LABEL: Record<string, string> = {
  personal: '个人',
  work: '工作',
  study: '学习',
};

const CATEGORY_KEYS = ['personal', 'work', 'study'] as const;
type Tab = 'all' | 'today' | 'done';

/**
 * bug #8：快速创建草稿的模块级缓存（React 本地 state 会随视图切换卸载丢失）。
 * height 记录用户手动拖拽后的内联高度（restore 后再应用回 textarea）。
 */
const quickDraftCache: { text: string; height: string | null } = { text: '', height: null };

/** 本地 yyyy-mm-dd（日期分组与月历的日期键）。 */
function dayKey(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${dd}`;
}

function shiftDay(d: Date, n: number): Date {
  const x = new Date(d);
  x.setDate(x.getDate() + n);
  return x;
}

/** 未完成条目排序：已安排升序在前，未安排靠后。 */
function sortAsc(list: Todo[]): void {
  list.sort((a, b) => {
    const ad = a.dueDate ? new Date(a.dueDate).getTime() : Number.MAX_SAFE_INTEGER;
    const bd = b.dueDate ? new Date(b.dueDate).getTime() : Number.MAX_SAFE_INTEGER;
    if (ad !== bd) return ad - bd;
    return b.id - a.id;
  });
}

/** 条目右侧时间 HH:mm。 */
function whenLabel(todo: Todo): string {
  if (!todo.dueDate) return '';
  const d = new Date(todo.dueDate);
  if (Number.isNaN(d.getTime())) return '';
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

export function TodoView() {
  const {
    todos,
    todosLoading,
    todosError,
    addTodoFull,
    editTodo,
    setTodoDone,
    deleteTodo,
    projects,
    projectFilter,
    clearProjectFilter,
    auxDateFilter,
    setAuxDateFilter,
    auxCategoryFilter,
    setAuxCategoryFilter,
  } = useStore();

  // ---- 快速创建 ----
  // bug #8：草稿与输入框高度提升到模块级缓存，跨视图切换（组件卸载）保留。
  const draftInit = quickDraftCache.text;
  const [draft, setDraft] = useState(draftInit);
  const [category, setCategory] = useState<string>('');  // 默认不选（个人/工作/学习空心圆点）
  const [dueChip, setDueChip] = useState<{ at: Date; label: string } | null>(null);
  const [dueSuppressed, setDueSuppressed] = useState(false); // 手动关闭「识别日期」预览，输入变化时重置
  const [projectPick, setProjectPick] = useState<number | null>(null);
  const [pendingDelete, setPendingDelete] = useState<Todo | null>(null);
  const [saving, setSaving] = useState<'add' | number | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  const [tab, setTab] = useState<Tab>('all');
  const [editingId, setEditingId] = useState<number | null>(null);
  // bug #1：记录光标位置，`#` 检测只在光标处 token 上触发。
  const [caret, setCaret] = useState<number | null>(null);
  const quickRef = useRef<HTMLTextAreaElement>(null);

  // 输入即解析：时间短语 →（未手动设 chip 时生效）；#词命中项目 → 项目。
  const parsed = useMemo(() => {
    const { title: t1, due } = extractDue(draft);
    const { title, projectId } = extractProjectToken(t1, projects);
    return { title, due, projectId };
  }, [draft, projects]);

  // bug #1：`#` 联想只在光标所在 token 触发（而非仅文末）。
  const query = projectQuery(draft, caret ?? undefined);
  const suggestions = useMemo(() => {
    if (query === null) return [];
    const q = query.toLowerCase();
    return projects.filter((p) => q === '' || p.name.toLowerCase().includes(q));
  }, [query, projects]);

  const pickProject = (id: number, name: string): void => {
    // 用当前光标所在 token 精确替换（而不是全局末尾替换），避免多 token 误伤。
    // token 范围 = 光标前最近的 `#` 到其后第一个空白/行尾（与 projectQuery 同语义），
    // 而不是 `#` 到光标——否则光标在 token 后的普通文本上时会把整段吞掉。
    const at = caret ?? draft.length;
    const before = draft.slice(0, at);
    const after = draft.slice(at);
    const hashAt = before.lastIndexOf('#');
    if (hashAt !== -1) {
      const lineEnd = draft.indexOf('\n', hashAt);
      const tokenEndInFull = (() => {
        const rest = draft.slice(hashAt + 1);
        const stop = rest.search(/[\s#]/);
        if (stop === -1) return hashAt + 1 + rest.length;
        return hashAt + 1 + stop;
      })();
      const tokenEnd = lineEnd !== -1 ? Math.min(tokenEndInFull, lineEnd) : tokenEndInFull;
      setProjectPick(id);
      const nextDraft = draft.slice(0, hashAt) + `#${name}` + draft.slice(tokenEnd);
      setDraft(nextDraft);
      quickDraftCache.text = nextDraft;
    } else {
      setProjectPick(id);
    }
  };

  const submit = async (): Promise<void> => {
    const text = draft.trim();
    if (!text || !parsed.title) {
      setLocalError('待办内容不能为空');
      return;
    }
    setLocalError(null);
    setSaving('add');
    try {
      await addTodoFull({
        title: parsed.title,
        category,
        dueDate: dueChip ? dueToRFC3339(dueChip.at) : parsed.due ? dueToRFC3339(parsed.due.at) : undefined,
        projectId: (projectPick ?? parsed.projectId) ?? undefined,
      });
      setDraft('');
      quickDraftCache.text = '';  // bug #8：提交后同步清空草稿缓存
      setDueChip(null);
      setProjectPick(null);
      setDueSuppressed(false);
      setCaret(null);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '待办添加失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const toggleTodo = async (todo: Todo): Promise<void> => {
    setLocalError(null);
    setSaving(todo.id);
    try {
      await setTodoDone(todo.id, !todo.done);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '待办状态保存失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const confirmDelete = async (todo: Todo): Promise<void> => {
    setLocalError(null);
    setSaving(todo.id);
    try {
      await deleteTodo(todo.id);
      setPendingDelete(null);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '待办删除失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  // ---- 列表数据 ----
  const undone = useMemo(() => todos.filter((t) => !t.done), [todos]);
  const dueTodayCount = useMemo(() => {
    const todayKey = dayKey(new Date());
    return todos.filter((t) => !t.done && t.dueDate && dayKey(new Date(t.dueDate)) <= todayKey).length;
  }, [todos]);
  const doneCount = useMemo(() => todos.filter((t) => t.done).length, [todos]);

  const visible = useMemo(() => {
    let list = undone;
    if (auxCategoryFilter) list = list.filter((t) => t.category === auxCategoryFilter);
    if (projectFilter !== null) list = list.filter((t) => t.projectId === projectFilter);
    return list;
  }, [undone, auxCategoryFilter, projectFilter]);

  /** 日期分组：今天（含过期，红点警示）/ 明天 / 以后 / 未安排（PRD 拟定决策 3）。 */
  const groups = useMemo(() => {
    const todayKey = dayKey(new Date());
    const tomorrowKey = dayKey(shiftDay(new Date(), 1));
    const g = { today: [] as Todo[], tomorrow: [] as Todo[], later: [] as Todo[], none: [] as Todo[] };
    for (const t of visible) {
      if (!t.dueDate) {
        g.none.push(t);
        continue;
      }
      const key = dayKey(new Date(t.dueDate));
      if (key <= todayKey) g.today.push(t);
      else if (key === tomorrowKey) g.tomorrow.push(t);
      else g.later.push(t);
    }
    sortAsc(g.today);
    sortAsc(g.tomorrow);
    sortAsc(g.later);
    return g;
  }, [visible]);

  const doneList = useMemo(() => {
    let list = todos.filter((t) => t.done);
    if (auxCategoryFilter) list = list.filter((t) => t.category === auxCategoryFilter);
    if (projectFilter !== null) list = list.filter((t) => t.projectId === projectFilter);
    return [...list].sort((a, b) => ((a.completedAt ?? '') < (b.completedAt ?? '') ? 1 : -1));
  }, [todos, auxCategoryFilter, projectFilter]);

  // 分组头格式（设计稿 15:11）：「今天 · 10月15日 周四 · N 项」。
  const WEEK_CN = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  const groupLabel = (key: string, label: string): string => {
    const base = new Date();
    if (key === 'today') return `今天 · ${base.getMonth() + 1}月${base.getDate()}日 ${WEEK_CN[base.getDay()]}`;
    if (key === 'tomorrow') {
      const t = new Date(base.getTime() + 86400000);
      return `明天 · ${t.getMonth() + 1}月${t.getDate()}日 ${WEEK_CN[t.getDay()]}`;
    }
    return label;
  };

  const groupsForTab = (): Array<{ key: string; label: string; dot: string; items: Todo[] }> => {
    if (tab === 'today') {
      // 单独「今天」Tab：仅今天组（口径 = 过期并入今天，红色组点警示）。
      return groups.today.length > 0
        ? [{ key: 'today', label: '今天（含过期）', dot: 'dot-red', items: groups.today }]
        : [];
    }
    return [
      { key: 'today', label: '今天', dot: 'dot-red', items: groups.today },
      { key: 'tomorrow', label: '明天', dot: 'dot-blue', items: groups.tomorrow },
      { key: 'later', label: '以后', dot: 'dot-slate', items: groups.later },
      { key: 'none', label: '未安排', dot: 'dot-gray', items: groups.none },
    ]
      .map((g) => ({ ...g, label: groupLabel(g.key, g.label) }))
      .filter((g) => g.items.length > 0);
  };

  // ⌘/Ctrl+K：聚焦快速创建输入（v1.1 确认决策保留）。
  useEffect(() => {
    const onKey = (e: KeyboardEvent): void => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        quickRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // bug #8：恢复/保存输入框拖拽高度（height 是纯 DOM 属性，不随 state 持久）。
  useEffect(() => {
    const el = quickRef.current;
    if (!el) return;
    if (quickDraftCache.height) el.style.height = quickDraftCache.height;
    const onUp = (): void => {
      const h = el.style.height;
      if (h) quickDraftCache.height = h;
    };
    el.addEventListener('mouseup', onUp);
    return () => el.removeEventListener('mouseup', onUp);
  }, []);

  const filteredProject = projectFilter !== null ? projects.find((p) => p.id === projectFilter) : null;
  const hasFilterChips = filteredProject != null || auxDateFilter !== null || auxCategoryFilter !== '';
  const visibleError = localError ?? todosError;

  return (
    <div className="todov2">
      {visibleError && <div role="alert" className="error-box">{visibleError}</div>}

      {/* ---- 快速创建卡 ---- */}
      <section className="dash-card quick-create">
        <header className="dash-card-head">
          <h2 className="dash-card-title">快速创建</h2>
          <span className="quick-hint">输入 # 关联项目 · 输入时间自动识别</span>
        </header>

        <div className="quick-input-wrap">
          <textarea
            ref={quickRef}
            className="todo-input quick-input"
            rows={4}
            placeholder="要做什么？⌘/Ctrl+Enter 提交"
            aria-label="快速创建"
            value={draft}
            disabled={todosLoading || saving !== null}
            onChange={(e) => {
              setDraft(e.target.value);
              setCaret(e.target.selectionStart);
              quickDraftCache.text = e.target.value;  // bug #8：草稿实时落缓存
            }}
            onClick={(e) => setCaret(e.currentTarget.selectionStart)}
            onKeyUp={(e) => setCaret(e.currentTarget.selectionStart)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                e.preventDefault();
                void submit();
              }
            }}
          />

          {query !== null && (
            <ProjectPopover
              anchor={quickRef.current}
              text={draft}
              caret={caret ?? draft.length}
              suggestions={suggestions}
              onPick={pickProject}
            />
          )}
        </div>

        {/* 操作行：左侧 = 内容输入同步解析出的日期/项目 chips；右侧 = 分类 + 添加待办 */}
        <footer className="quick-actions">
          <div className="quick-chips quick-live-chips">
            {dueChip && (
              <span className="chip">
                <IconCalendar size={12} />
                {dueChip.label}
                <button aria-label="移除日期" onClick={() => setDueChip(null)}><IconX size={10} /></button>
              </span>
            )}
            {!dueChip && parsed.due && !dueSuppressed && (
              <span className="chip is-preview">
                <IconCalendar size={12} />
                {parsed.due.label}
                <button aria-label="移除识别日期" onClick={() => setDueSuppressed(true)}><IconX size={10} /></button>
              </span>
            )}
            {projectPick !== null && (
              <span className="chip">
                <span className={`side-dot proj-${projects.find((p) => p.id === projectPick)?.color ?? 'slate'}`} />
                #{projects.find((p) => p.id === projectPick)?.name ?? ''}
                <button aria-label="移除项目" onClick={() => setProjectPick(null)}><IconX size={10} /></button>
              </span>
            )}
            {projectPick === null && parsed.projectId !== null && (
              <span className="chip is-preview">
                <span className={`side-dot proj-${projects.find((p) => p.id === parsed.projectId)?.color ?? 'slate'}`} />
                #{projects.find((p) => p.id === parsed.projectId)?.name ?? ''}
              </span>
            )}
          </div>
          <div className="quick-actions-right">
            <div className="category-radios" role="radiogroup" aria-label="分类">
              {CATEGORY_KEYS.map((key) => (
                <button
                  key={key}
                  className={`category-radio${category === key ? ' is-active' : ''}`}
                  role="radio"
                  aria-checked={category === key}
                  onClick={() => setCategory((c) => (c === key ? '' : key))}
                >
                  <span className={`category-ring ring-${key}`} />
                  {CATEGORY_LABEL[key]}
                </button>
              ))}
            </div>
            <button className="btn is-primary" disabled={todosLoading || saving !== null} onClick={() => void submit()}>
              <IconPlus />添加待办
            </button>
          </div>
        </footer>
      </section>

      {/* ---- 待办列表卡 ---- */}
      <section className="dash-card todo-list-card">
      {/* bug #9：筛选 chip 与 Tab 同一行（Tab 右侧），不再换行堆叠。 */}
      <div className="list-tabs-row">
        <div className="filter-tabs list-tabs">
          <button className={`filter-tab${tab === 'all' ? ' is-active' : ''}`} onClick={() => setTab('all')}>
            全部 <span className="tab-count">{visible.length}</span>
          </button>
          <button className={`filter-tab${tab === 'today' ? ' is-active' : ''}`} onClick={() => setTab('today')}>
            今天 <span className="tab-count">{dueTodayCount}</span>
          </button>
          <button className={`filter-tab${tab === 'done' ? ' is-active' : ''}`} onClick={() => setTab('done')}>
            已完成 <span className="tab-count">{doneCount}</span>
          </button>
        </div>

        {(filteredProject != null || auxDateFilter !== null || auxCategoryFilter !== '') && (
          <div className="quick-chips list-filter-chips">
            {filteredProject != null && (
              <span className="chip">
                <span className={`side-dot proj-${filteredProject.color}`} />
                #{filteredProject.name}
                <button aria-label="清除项目过滤" onClick={clearProjectFilter}><IconX size={10} /></button>
              </span>
            )}
            {auxDateFilter !== null && (
              <span className="chip">
                <IconCalendar size={12} />
                {auxDateFilter.slice(5)}
                <button aria-label="清除日期过滤" onClick={() => setAuxDateFilter(null)}><IconX size={10} /></button>
              </span>
            )}
            {auxCategoryFilter !== '' && (
              <span className="chip">
                {CATEGORY_LABEL[auxCategoryFilter] ?? auxCategoryFilter}
                <button aria-label="清除分类过滤" onClick={() => setAuxCategoryFilter('')}><IconX size={10} /></button>
              </span>
            )}
          </div>
        )}
      </div>

        {todosLoading ? (
          <div className="aux-empty">正在读取待办…</div>
        ) : tab === 'done' ? (
          <ul className="todo-group-list">
            {doneList.map((todo) => (
              <TodoRow
                key={todo.id}
                todo={todo}
                editing={editingId === todo.id}
                setEditingId={setEditingId}
                saving={saving}
                setSaving={setSaving}
                setLocalError={setLocalError}
                setPendingDelete={setPendingDelete}
                toggleTodo={toggleTodo}
              />
            ))}
            {doneList.length === 0 && <div className="aux-empty">还没有已完成项</div>}
          </ul>
        ) : (
          <>
            {groupsForTab().map((g) => (
              <div key={g.key} className="todo-group">
                <div className="todo-group-head">
                  <span className={`group-dot ${g.dot}`} />
                  <span className="todo-group-label">
                    {g.label} · {g.items.length} 项
                  </span>
                </div>
                <div className="todo-group-divider" />
                <ul className="todo-group-list">
                  {g.items.map((todo) => (
                    <TodoRow
                      key={todo.id}
                      todo={todo}
                      editing={editingId === todo.id}
                      setEditingId={setEditingId}
                      saving={saving}
                      setSaving={setSaving}
                      setLocalError={setLocalError}
                      setPendingDelete={setPendingDelete}
                      toggleTodo={toggleTodo}
                    />
                  ))}
                </ul>
              </div>
            ))}
            {groupsForTab().length === 0 && <div className="aux-empty">这个筛选组没有待办</div>}
          </>
        )}
      </section>

      {pendingDelete && (
        <ConfirmDialog
          title="删除待办"
          confirmText="删除"
          onConfirm={() => void confirmDelete(pendingDelete)}
          onCancel={() => {
            if (saving === null) setPendingDelete(null);
          }}
        >
          确定删除「{pendingDelete.title}」？该操作不可恢复。
        </ConfirmDialog>
      )}
    </div>
  );
}

/** 单条待办行：阅读态 / 编辑态（编辑态点击标题进入）。 */
function TodoRow(props: {
  todo: Todo;
  editing: boolean;
  setEditingId: (id: number | null) => void;
  saving: 'add' | number | null;
  setSaving: (s: 'add' | number | null) => void;
  setLocalError: (m: string | null) => void;
  setPendingDelete: (t: Todo | null) => void;
  toggleTodo: (todo: Todo) => Promise<void>;
}): React.JSX.Element {
  if (props.editing) {
    return <TodoEditRow {...props} />;
  }
  const { todo, setEditingId, saving, setPendingDelete, toggleTodo } = props;
  const { projects } = useStore();
  const project = todo.projectId !== null ? projects.find((p) => p.id === todo.projectId) : null;

  return (
    <li className={`todo-item${todo.done ? ' is-done' : ''}`}>
      <button
        className="todo-check"
        role="checkbox"
        aria-checked={todo.done}
        aria-label={todo.done ? `恢复 ${todo.title}` : `完成 ${todo.title}`}
        disabled={saving !== null}
        onClick={() => void toggleTodo(todo)}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M4 12.5 9.5 18 20 6.5" />
        </svg>
      </button>
      {/* 点击标题进入行内编辑（复选框/删除不受影响）。 */}
      <button className="todo-text" onClick={() => setEditingId(todo.id)}>
        {todo.title}
      </button>
      {project ? (
        <span className={`todo-project proj-text-${project.color}`}>#{project.name}</span>
      ) : todo.category ? (
        <span className={`todo-category cat-text-${todo.category}`}>{CATEGORY_LABEL[todo.category] ?? ''}</span>
      ) : null}
      <span className="todo-when">{whenLabel(todo)}</span>
      <button
        className="btn-icon"
        aria-label={`删除 ${todo.title}`}
        disabled={saving !== null}
        onClick={() => setPendingDelete(todo)}
      >
        <IconTrash />
      </button>
    </li>
  );
}

/** 行内编辑态：标题输入 + 日期 chip + 项目 chip + 分类单选 + 取消/保存。 */
function TodoEditRow(props: {
  todo: Todo;
  setEditingId: (id: number | null) => void;
  saving: 'add' | number | null;
  setSaving: (s: 'add' | number | null) => void;
  setLocalError: (m: string | null) => void;
  setPendingDelete: (t: Todo | null) => void;
  toggleTodo: (todo: Todo) => Promise<void>;
}): React.JSX.Element {
  const { todo, setEditingId, saving, setSaving, setLocalError } = props;
  const { projects, editTodo } = useStore();
  const [title, setTitle] = useState(todo.title);
  // bug #10：去掉 || 'work' 兑底——存量未分类保持未分类，不再新增脏数据。
  const [category, setCategory] = useState<string>(todo.category);
  const [projectPick, setProjectPick] = useState<number | null>(todo.projectId);
  const editRef = useRef<HTMLTextAreaElement>(null);

  // bug #4：标题改用多行 textarea，按内容自适应高度（初始即撑满）。
  useEffect(() => {
    const el = editRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${el.scrollHeight}px`;
  }, [title]);

  const save = async (): Promise<void> => {
    const trimmed = title.trim();
    if (!trimmed) {
      setLocalError('待办内容不能为空');
      return;
    }
    setSaving(todo.id);
    setLocalError(null);
    try {
      await editTodo(todo.id, {
        title: trimmed,
        category,
        dueDate: todo.dueDate ?? undefined,
        projectId: projectPick ?? undefined,
      });
      setEditingId(null);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '待办保存失败，请稍后重试'));
    } finally {
      setSaving(null);
    }
  };

  const project = projectPick !== null ? projects.find((p) => p.id === projectPick) : null;

  return (
    <li className="todo-item is-editing">
      <div className="todo-edit-top">
        <textarea
          ref={editRef}
          className="todo-edit-input"
          rows={1}
          value={title}
          autoFocus
          disabled={saving !== null}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              void save();
            }
            if (e.key === 'Escape') setEditingId(null);
          }}
        />
      </div>
      {/* bug #3：底部一行 = 左侧日期/项目 chip，右侧分类单选紧挨取消/保存（用户复测反馈：
          单选应靠右与按钮同组，而非跟在 chip 后）。 */}
      <div className="todo-edit-footer">
        <div className="todo-edit-meta">
          <span className="chip">
            <IconCalendar size={12} />
            {whenLabel(todo) || '未安排'}
          </span>
          {project != null && (
            <span className="chip">
              <span className={`side-dot proj-${project.color}`} />
              #{project.name}
              <button aria-label="移除项目" onClick={() => setProjectPick(null)}>
                <IconX size={10} />
              </button>
            </span>
          )}
        </div>
        <div className="category-radios" role="radiogroup" aria-label="分类">
          {CATEGORY_KEYS.map((key) => (
            <button
              key={key}
              className={`category-radio${category === key ? ' is-active' : ''}`}
              role="radio"
              aria-checked={category === key}
              onClick={() => setCategory(key)}
            >
              <span className={`category-ring ring-${key}`} />
              {CATEGORY_LABEL[key]}
            </button>
          ))}
        </div>
        <div className="todo-edit-buttons">
          <button className="btn" disabled={saving !== null} onClick={() => setEditingId(null)}>取消</button>
          <button className="btn is-primary" disabled={saving !== null} onClick={() => void save()}>保存</button>
        </div>
      </div>
    </li>
  );
}

/**
 * 项目联想浮层（bug #2 修复）：用「镜像 div」测量光标在 textarea 内的坐标，
 * 把浮层定位到光标附近，而不是固定在输入框下方。
 * 原理：复制 textarea 的字体/行高/宽度/内边距到隐藏 div，插入光标前文本 +
 * 定位 span，读取 span 偏移即光标坐标。
 */
function ProjectPopover(props: {
  anchor: HTMLTextAreaElement | null;
  text: string;
  caret: number;
  suggestions: Array<{ id: number; name: string; color: string; openTodos: number }>;
  onPick: (id: number, name: string) => void;
}): React.JSX.Element | null {
  const { anchor, text, caret, suggestions, onPick } = props;

  // 每次渲染计算一次光标坐标（输入量小，直接同步测量即可）。
  let left = 0;
  let top = 0;
  if (anchor) {
    const mirror = document.createElement('div');
    const style = window.getComputedStyle(anchor);
    for (const prop of [
      'fontFamily', 'fontSize', 'fontWeight', 'lineHeight', 'letterSpacing',
      'paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft',
      'borderWidth', 'boxSizing', 'whiteSpace', 'wordWrap', 'textTransform',
    ] as const) {
      mirror.style[prop] = style[prop];
    }
    mirror.style.position = 'absolute';
    mirror.style.visibility = 'hidden';
    mirror.style.width = `${anchor.clientWidth}px`;
    mirror.style.whiteSpace = 'pre-wrap';
    mirror.style.wordWrap = 'break-word';
    mirror.textContent = text.slice(0, caret);
    const marker = document.createElement('span');
    marker.textContent = '\u200b'; // 零宽字符锚定光标点
    mirror.appendChild(marker);
    document.body.appendChild(mirror);
    left = marker.offsetLeft;
    top = marker.offsetTop;
    document.body.removeChild(mirror);
  }

  return (
    <div className="project-pop is-caret" style={{ left, top }}>
      <div className="project-pop-title">选择项目</div>
      {suggestions.map((p) => (
        <button key={p.id} className="project-pop-item" onClick={() => onPick(p.id, p.name)}>
          <span className={`side-dot proj-${p.color}`} />
          <span className="project-pop-name">#{p.name}</span>
          <span className="project-pop-count">{p.openTodos} 项待办</span>
        </button>
      ))}
      {suggestions.length === 0 && <div className="aux-empty">无匹配项目</div>}
    </div>
  );
}
