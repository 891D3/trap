// NoteView —— 信息记录 v2（M0.5，ARdot 35:1 / 39:41 对应实现）：
//   视图 Tab（灵感速记 | 笔记文档）+ 双模式主体：
//   • 灵感速记态：速记输入卡 + 灵感卡流（306 卡宽流式分栏）+ 归档浮层；
//   • 笔记文档态：编辑器卡（文档头 21 Bold 组合标题/保存状态/预览切换/删除/
//     项目 chip/字数）+ Milkdown 正文（v3 排版对齐）。
// 数据全部经 NoteService v2 typed IPC（N1），组件不触碰 IPC 细节。
import { useEffect, useMemo, useRef, useState } from 'react';
import { ConfirmDialog } from '../../components/dialogs';
import { IconCalendar, IconNote, IconPencil, IconPlus, IconTrash } from '../../components/icons';
import {
  createNote,
  deleteNote,
  exportNotes,
  getNoteV2,
  saveNoteV2,
} from '../../api/notes';
import { toChineseError } from '../../api/app-settings';
import { useStore, type NoteSummaryV2, type NoteV2 } from '../../state/app-store';

type NoteTab = 'spark' | 'doc';

/** 灵感卡 meta 行的短时间（HH:mm）。 */
function shortTime(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

/** 归档日期 MM-DD。 */
function shortDate(iso: string | null): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export function NoteView() {
  const {
    sparks,
    sparksArchived,
    sparksError,
    addSpark,
    promoteSpark,
    deleteSpark,
    archiveSpark,
    restoreSpark,
    notesV2,
    notesV2Loading,
    runSweep,
    projects,
    noteActiveId,
    setNoteActiveId,
  } = useStore();

  const [tab, setTab] = useState<NoteTab>('spark');

  // ---- 灵感速记态 ----
  const [sparkDraft, setSparkDraft] = useState('');
  const [sparkSaving, setSparkSaving] = useState(false);
  const [pendingSparkDelete, setPendingSparkDelete] = useState<NoteSummaryV2 | null>(null);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [archiveShowAll, setArchiveShowAll] = useState(false);
  const [sparkBusy, setSparkBusy] = useState<number | null>(null);

  // ---- 笔记文档态 ----
  const [active, setActive] = useState<NoteV2 | null>(null);
  const [titleDraft, setTitleDraft] = useState('');
  const [contentDraft, setContentDraft] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [preview, setPreview] = useState(false);
  const [projectPickOpen, setProjectPickOpen] = useState(false);
  const [pendingNoteDelete, setPendingNoteDelete] = useState<NoteV2 | null>(null);
  const [noteError, setNoteError] = useState<string | null>(null);

  // 辅助区点选笔记行的联动：外部 setNoteActiveId 后加载该笔记（doc 态）。
  const lastSyncedRef = useRef<number | null>(null);
  useEffect(() => {
    if (noteActiveId !== null && noteActiveId !== lastSyncedRef.current && (active === null || noteActiveId !== active.id)) {
      lastSyncedRef.current = noteActiveId;
      void openNote(noteActiveId);
    }
    if (noteActiveId === null) lastSyncedRef.current = null;
  }, [noteActiveId]);
  // saveNoteV2 成功后会同步 noteActiveId（辅助区高亮目标）。
  useEffect(() => {
    if (active && noteActiveId !== active.id) setNoteActiveId(active.id);
  }, [active, noteActiveId]);

  // 进入灵感 Tab 触发清扫（PRD §3.5.2；启动时 Go 已执行一次）。
  const sweptRef = useRef(false);
  useEffect(() => {
    if (tab === 'spark' && !sweptRef.current) {
      sweptRef.current = true;
      void runSweep();
    }
  }, [tab, runSweep]);

  const submitSpark = async (): Promise<void> => {
    const text = sparkDraft.trim();
    if (!text || sparkSaving) return;
    setSparkSaving(true);
    try {
      await addSpark(text, null);
      setSparkDraft('');
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '灵感保存失败，请稍后重试'));
    } finally {
      setSparkSaving(false);
    }
  };

  const openNote = async (id: number): Promise<void> => {
    try {
      const note = await getNoteV2(id);
      setTab('doc');
      setActive(note);
      setTitleDraft(note.title);
      setContentDraft(note.content);
      setDirty(false);
      setSaveState('idle');
      setPreview(false);
      setNoteError(null);
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '信息记录读取失败，请稍后重试'));
    }
  };

  const createNewNote = async (): Promise<void> => {
    try {
      const created = await createNote('未命名记录');
      const full = await getNoteV2(created.id);
      setTab('doc');
      setActive(full);
      setTitleDraft(full.title);
      setContentDraft(full.content);
      setDirty(false);
      setSaveState('idle');
      setPreview(false);
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '新建记录失败，请稍后重试'));
    }
  };

  // 2s 防抖自动保存（笔记文档态）。
  const titleRef = useRef(titleDraft);
  const contentRef = useRef(contentDraft);
  titleRef.current = titleDraft;
  contentRef.current = contentDraft;

  useEffect(() => {
    if (!active || !dirty) return;
    const timer = window.setTimeout(() => {
      void persist();
    }, 2000);
    return () => window.clearTimeout(timer);
    // persist 依赖 active/dirty；防抖保存语义与 v1 一致。
    function persist(): void {
      if (!active) return;
      setSaveState('saving');
      saveNoteV2(active.id, titleRef.current.trim() || '未命名记录', contentRef.current, active.projectId ?? null)
        .then((saved) => {
          setActive((current) => (current && current.id === saved.id ? saved : current));
          setTitleDraft(saved.title);
          setContentDraft(saved.content);
          setDirty(false);
          setSaveState('saved');
        })
        .catch((error: unknown) => {
          setSaveState('error');
          setNoteError(toChineseError(error, '信息记录保存失败，请稍后重试'));
        });
    }
  }, [active, dirty, titleDraft, contentDraft]);

  const promote = async (sparkID: number, as: 'note' | 'todo'): Promise<void> => {
    setSparkBusy(sparkID);
    try {
      const res = await promoteSpark(sparkID, as);
      if (as === 'note' && res.note) {
        await openNote(res.note.id);
      }
      // todo 模式不跳转：待办角标/列表由 todo.changed 刷新。
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '转换失败，请稍后重试'));
    } finally {
      setSparkBusy(null);
    }
  };

  const removeSpark = async (id: number): Promise<void> => {
    setSparkBusy(id);
    try {
      await deleteSpark(id);
      setPendingSparkDelete(null);
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '灵感删除失败，请稍后重试'));
    } finally {
      setSparkBusy(null);
    }
  };

  const visibleError = noteError ?? sparksError;
  /** appNameOf 返回项目完整信息（色点/名称），无关联返回 null。 */
  const projectOf = (projectId: number | null): { id: number; name: string; color: string } | null => {
    if (projectId == null) return null;
    const p = projects.find((item) => item.id === projectId);
    return p ? { id: p.id, name: p.name, color: p.color } : null;
  };
  const appNameOf = projectOf;

  // ---- 笔记文档态视图 ----
  const docTitle = active
    ? appNameOf(active.projectId)?.name
      ? `${appNameOf(active.projectId)?.name} · ${titleDraft || active.title}`
      : titleDraft || active.title
    : '';

  return (
    <div className="notev2">
      {visibleError && <div role="alert" className="error-box">{visibleError}</div>}

      {/* 视图 Tab 行 */}
      <div className="view-tab-row">
        <button className={`view-tab${tab === 'spark' ? ' is-active' : ''}`} onClick={() => setTab('spark')}>
          <span className="view-tab-label">灵感速记</span>
          <span className="view-tab-underline" />
        </button>
        <button className={`view-tab${tab === 'doc' ? ' is-active' : ''}`} onClick={() => setTab('doc')}>
          <span className="view-tab-label">笔记文档</span>
          <span className="view-tab-underline" />
        </button>
        {/* 弹性填充），右端「新建笔记」仅在笔记文档态出现（设计稿 39:428） */}
        <div className="view-tab-flex" />
        {tab === 'doc' && (
          <button className="btn is-primary view-tab-action" onClick={() => void createNewNote()}>
            <IconPlus size={14} />新建笔记
          </button>
        )}
      </div>

      {tab === 'spark' ? (
        /* ---------- 灵感速记 ---------- */
        <>
          <div className="spark-input-card">
            <span className="spark-zap">⚡</span>
            <input
              className="spark-input"
              placeholder="随手记一条灵感，回车保存…"
              aria-label="灵感速记"
              value={sparkDraft}
              disabled={sparkSaving}
              onChange={(e) => setSparkDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || (e.key === 'Enter' && (e.metaKey || e.ctrlKey))) {
                  e.preventDefault();
                  void submitSpark();
                }
              }}
            />
            <span className="spark-hint">⌘⏎ 快速保存</span>
          </div>

          <div className="spark-flow-head">
            <span className="spark-flow-count">未处理灵感 · {sparks.length}</span>
            <button
              className="spark-archive-btn"
              aria-expanded={archiveOpen}
              onClick={() => { setArchiveOpen((o) => !o); setArchiveShowAll(false); }}
            >
              归档 · {sparksArchived.length}
            </button>
            {archiveOpen && (
              <ArchivePopover
                items={sparksArchived}
                showAll={archiveShowAll}
                onToggleAll={() => setArchiveShowAll((v) => !v)}
                busy={sparkBusy}
                onRestore={(id) => void restoreSpark(id)}
                onDelete={(item) => setPendingSparkDelete(item)}
                onClose={() => setArchiveOpen(false)}
              />
            )}
          </div>

          <div className="spark-flow">
            {sparks.map((spark) => {
              const meta = appNameOf(spark.projectId);
              return (
                <div key={spark.id} className="spark-card">
                  <div className="spark-content">{spark.content}</div>
                  <div className="spark-meta">
                    {meta && <span className={`side-dot proj-${meta.color}`} />}
                    {meta && <span className="spark-proj">#{meta.name}</span>}
                    <span className="spark-time">{shortTime(spark.updatedAt)}</span>
                  </div>
                  <div className="spark-actions">
                    <button
                      className="spark-action"
                      title="转笔记"
                      aria-label={`转笔记 ${spark.title.slice(0, 8)}`}
                      disabled={sparkBusy !== null}
                      onClick={() => void promote(spark.id, 'note')}
                    >
                      <IconNote size={14} />
                    </button>
                    <button
                      className="spark-action"
                      title="转待办"
                      aria-label={`转待办 ${spark.title.slice(0, 8)}`}
                      disabled={sparkBusy !== null}
                      onClick={() => void promote(spark.id, 'todo')}
                    >
                      <IconCalendar size={14} />
                    </button>
                    <button
                      className="spark-action"
                      title="删除"
                      aria-label={`删除 ${spark.title.slice(0, 8)}`}
                      disabled={sparkBusy !== null}
                      onClick={() => setPendingSparkDelete(spark)}
                    >
                      <IconTrash size={14} />
                    </button>
                  </div>
                </div>
              );
            })}
            {sparks.length === 0 && (
              <div className="aux-empty">
                {notesV2Loading ? '正在读取灵感…' : '没有未处理灵感。随手记一下吧 ⚡'}
              </div>
            )}
          </div>
        </>
      ) : (
        /* ---------- 笔记文档 ---------- */
        active ? (
          <section className="note-editor-card">
            <div className="note-doc-head">
              <div className="note-doc-title-row">
                {/* 组合标题=项目 · 标题）。可编辑仅限标题部分（点击进入输入） */}
                <input
                  className="note-doc-title"
                  aria-label="笔记标题"
                  value={appNameOf(active.projectId)?.name ? `${appNameOf(active.projectId)?.name} · ${titleDraft}` : titleDraft}
                  onChange={(e) => setTitleDraft(e.target.value)}
                  onBlur={() => {
                    // 标题带「项目 · 」前缀部分不可改（组合标题只读），保存仅取尾部
                    const tail = titleDraft.split(' · ').slice(titleDraft.split(' · ').length - 1)[0] ?? '';
                    void saveTitleChange(tail);
                  }}
                  disabled={!active}
                />
                <div className="note-doc-actions">
                  <span className={`note-save-state${saveState === 'saved' ? ' is-saved' : ''}`}>
                    <span className="note-save-dot" />
                    {saveState === 'error' ? '保存失败' : saveState === 'saving' ? '保存中…' : dirty ? '未保存' : saveState === 'saved' ? '已自动保存' : ''}
                  </span>
                  <button
                    className="btn-icon"
                    title={preview ? '切换到编辑（代码）' : '切换到预览'}
                    aria-label={preview ? '切换到编辑' : '切换到预览'}
                    onClick={() => setPreview((p) => !p)}
                  >
                    {preview ? <IconPencil size={16} /> : <IconNote size={16} />}
                  </button>
                  <button
                    className="btn-icon"
                    title="删除笔记"
                    aria-label="删除当前记录"
                    onClick={() => setPendingNoteDelete(active)}
                  >
                    <IconTrash size={16} />
                  </button>
                </div>
              </div>
              <div className="note-doc-meta-row">
                <button
                  className="note-doc-project-chip"
                  onClick={() => setProjectPickOpen((o) => !o)}
                  title="关联项目"
                >
                  {active.projectId !== null && appNameOf(active.projectId) ? `# ${appNameOf(active.projectId)?.name}` : '# 未关联'}
                </button>
                <span className="note-doc-stamp">更新于 {stamp(active.updatedAt)}</span>
                <span className="note-doc-stamp">· 约 {wordCount(contentDraft)} 字</span>
                {projectPickOpen && (
                  <ProjectPick
                    activeId={active.projectId}
                    onPick={async (pid) => {
                      try {
                        const updated = await linkNoteProjectSave(active.id, pid, titleDraft, contentDraft);
                        setActive(updated);
                        setProjectPickOpen(false);
                      } catch (error: unknown) {
                        setNoteError(toChineseError(error, '项目关联保存失败'));
                      }
                    }}
                    onClose={() => setProjectPickOpen(false)}
                  />
                )}
              </div>
              <div className="note-doc-divider" />
            </div>
            <div className={`note-doc-body${preview ? ' is-preview' : ''}`}>
              <MarkdownEditorLite
                value={contentDraft}
                onChange={(md) => { setContentDraft(md); setDirty(true); }}
                readOnly={preview}
              />
            </div>
          </section>
        ) : (
          <div className="note-empty" aria-label="未选择记录">
            <div>
              还没有打开的笔记，点右上「新建笔记」或右侧最近笔记列表选择一条
            </div>
            <button className="btn is-primary" onClick={() => void createNewNote()}>新建笔记</button>
          </div>
        )
      )}

      {/* 灵感删除二次确认 */}
      {pendingSparkDelete && (
        <ConfirmDialog
          title="删除灵感"
          confirmText="删除"
          onConfirm={() => void removeSpark(pendingSparkDelete.id)}
          onCancel={() => setPendingSparkDelete(null)}
        >
          确定删除「{pendingSparkDelete.title.slice(0, 30)}」？
        </ConfirmDialog>
      )}

      {/* 笔记删除二次确认（删除后回到打开最近一条或空态） */}
      {pendingNoteDelete && (
        <ConfirmDialog
          title="删除笔记"
          confirmText="删除"
          onConfirm={() => void removeNote(pendingNoteDelete.id)}
          onCancel={() => setPendingNoteDelete(null)}
        >
          确定删除「{pendingNoteDelete.title.slice(0, 30)}」？该操作不可恢复。
        </ConfirmDialog>
      )}
    </div>
  );

  async function saveTitleChange(tail: string): Promise<void> {
    if (!active) return;
    const pure = tail.trim();
    if (!pure || pure === active.title) return;
    try {
      const updated = await saveNoteV2(active.id, pure, active.content, active.projectId ?? null);
      setActive(updated);
      setTitleDraft(updated.title);
      setSaveState('saved');
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '标题保存失败，请稍后重试'));
    }
  }

  async function removeNote(id: number): Promise<void> {
    // 先取消未保存的防抖保存（避免定时器对已删除记录报错并覆盖空态）。
    setDirty(false);
    setSaveState('idle');
    try {
      await deleteNote(id);
      setPendingNoteDelete(null);
      if (active?.id === id) {
        const rest = notesV2.filter((n) => n.id !== id);
        if (rest.length > 0 && rest[0] !== undefined) {
          await openNote(rest[0].id);
        } else {
          setActive(null);
          setNoteActiveId(null);
        }
      }
    } catch (error: unknown) {
      setNoteError(toChineseError(error, '信息记录删除失败，请稍后重试'));
    }
  }

  async function linkNoteProjectSave(id: number, pid: number | null, title: string, content: string) {
    return saveNoteV2(id, title.trim() || '未命名记录', content, pid);
  }
}

/** MarkdownEditorLite：同一套 Milkdown 主题；预览态加只读样式覆盖。 */
import { MarkdownEditor } from '../../components/MarkdownEditor';
function MarkdownEditorLite({ value, onChange, readOnly }: { value: string; onChange: (md: string) => void; readOnly: boolean }) {
  return (
    <div className={readOnly ? 'note-doc-editor is-preview' : 'note-doc-editor'}>
      <MarkdownEditor value={value} onChange={readOnly ? () => undefined : onChange} />
    </div>
  );
}

/* ---------- 归档浮层（39:395） ---------- */
function ArchivePopover(props: {
  items: NoteSummaryV2[];
  showAll: boolean;
  onToggleAll: () => void;
  busy: number | null;
  onRestore: (id: number) => void;
  onDelete: (item: NoteSummaryV2) => void;
  onClose: () => void;
}): React.JSX.Element {
  const { items, showAll, onToggleAll, busy, onRestore, onDelete, onClose } = props;
  // 点外部关闭
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const onDoc = (e: MouseEvent): void => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [onClose]);
  const shown = showAll ? items : items.slice(0, 3);
  return (
    <div className="archive-pop" ref={ref} role="dialog" aria-label="归档">
      <div className="archive-pop-head">
        <span className="archive-pop-title">归档</span>
        <span className="archive-pop-count">{items.length} 条</span>
      </div>
      <p className="archive-pop-hint">未处理灵感 30 天后自动归档，保留 90 天，可随时恢复</p>
      <div className="archive-pop-divider" />
      {shown.map((item) => (
        <div key={item.id} className="archive-item">
          <div className="archive-item-main">
            <div className="archive-item-text">{item.title}</div>
            <div className="archive-item-meta">归档于 {shortDate(item.archivedAt)}</div>
          </div>
          <button
            className="archive-restore"
            disabled={busy !== null}
            onClick={() => onRestore(item.id)}
          >
            恢复
          </button>
          <button
            className="btn-icon"
            aria-label={`删除 ${item.title.slice(0, 8)}`}
            disabled={busy !== null}
            onClick={() => onDelete(item)}
          >
            <IconTrash size={14} />
          </button>
        </div>
      ))}
      {items.length === 0 && <div className="aux-empty">暂无归档灵感</div>}
      <div className="archive-pop-foot">
        {items.length > 3 && (
          <button className="archive-view-all" onClick={onToggleAll}>
            {showAll ? '收起' : `查看全部 ${items.length} 条`}
          </button>
        )}
      </div>
    </div>
  );
}

/* ---------- 项目选择浮层（文档头 chip） ---------- */
function ProjectPick(props: {
  activeId: number | null;
  onPick: (pid: number | null) => void;
  onClose: () => void;
}): React.JSX.Element {
  const { projects } = useStore();
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const onDoc = (e: MouseEvent): void => {
      if (ref.current && !ref.current.contains(e.target as Node)) props.onClose();
    };
    document.addEventListener('mousedown', onDoc);
    return () => document.removeEventListener('mousedown', onDoc);
  }, [props]);
  return (
    <div className="archive-pop project-pick" ref={ref} role="dialog" aria-label="选择项目">
      {projects.map((p) => (
        <button
          key={p.id}
          className={`archive-item project-pick-item${props.activeId === p.id ? ' is-active' : ''}`}
          onClick={() => props.onPick(p.id)}
        >
          <span className={`side-dot proj-${p.color}`} />
          <span className="archive-item-text">{p.name}</span>
        </button>
      ))}
      {/* 取消关联入口 */}
      {props.activeId !== null && (
        <button
          className="archive-item project-pick-item"
          onClick={() => props.onPick(null)}
          aria-label="取消关联项目"
        >
          <span className="side-dot" style={{ background: 'var(--text-3)' }} />
          <span className="archive-item-text">未关联</span>
        </button>
      )}
      {projects.length === 0 && <div className="aux-empty">还没有项目</div>}
    </div>
  );
}

/** stamp() = 「今天 10:24」样式的更新时间。 */
function stamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  const hhmm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
  if (sameDay) return `今天 ${hhmm}`;
  const mm = String(d.getMonth() + 1).padStart(2, '0');
  const dd = String(d.getDate()).padStart(2, '0');
  return `${mm}-${dd} ${hhmm}`;
}

function wordCount(text: string): string {
  // 简单可视口径：CJK 字数 + 英文词数（近似,不逐一区分）。
  const cjk = (text.match(/[\u4e00-\u9fff]/g) ?? []).length;
  const words = (text.match(/[A-Za-z0-9']+/g) ?? []).length;
  return (cjk + words).toLocaleString();
}

