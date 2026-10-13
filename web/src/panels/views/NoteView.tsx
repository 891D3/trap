import { useState } from 'react';
import { ConfirmDialog } from '../../components/dialogs';
import { IconArchive, IconExport, IconPlus, IconTrash } from '../../components/icons';
import { useStore, type Note } from '../../state/app-store';

const fmtTime = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' });

export function NoteView() {
  const { notes, addNote, setNoteArchived, deleteNote } = useStore();
  const [tab, setTab] = useState<'all' | 'archived'>('all');
  const [pendingDelete, setPendingDelete] = useState<Note | null>(null);

  const shown = notes.filter((n) => (tab === 'all' ? !n.archived : n.archived));

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12 }}>
        <div className="filter-tabs">
          <button className={`filter-tab${tab === 'all' ? ' is-active' : ''}`} onClick={() => setTab('all')}>
            全部
          </button>
          <button
            className={`filter-tab${tab === 'archived' ? ' is-active' : ''}`}
            onClick={() => setTab('archived')}
          >
            已归档
          </button>
        </div>
        <div style={{ display: 'flex', gap: 10 }}>
          <button className="btn" disabled title="T2 数据层完成后开放">
            <IconExport />
            整理导出
          </button>
          <button className="btn is-primary" onClick={addNote}>
            <IconPlus />
            新建记录
          </button>
        </div>
      </div>

      {shown.length === 0 ? (
        <div className="aux-placeholder" style={{ marginTop: 24 }}>
          <span>{tab === 'all' ? '暂无记录' : '没有已归档的记录'}</span>
        </div>
      ) : (
        <div className="note-grid">
          {shown.map((n) => (
            <div className="note-card" key={n.id}>
              <div className="note-card-title">{n.title}</div>
              <div className="note-card-excerpt">{n.content || '（空记录，编辑器 T5 接入）'}</div>
              <div className="note-card-meta">
                <span className="note-badge">{n.archived ? '已归档' : '进行中'}</span>
                <span>{fmtTime(n.updatedAt)}</span>
                <span style={{ marginLeft: 'auto', display: 'flex' }}>
                  <button
                    className="btn-icon"
                    aria-label={n.archived ? '取消归档' : '归档'}
                    title={n.archived ? '取消归档' : '归档'}
                    onClick={() => setNoteArchived(n.id, !n.archived)}
                  >
                    <IconArchive />
                  </button>
                  <button className="btn-icon" aria-label={`删除 ${n.title}`} onClick={() => setPendingDelete(n)}>
                    <IconTrash />
                  </button>
                </span>
              </div>
            </div>
          ))}
        </div>
      )}

      {pendingDelete && (
        <ConfirmDialog
          title="删除记录"
          confirmText="删除"
          onConfirm={() => {
            deleteNote(pendingDelete.id);
            setPendingDelete(null);
          }}
          onCancel={() => setPendingDelete(null)}
        >
          确定删除「{pendingDelete.title}」？该操作不可恢复。
        </ConfirmDialog>
      )}
    </div>
  );
}