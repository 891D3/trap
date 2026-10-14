import { useCallback, useEffect, useRef, useState } from 'react';
import { toChineseError } from '../../api/app-settings';
import { ConfirmDialog } from '../../components/dialogs';
import { MarkdownEditor } from '../../components/MarkdownEditor';
import { IconArchive, IconCollapse, IconExpand, IconExport, IconNote, IconPlus, IconTrash } from '../../components/icons';
import { useStore, type Note, type NoteSummary } from '../../state/app-store';

const fmtTime = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });

/** T5 信息记录视图：列表筛选 + 当前记录 Markdown 编辑区。 */
export function NoteView() {
  const {
    notes,
    notesLoading,
    notesError,
    addNote,
    getNote,
    saveNote,
    setNoteArchived,
    deleteNote,
    exportNote,
  } = useStore();
  const [tab, setTab] = useState<'all' | 'archived'>('all');
  const [activeNote, setActiveNote] = useState<Note | null>(null);
  const [titleDraft, setTitleDraft] = useState('');
  const [contentDraft, setContentDraft] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [pendingDelete, setPendingDelete] = useState<NoteSummary | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  // 右侧记录列表的收起/展开状态（会话内保持，默认展开）。
  const [listCollapsed, setListCollapsed] = useState(false);
  // 进入视图后自动打开最近一条记录，使中间默认呈现大编辑器。
  const autoOpenedRef = useRef(false);
  const titleRef = useRef(titleDraft);
  const contentRef = useRef(contentDraft);

  titleRef.current = titleDraft;
  contentRef.current = contentDraft;

  const persistNote = useCallback(async (): Promise<void> => {
    if (!activeNote || !dirty) return;
    setSaveState('saving');
    try {
      const saved = await saveNote(activeNote.id, titleRef.current.trim() || '未命名记录', contentRef.current);
      setActiveNote(saved);
      setTitleDraft(saved.title);
      setContentDraft(saved.content);
      setDirty(false);
      setSaveState('saved');
    } catch (error: unknown) {
      setSaveState('error');
      setLocalError(toChineseError(error, '信息记录保存失败，请稍后重试'));
    }
  }, [activeNote, dirty, saveNote]);

  // 标题或正文停止变化 2 秒后自动保存。
  useEffect(() => {
    if (!activeNote || !dirty) return;
    const timer = window.setTimeout(() => {
      void persistNote();
    }, 2000);
    return () => window.clearTimeout(timer);
  }, [activeNote, titleDraft, contentDraft, dirty, persistNote]);

  const openNote = async (summary: NoteSummary): Promise<void> => {
    if (activeNote?.id === summary.id) return;
    if (dirty) await persistNote();
    setLocalError(null);
    try {
      const note = await getNote(summary.id);
      setActiveNote(note);
      setTitleDraft(note.title);
      setContentDraft(note.content);
      setDirty(false);
      setSaveState('idle');
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '信息记录读取失败，请稍后重试'));
    }
  };

  const createRecord = async (): Promise<void> => {
    setLocalError(null);
    try {
      const note = await addNote();
      setActiveNote(note);
      setTitleDraft(note.title);
      setContentDraft(note.content);
      setDirty(false);
      setSaveState('idle');
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '信息记录创建失败，请稍后重试'));
    }
  };

  const toggleArchive = async (): Promise<void> => {
    if (!activeNote) return;
    setLocalError(null);
    try {
      await setNoteArchived(activeNote.id, !activeNote.archived);
      setActiveNote((current) => current ? { ...current, archived: !current.archived } : current);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '信息记录归档失败，请稍后重试'));
    }
  };

  const confirmDelete = async (): Promise<void> => {
    if (!pendingDelete) return;
    const id = pendingDelete.id;
    setLocalError(null);
    try {
      await deleteNote(id);
      if (activeNote?.id === id) {
        setActiveNote(null);
        setTitleDraft('');
        setContentDraft('');
        setDirty(false);
      }
      setPendingDelete(null);
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '信息记录删除失败，请稍后重试'));
    }
  };

  const exportCurrent = async (): Promise<void> => {
    if (!activeNote) return;
    setLocalError(null);
    try {
      await persistNote();
      await exportNote(activeNote.id);
      setSaveState('saved');
    } catch (error: unknown) {
      setLocalError(toChineseError(error, '信息记录导出失败，请稍后重试'));
    }
  };

  const shown = notes.filter((note) => (tab === 'all' ? !note.archived : note.archived));
  const visibleError = localError ?? notesError;

  // 首次加载完成后自动打开最近一条记录（列表按创建时间倒序，首条即最近）。
  useEffect(() => {
    if (autoOpenedRef.current || notesLoading || notes.length === 0) return;
    autoOpenedRef.current = true;
    const latest = notes[0];
    if (latest) void openNote(latest);
    // openNote 为每次渲染重建的普通函数，这里只触发一次，不列入依赖。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [notes, notesLoading]);

  return (
    <div>
      {visibleError && (
        <div
          role="alert"
          style={{ marginBottom: 16, padding: '8px 10px', borderRadius: 'var(--radius-item)', color: 'var(--danger)', background: 'var(--danger-soft)' }}
        >
          {visibleError}
        </div>
      )}
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12 }}>
        <div style={{ display: 'flex', gap: 10 }}>
          <button
            className="btn"
            aria-label="切换记录列表"
            disabled={notesLoading}
            onClick={() => setListCollapsed((collapsed) => !collapsed)}
          >
            {listCollapsed ? <IconExpand /> : <IconCollapse />}
            {listCollapsed ? '展开列表' : '收起列表'}
          </button>
          <button className="btn is-green" disabled={!activeNote} onClick={() => void exportCurrent()}>
            <IconExport />
            导出 Markdown
          </button>
          <button className="btn is-primary" disabled={notesLoading} onClick={() => void createRecord()}>
            <IconPlus />
            新建记录
          </button>
        </div>
      </div>

      {notesLoading ? (
        <div className="aux-placeholder" style={{ marginTop: 24 }}><span>正在读取信息记录…</span></div>
      ) : (
        <div style={{ display: 'flex', gap: 16, marginTop: 16, alignItems: 'stretch' }}>
          {/* 中间主区域：默认大编辑器；无选中记录时空态 + 新建入口。 */}
          <section style={{ flex: 1, minWidth: 0 }}>
            {activeNote ? (
              <div className="note-editor-panel">
                <div className="settings-row" style={{ padding: 0, border: 0 }}>
                  <input
                    aria-label="记录标题"
                    value={titleDraft}
                    onChange={(event) => { setTitleDraft(event.target.value); setDirty(true); setSaveState('idle'); }}
                    placeholder="记录标题"
                    style={{ flex: 1, padding: '8px 10px', border: '1px solid var(--border)', borderRadius: 'var(--radius-item)', background: 'var(--bg)', fontSize: 16, fontWeight: 600 }}
                  />
                  <button className="btn-icon" aria-label={activeNote.archived ? '取消归档' : '归档'} onClick={() => void toggleArchive()}>
                    <IconArchive />
                  </button>
                  <button className="btn-icon" aria-label="删除当前记录" onClick={() => setPendingDelete(activeNote)}>
                    <IconTrash />
                  </button>
                </div>
                <div className="settings-row-hint" style={{ margin: '8px 0 12px' }}>
                  {saveState === 'saving' ? '正在保存…' : saveState === 'saved' ? '已保存' : saveState === 'error' ? '保存失败' : '自动保存：停止编辑 2 秒后保存'}
                </div>
                <MarkdownEditor
                  key={activeNote.id}
                  value={contentDraft}
                  onChange={(markdown) => { setContentDraft(markdown); setDirty(true); setSaveState('idle'); }}
                />
              </div>
            ) : (
              <div className="aux-placeholder" style={{ minHeight: 360, border: '1px dashed var(--border)', borderRadius: 'var(--radius-card)' }}>
                <IconNote />
                <span>还没有打开的记录</span>
                <button className="btn is-primary" disabled={notesLoading} onClick={() => void createRecord()}>
                  <IconPlus />
                  新建记录
                </button>
              </div>
            )}
          </section>

          {/* 右侧记录列表：可收起/展开。 */}
          {!listCollapsed && (
            <aside style={{ width: 280, flex: 'none', display: 'flex', flexDirection: 'column', gap: 10 }}>
              <div className="filter-tabs">
                <button className={`filter-tab${tab === 'all' ? ' is-active' : ''}`} onClick={() => setTab('all')}>
                  全部
                </button>
                <button className={`filter-tab${tab === 'archived' ? ' is-active' : ''}`} onClick={() => setTab('archived')}>
                  已归档
                </button>
              </div>
              <div className="note-grid" style={{ alignContent: 'start', overflowY: 'auto' }}>
                {shown.length === 0 ? (
                  <div className="aux-placeholder"><span>{tab === 'all' ? '暂无记录' : '没有已归档的记录'}</span></div>
                ) : shown.map((note) => (
                  <button
                    key={note.id}
                    className="note-card"
                    style={{ textAlign: 'left', border: note.id === activeNote?.id ? '1px solid var(--primary)' : undefined }}
                    onClick={() => void openNote(note)}
                  >
                    <div className="note-card-title">{note.title}</div>
                    <div className="note-card-excerpt">点击打开 Markdown 编辑器</div>
                    <div className="note-card-meta">
                      <span className="note-badge">{note.archived ? '已归档' : '进行中'}</span>
                      <span>{fmtTime(note.updatedAt)}</span>
                      <span style={{ marginLeft: 'auto' }}><IconArchive size={14} /></span>
                    </div>
                  </button>
                ))}
              </div>
            </aside>
          )}
        </div>
      )}

      {pendingDelete && (
        <ConfirmDialog title="删除记录" confirmText="删除" onConfirm={() => void confirmDelete()} onCancel={() => setPendingDelete(null)}>
          确定删除「{pendingDelete.title}」？该操作不可恢复。
        </ConfirmDialog>
      )}
    </div>
  );
}
