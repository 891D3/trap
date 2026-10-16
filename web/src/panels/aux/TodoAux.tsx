// TodoAux —— 待办视图的辅助区四卡（ui-baseline §5.3 辅助区映射）：
//   月历（点击日期 → 列表过滤该日，可取消）/ 按属性看（分类计数筛选）/
//   按项目查看（项目筛选，点击联动侧栏过滤态）/ 即将到来（未来 3 个有日期未完成）。
// 过滤态在 store（auxDateFilter/auxCategoryFilter/projectFilter），与列表卡联动。
import { useMemo, useState } from 'react';
import { dueToRFC3339 } from '../../api/todo-parse';
import { useStore, type Todo } from '../../state/app-store';

const WEEK_HEADS = ['一', '二', '三', '四', '五', '六', '日'];
const WEEKDAY_LABEL = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
const CATEGORY_LABEL: Record<string, string> = {
  personal: '个人',
  work: '工作',
  study: '学习',
};
const CATEGORY_KEYS = ['personal', 'work', 'study'] as const;

function dayKey(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${m}-${dd}`;
}

export function TodoAux(): React.JSX.Element {
  const { todos, projects, auxDateFilter, setAuxDateFilter, auxCategoryFilter, setAuxCategoryFilter, projectFilter, openProject } = useStore();

  const undone = useMemo(() => todos.filter((t) => !t.done), [todos]);

  // ---- 月历 ----
  const [monthOffset, setMonthOffset] = useState(0);
  const base = useMemo(() => {
    const d = new Date();
    return new Date(d.getFullYear(), d.getMonth() + monthOffset, 1);
  }, [monthOffset]);

  const weeks = useMemo(() => {
    const first = new Date(base.getFullYear(), base.getMonth(), 1);
    const lead = (first.getDay() + 6) % 7; // 周一开头
    const cells: Array<{ key: string; day: number; inMonth: boolean; date: Date }> = [];
    const start = new Date(first);
    start.setDate(first.getDate() - lead);
    for (let i = 0; i < 42; i++) {
      const d = new Date(start);
      d.setDate(start.getDate() + i);
      cells.push({ key: dayKey(d), day: d.getDate(), inMonth: d.getMonth() === base.getMonth(), date: d });
    }
    const grid: typeof cells[] = [];
    for (let w = 0; w < 6; w++) grid.push(cells.slice(w * 7, w * 7 + 7));
    return grid;
  }, [base]);

  const eventsByDay = useMemo(() => {
    const map = new Map<string, Todo[]>();
    for (const t of undone) {
      if (!t.dueDate) continue;
      const key = dayKey(new Date(t.dueDate));
      const list = map.get(key) ?? [];
      list.push(t);
      map.set(key, list);
    }
    return map;
  }, [undone]);

  const todayKey = dayKey(new Date());

  // ---- 按属性看 ----
  const categoryCount = (key: string): number => undone.filter((t) => t.category === key).length;

  // ---- 即将到来 ----
  const upcoming = useMemo(() => {
    return undone
      .filter((t) => t.dueDate && new Date(t.dueDate).getTime() > Date.now())
      .sort((a, b) => new Date(a.dueDate ?? '').getTime() - new Date(b.dueDate ?? '').getTime())
      .slice(0, 3);
  }, [undone]);

  return (
    <>
      {/* 月历卡 */}
      <div className="aux-card aux-month">
        <div className="aux-month-head">
          <span className="aux-month-title">{base.getFullYear()}年{base.getMonth() + 1}月</span>
          <div className="aux-month-btns">
            <button className="aux-month-btn" aria-label="上月" onClick={() => setMonthOffset((o) => o - 1)}>‹</button>
            <button className="aux-month-today" onClick={() => { setMonthOffset(0); setAuxDateFilter(todayKey); }}>今天</button>
            <button className="aux-month-btn" aria-label="下月" onClick={() => setMonthOffset((o) => o + 1)}>›</button>
          </div>
        </div>
        <div className="aux-month-grid">
          {WEEK_HEADS.map((h) => <span key={h} className="aux-month-weekhead">{h}</span>)}
          {weeks.flat().map((cell) => {
            const events = eventsByDay.get(cell.key);
            const isToday = cell.key === todayKey;
            const isSelected = auxDateFilter === cell.key;
            return (
              <button
                key={cell.key}
                className={`aux-month-day${cell.inMonth ? '' : ' is-out'}${isToday ? ' is-today' : ''}${isSelected ? ' is-selected' : ''}`}
                aria-label={`${cell.key}${events ? ` ${events.length} 项` : ''}`}
                onClick={() => setAuxDateFilter(isSelected ? null : cell.key)}
              >
                <span className="aux-month-num">{cell.day}</span>
                {events && events.length > 0 && <span className="aux-month-dot" />}
              </button>
            );
          })}
        </div>
      </div>

      {/* 按属性看 */}
      <div className="aux-card">
        <h2 className="aux-title-sm">按属性看</h2>
        <div className="aux-filter-list">
          {CATEGORY_KEYS.map((key) => (
            <button
              key={key}
              className={`aux-filter-row${auxCategoryFilter === key ? ' is-active' : ''}`}
              onClick={() => setAuxCategoryFilter(auxCategoryFilter === key ? '' : key)}
            >
              <span className={`category-ring ring-${key}`} />
              <span className="aux-filter-name">{CATEGORY_LABEL[key]}</span>
              <span className="aux-filter-count">{categoryCount(key)}</span>
            </button>
          ))}
        </div>
      </div>

      {/* 按项目查看 */}
      <div className="aux-card">
        <h2 className="aux-title-sm">按项目查看</h2>
        <div className="aux-filter-list">
          {projects.map((p) => (
            <button
              key={p.id}
              className={`aux-filter-row${projectFilter === p.id ? ' is-active' : ''}`}
              onClick={() => openProject(p.id)}
              // 与侧栏一致：openProject 会登记顶部标签页并进入过滤视图。
              data-project={p.id}
            >
              <span className={`side-dot proj-${p.color}`} />
              <span className="aux-filter-name">{p.name}</span>
              <span className="aux-filter-count">{p.openTodos}</span>
            </button>
          ))}
          {projects.length === 0 && <div className="aux-empty">还没有项目</div>}
        </div>
      </div>

      {/* 即将到来 */}
      <div className="aux-card">
        <h2 className="aux-title-sm">即将到来</h2>
        <div className="aux-upcoming-list">
          {upcoming.map((t) => {
            const d = new Date(t.dueDate ?? '');
            return (
              <div key={t.id} className="aux-upcoming-row">
                <span className="aux-upcoming-date">
                  <span className="aux-upcoming-day">{d.getDate()}</span>
                  <span className="aux-upcoming-week">{WEEKDAY_LABEL[d.getDay()]}</span>
                </span>
                <span className="aux-upcoming-text">
                  <span className="aux-upcoming-title">{t.title}</span>
                  <span className="aux-upcoming-time">
                    {dueToRFC3339(d) ? `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}` : ''}
                  </span>
                </span>
              </div>
            );
          })}
          {upcoming.length === 0 && <div className="aux-empty">没有未来安排</div>}
        </div>
      </div>
    </>
  );
}
