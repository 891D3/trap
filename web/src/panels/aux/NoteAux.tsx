// NoteAux —— 信息记录 v2 辅助区（ui-baseline v3 §5.4c）：
//   最近笔记卡（当前项 #EEF1F6 高亮/项目色点+项目名+时间 meta/新建快捷钮/收起钮）
//   → 弹性占位 → 常用标签卡（TagStats Top5 点击过滤）→ 导出卡（MD/富文本/PDF 置灰）。
import { useEffect, useMemo, useState } from 'react';
import { copyNoteRichText, exportNotes, listNotesV2 } from '../../api/notes';
import { toChineseError } from '../../api/app-settings';
import { IconPanelLeftOpen, IconPlus } from '../../components/icons';
import { useStore } from '../../state/app-store';

/** 相对时间（灵感/最近笔记 meta 用）。 */
function relativeStamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return `今天 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  }
  const dayStart = (x: Date): number => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diffDays = Math.round((dayStart(now) - dayStart(d)) / 86400000);
  if (diffDays === 1) return '昨天';
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  if (diffDays < 7) return week[d.getDay()] ?? '';
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export function NoteAux({
  onAuxCollapse,
}: {
  onAuxCollapse?: () => void;
}): React.JSX.Element {
  const { notesV2, projects, sparks, noteActiveId, addNote, setNoteActiveId } = useStore();
  const [tagFilter, setTagFilter] = useState('');
  const [tags, setTags] = useState<{ tag: string; count: number }[]>([]);
  const [recentFiltered, setRecentFiltered] = useState<Array<{ id: number; title: string; kind: string }> | null>(null);

  // TagStats（Go 端解析正文 #token Top5）。
  useEffect(() => {
    let cancelled = false;
    import('../../api/notes')
      .then(async (mod) => {
        const { NoteService } = await import('../../mygo');
        try {
          const list = await NoteService.tagStats(5);
          if (!cancelled) setTags(list.map((item) => ({ tag: item.tag, count: item.count })));
        } catch {
          // 统计失败不阻塞（空标签卡）。
        }
      })
      .catch(() => undefined);
    return () => { cancelled = true; };
  }, [notesV2.length, sparks.length]);

  // 标签点击 → 过滤最近笔记（ListNotesV2 tag 参数）。
  const applyTag = async (tag: string): Promise<void> => {
    const next = tagFilter === tag ? '' : tag;
    setTagFilter(next);
    try {
      const rows = (await listNotesV2(null, next)).filter((n) => n.kind !== 'spark');
      setRecentFiltered(rows.map((n) => ({ id: n.id, title: n.title, kind: n.kind })));
    } catch (error: unknown) {
      console.error(toChineseError(error, '标签过滤失败'));
    }
  };

  const exportCurrent = async (): Promise<void> => {
    if (noteActiveId !== null) await exportNotes([noteActiveId]);
  };
  const copyRich = async (): Promise<void> => {
    if (noteActiveId !== null) await copyNoteRichText(noteActiveId);
  };

  // 最近笔记 = notesV2 前 5 条（tagFilter 有值时展示过滤结果）。
  const rows = useMemo(() => {
    const source = recentFiltered ?? notesV2;
    return source.slice(0, 5);
  }, [recentFiltered, notesV2]);

  return (
    <>
      {/* 最近笔记卡 */}
      <div className="aux-card aux-note-recent">
        <div className="aux-recent-head">
          <span className="aux-title-sm">最近笔记</span>
          <button
            className="aux-mini-btn"
            title="新建笔记"
            aria-label="新建笔记"
            onClick={() => void (async () => {
              try {
                const created = await addNote('未命名记录');
                setNoteActiveId(created.id);
              } catch (error: unknown) {
                console.error(toChineseError(error, '新建记录失败'));
              }
            })()}
          >
            <IconPlus size={16} />
          </button>
          <button
            className="aux-mini-btn"
            title="收起面板"
            aria-label="收起最近笔记面板"
            onClick={onAuxCollapse}
          >
            <IconPanelLeftOpen size={16} />
          </button>
        </div>
        {rows.map((n) => (
          <button
            key={n.id}
            className={`aux-note-row${noteActiveId === n.id ? ' is-active' : ''}`}
            onClick={() => setNoteActiveId(n.id)}
          >
            <span className="aux-note-title">{n.title || '未命名记录'}</span>
            <span className="aux-note-meta">
              {(() => {
                const pId = notesV2.find((x) => x.id === n.id)?.projectId ?? null;
                const proj = pId != null ? projects.find((item) => item.id === pId) : null;
                const time = relativeStamp(notesV2.find((x) => x.id === n.id)?.updatedAt ?? '');
                return proj
                  ? <><span className="aux-note-proj">#{proj.name}</span><span className="aux-note-dot">·</span><span className="aux-note-time">{time}</span></>
                  : <span className="aux-note-time">{time}</span>;
              })()}
            </span>
          </button>
        ))}
        {rows.length === 0 && <div className="aux-empty">{recentFiltered ? '没有匹配标签的笔记' : '还没有笔记'}</div>}
      </div>

      {/* 弹性占位 */}
      <div className="aux-flex-spacer" />

      {/* 常用标签卡 */}
      <div className="aux-card">
        <div className="aux-title-row"><span className="aux-title-sm">常用标签</span></div>
        <div className="aux-tag-cloud">
          {tags.map((t) => (
            <button
              key={t.tag}
              className={`aux-tag${tagFilter === t.tag ? ' is-active' : ''}`}
              onClick={() => void applyTag(t.tag)}
            >
              # {t.tag}
            </button>
          ))}
          {tags.length === 0 && <div className="aux-empty">正文里用 #标签 标记，会聚在这里</div>}
        </div>
      </div>

      {/* 导出卡 */}
      <div className="aux-card aux-export">
        <div className="aux-title-row"><span className="aux-title-sm">导出</span></div>
        <button className="aux-export-row" disabled={noteActiveId === null} onClick={() => void exportCurrent()} title="导出当前笔记为 Markdown 文件">
          <span className="aux-export-icon is-md">M↓</span>
          <span className="aux-export-name">Markdown 文件</span>
          <span className="aux-export-ext">.md</span>
        </button>
        <button className="aux-export-row is-disabled" title="后续版本开放" disabled>
          <span className="aux-export-icon is-pdf">PDF</span>
          <span className="aux-export-name">PDF 文档</span>
        </button>
        <button
          className="aux-export-row"
          disabled={noteActiveId === null}
          onClick={() => void copyRich()}
          title="复制为富文本（⌘⇧C）"
        >
          <span className="aux-export-icon is-rich">⧉</span>
          <span className="aux-export-name">复制为富文本</span>
          <span className="aux-export-ext">⌘⇧C</span>
        </button>
      </div>
    </>
  );
}
