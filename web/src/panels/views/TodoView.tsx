import { useState } from 'react';
import { ConfirmDialog } from '../../components/dialogs';
import { IconPlus, IconTrash } from '../../components/icons';
import { useStore, type Todo } from '../../state/app-store';

export function TodoView() {
  const { todos, addTodo, setTodoDone, deleteTodo } = useStore();
  const [draft, setDraft] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Todo | null>(null);

  const submit = () => {
    const title = draft.trim();
    if (!title) return;
    addTodo(title);
    setDraft('');
  };

  const active = todos.filter((t) => !t.done);
  const done = todos.filter((t) => t.done);

  const renderItem = (t: Todo) => (
    <li key={t.id} className={`todo-item${t.done ? ' is-done' : ''}`}>
      <button
        className="todo-check"
        role="checkbox"
        aria-checked={t.done}
        aria-label={t.done ? `恢复 ${t.title}` : `完成 ${t.title}`}
        onClick={() => setTodoDone(t.id, !t.done)}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3.2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M4 12.5 9.5 18 20 6.5" />
        </svg>
      </button>
      <span className="todo-text">{t.title}</span>
      <button className="btn-icon" aria-label={`删除 ${t.title}`} onClick={() => setPendingDelete(t)}>
        <IconTrash />
      </button>
    </li>
  );

  return (
    <div>
      <input
        className="todo-input"
        value={draft}
        placeholder="添加待办，回车提交"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && submit()}
      />

      {active.length === 0 && done.length === 0 ? (
        <div className="aux-placeholder" style={{ marginTop: 24 }}>
          <IconPlus />
          <span>还没有待办，输入一条开始</span>
        </div>
      ) : (
        <>
          <ul className="todo-list">{active.map(renderItem)}</ul>
          {done.length > 0 && <div className="todo-done-divider">已完成 · {done.length}</div>}
          <ul className="todo-list">{done.map(renderItem)}</ul>
        </>
      )}

      {pendingDelete && (
        <ConfirmDialog
          title="删除待办"
          confirmText="删除"
          onConfirm={() => {
            deleteTodo(pendingDelete.id);
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