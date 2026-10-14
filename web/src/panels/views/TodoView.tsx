import { useState } from 'react';
import { toChineseError } from '../../api/app-settings';
import { ConfirmDialog } from '../../components/dialogs';
import { IconPlus, IconTrash } from '../../components/icons';
import { useStore, type Todo } from '../../state/app-store';

/** T4 待办视图：所有增删改均经 Go TodoService，组件不直接访问 IPC。 */
export function TodoView() {
  const {
    todos,
    todosLoading,
    todosError,
    addTodo,
    setTodoDone,
    deleteTodo,
  } = useStore();
  const [draft, setDraft] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Todo | null>(null);
  const [saving, setSaving] = useState<'add' | number | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);

  const submit = async (): Promise<void> => {
    const title = draft.trim();
    if (!title) {
      setLocalError('待办内容不能为空');
      return;
    }
    setLocalError(null);
    setSaving('add');
    try {
      await addTodo(title);
      setDraft('');
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

  const confirmDelete = async (): Promise<void> => {
    if (!pendingDelete) return;
    const todo = pendingDelete;
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

  const active = todos.filter((todo) => !todo.done);
  const done = todos.filter((todo) => todo.done);
  const visibleError = localError ?? todosError;

  const renderItem = (todo: Todo) => (
    <li key={todo.id} className={`todo-item${todo.done ? ' is-done' : ''}`}>
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
      <span className="todo-text">{todo.title}</span>
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

  return (
    <div>
      {visibleError && (
        <div
          role="alert"
          style={{
            marginBottom: 16,
            padding: '8px 10px',
            borderRadius: 'var(--radius-item)',
            color: 'var(--danger)',
            background: 'var(--danger-soft)',
          }}
        >
          {visibleError}
        </div>
      )}
      <div className="todo-add-row">
        <input
          className="todo-input"
          value={draft}
          disabled={todosLoading || saving !== null}
          placeholder="添加待办，回车提交"
          onChange={(event) => setDraft(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              void submit();
            }
          }}
        />
        <button className="btn is-primary" disabled={todosLoading || saving !== null} onClick={() => void submit()}>
          <IconPlus />
          添加
        </button>
      </div>

      {todosLoading ? (
        <div className="aux-placeholder" style={{ marginTop: 24 }}>
          <span>正在读取待办…</span>
        </div>
      ) : active.length === 0 && done.length === 0 ? (
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
          onConfirm={() => void confirmDelete()}
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
