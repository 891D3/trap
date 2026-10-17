// TodayFocus —— 辅助区「今日聚焦」（ui-baseline §5.2）：
//   今日日期卡（大日号 + 周日历条）→ 快速灵感（Enter 保存）→ 最近记录（4 条，
//   「整理为 Markdown」导出未归档记录）→ 项目进度（进度条按项目色）。
// 数据源：store 的 notes/projects（事件驱动）。
import { useMemo, useState } from 'react';
import { IconNote } from '../../components/icons';
import { exportNotes } from '../../api/notes';
import { toChineseError } from '../../api/app-settings';
import { useStore } from '../../state/app-store';
import { lunarOf } from '../../api/lunar';

/** 周日历条：本周一~周日 7 胶囊，今日主色底白字。 */
function weekStrip(): { label: string; isToday: boolean }[] {
  const now = new Date();
  const dayIndex = (now.getDay() + 6) % 7; // 周一=0
  const monday = new Date(now);
  monday.setDate(now.getDate() - dayIndex);
  const labels = ['一', '二', '三', '四', '五', '六', '日'];
  return labels.map((label, i) => {
    const d = new Date(monday);
    d.setDate(monday.getDate() + i);
    return { label, isToday: d.toDateString() === now.toDateString() };
  });
}

/** 相对时间（10 分钟前 / 昨天 / 3 天前；一周以上显示日期）。 */
function relativeTime(iso: string): string {
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return '';
  const diff = Date.now() - t;
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days === 1) return '昨天';
  if (days < 7) return `${days} 天前`;
  const d = new Date(t);
  return `${d.getMonth() + 1}月${d.getDate()}日`;
}

function weekday(): string {
  const week = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'];
  return week[new Date().getDay()] ?? '';
}

export function TodayFocus(): React.JSX.Element {
  const { notes, projects, addNote, saveNote, exportNote: exportNoteUnused, setView } = useStore();
  void exportNoteUnused; // 导出能力由「整理为 Markdown」按钮承担（走 exportNotes 批量）

  // 农历（模块级换算即可，日变化以天为单位；date 变化时组件随事件重渲染，
  // 跨天未重渲染时显示昨天的农历可忽略——下次挂载会刷新）。
  const lunar = useMemo(() => lunarOf(), []);

  // 最近记录：未归档按更新时间倒序取 4 条（store 列表为创建序，本地排序）。
  const recent = useMemo(
    () =>
      [...notes]
        .filter((n) => !n.archived)
        .sort((a, b) => (a.updatedAt < b.updatedAt ? 1 : -1))
        .slice(0, 4),
    [notes],
  );

  // 快速灵感：Enter 保存 → 创建记录并把内容写入正文（标题取首行）。
  const [spark, setSpark] = useState('');
  const [saving, setSaving] = useState(false);

  const saveSpark = async (): Promise<void> => {
    const text = spark.trim();
    if (!text || saving) return;
    setSaving(true);
    try {
      const note = await addNote('灵感');
      const firstLine = text.split('\n')[0] ?? '';
      const title = firstLine.slice(0, 20) || '灵感';
      await saveNote(note.id, title, text);
      setSpark('');
    } catch (error: unknown) {
      console.error(toChineseError(error, '灵感保存失败，请稍后重试'));
    } finally {
      setSaving(false);
    }
  };

  const organizeMarkdown = async (): Promise<void> => {
    const ids = recent.map((n) => n.id);
    if (ids.length === 0) return;
    try {
      await exportNotes(ids);
    } catch (error: unknown) {
      console.error(toChineseError(error, 'Markdown 整理失败，请稍后重试'));
    }
  };

  return (
    <>
      {/* 今日日期卡 */}
      <div className="aux-card aux-focus-head">
        <div className="aux-focus-title-row">
          <h2 className="aux-title">今日聚焦</h2>
        </div>
        <div className="aux-date-row">
          <span className="aux-day">{new Date().getDate()}</span>
          <span className="aux-date-text">
            <span className="aux-month">{new Date().getMonth() + 1}月 · {weekday()}</span>
            {/* 复测反馈：日期区显示农历（农历X月初X，ICU 内置中历，零依赖） */}
            {lunar && (
              <span className="aux-lunar">农历{lunar.month}{lunar.day}</span>
            )}
          </span>
        </div>
        <div className="aux-week">
          {weekStrip().map((w) => (
            <span key={w.label} className={`aux-week-item${w.isToday ? ' is-today' : ''}`}>{w.label}</span>
          ))}
        </div>
      </div>

      {/* 快速灵感 */}
      <div className="aux-card">
        <div className="aux-title-row">
          <h2 className="aux-title-sm">快速灵感</h2>
          <span className="aux-hint">Enter 保存</span>
        </div>
        <textarea
          className="aux-spark-input"
          placeholder="记下此刻的想法…"
          rows={3}
          value={spark}
          onChange={(e) => setSpark(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              void saveSpark();
            }
          }}
        />
        <button
          className="btn is-primary aux-spark-save"
          disabled={!spark.trim() || saving}
          onClick={() => void saveSpark()}
        >
          保存记录
        </button>
      </div>

      {/* 最近记录 */}
      <div className="aux-card aux-recent">
        <div className="aux-title-row">
          <h2 className="aux-title-sm">最近记录</h2>
          <button className="aux-link" onClick={() => setView('note')}>
            查看全部
          </button>
        </div>
        <ul className="aux-recent-list">
          {recent.map((note) => (
            <li key={note.id} className="aux-recent-item">
              <span className="aux-recent-icon"><IconNote size={14} /></span>
              <span className="aux-recent-text">
                <span className="aux-recent-title">{note.title || '未命名记录'}</span>
                <span className="aux-recent-time">{relativeTime(note.updatedAt)}</span>
              </span>
            </li>
          ))}
          {recent.length === 0 && <li className="aux-empty">还没有记录，先随手记一条</li>}
        </ul>
        <button
          className="aux-md-btn"
          disabled={recent.length === 0}
          onClick={() => void organizeMarkdown()}
        >
          整理为 Markdown 本地数据 ↓
        </button>
      </div>

      {/* 项目进度 */}
      <div className="aux-card">
        <h2 className="aux-title-sm">项目进度</h2>
        <div className="aux-progress-list">
          {projects.map((p) => {
            const percent = p.totalTodos === 0 ? 0 : Math.round(((p.totalTodos - p.openTodos) / p.totalTodos) * 100);
            return (
              <div key={p.id} className="aux-progress-row">
                <div className="aux-progress-head">
                  <span className="aux-progress-name">{p.name}</span>
                  <span className="aux-progress-percent">{percent}%</span>
                </div>
                <div className="aux-progress-track">
                  <div className={`aux-progress-fill proj-${p.color}`} style={{ width: `${percent}%` }} />
                </div>
              </div>
            );
          })}
          {projects.length === 0 && <div className="aux-empty">还没有项目</div>}
        </div>
      </div>
    </>
  );
}
