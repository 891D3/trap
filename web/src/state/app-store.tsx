import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import {
  DEFAULT_APP_SETTINGS,
  getAppSettings,
  setAppTheme,
  setExportDirectory,
  toChineseError,
  updateUserProfile,
  type AppSettings,
  type ThemeSetting,
} from '../api/app-settings';
import { listTodos, createTodo, updateTodoDone, removeTodo, type Todo } from '../api/todos';
import {
  createNote,
  deleteNote as deleteNoteAPI,
  exportNotes,
  getNote,
  listNotes,
  saveNote as saveNoteAPI,
  setNoteArchived,
  type Note,
  type NoteSummary,
} from '../api/notes';
import { subscribeSettingsChanged } from '../realtime/settings-events';
import { subscribeTodoChanged } from '../realtime/todo-events';
import { subscribeNoteChanged } from '../realtime/note-events';

export type { ThemeSetting } from '../api/app-settings';
export type { Todo } from '../api/todos';
export type { Note, NoteSummary } from '../api/notes';

/* ---------- 实体类型（T2 与 Go 侧对齐） ---------- */

export type QuickLink = {
  id: number;
  name: string;
  target: string;
  kind: 'url' | 'path';
  groupName: string;
};

export type ViewId = 'dashboard' | 'todo' | 'note' | 'link' | 'settings';

/* ---------- 仍未接入 T4–T6 的 mock 数据 ---------- */

const now = () => new Date().toISOString();

let nextId = 100;

const initialLinks: QuickLink[] = [
  { id: 20, name: 'GitHub', target: 'https://github.com', kind: 'url', groupName: '开发' },
  { id: 21, name: 'mygo 仓库', target: 'https://github.com/egoist/mygo', kind: 'url', groupName: '开发' },
  { id: 22, name: 'npm', target: 'https://www.npmjs.com', kind: 'url', groupName: '开发' },
];

// v2 样式：取模板文件图标的色板（blue/green/orange/purple/pink-500），与图标块浅底搭配
const avatarColors = ['#3b82f6', '#22c55e', '#f97316', '#a855f7', '#ec4899'];

export const avatarColorFor = (name: string): string => {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0;
  return avatarColors[hash % avatarColors.length] ?? '#3b82f6';
};

/* ---------- 主题 ---------- */

function resolveTheme(setting: ThemeSetting): 'light' | 'dark' {
  if (setting !== 'system') return setting;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/* ---------- 应用状态 ---------- */

type AppStore = {
  user: { name: string; avatarPath: string };
  renameUser: (name: string) => Promise<void>;
  theme: ThemeSetting;
  setTheme: (setting: ThemeSetting) => Promise<void>;
  exportDir: string;
  dataDir: string;
  settingsLoading: boolean;
  settingsError: string | null;
  setExportDir: (directory: string) => Promise<void>;
  todos: Todo[];
  todosLoading: boolean;
  todosError: string | null;
  addTodo: (title: string) => Promise<void>;
  setTodoDone: (id: number, done: boolean) => Promise<void>;
  deleteTodo: (id: number) => Promise<void>;
  notes: NoteSummary[];
  notesLoading: boolean;
  notesError: string | null;
  addNote: (title?: string) => Promise<Note>;
  getNote: (id: number) => Promise<Note>;
  saveNote: (id: number, title: string, content: string) => Promise<Note>;
  setNoteArchived: (id: number, archived: boolean) => Promise<void>;
  deleteNote: (id: number) => Promise<void>;
  exportNote: (id: number) => Promise<string>;
  links: QuickLink[];
  addLink: (name: string, target: string, groupName: string) => void;
  deleteLink: (id: number) => void;
  doneToday: number;
};

const StoreContext = createContext<AppStore | null>(null);

export function AppStoreProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_APP_SETTINGS);
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [settingsError, setSettingsError] = useState<string | null>(null);
  const settingsEventVersion = useRef(0);

  // 首次从 Go 加载设置，并在 Provider 生命周期内统一订阅 settings.changed。
  useEffect(() => {
    let active = true;
    const unsubscribe = subscribeSettingsChanged((nextSettings) => {
      if (!active) return;
      settingsEventVersion.current += 1;
      setSettings(nextSettings);
      setSettingsError(null);
    });
    const requestedEventVersion = settingsEventVersion.current;

    void getAppSettings()
      .then((nextSettings) => {
        // 如果请求期间已经收到其他窗口的更新，避免旧响应覆盖事件中的新值。
        if (active && settingsEventVersion.current === requestedEventVersion) {
          setSettings(nextSettings);
        }
      })
      .catch((error: unknown) => {
        if (active) setSettingsError(toChineseError(error, '设置加载失败，请稍后重试'));
      })
      .finally(() => {
        if (active) setSettingsLoading(false);
      });

    return () => {
      active = false;
      unsubscribe();
    };
  }, []);

  const renameUser = useCallback(
    async (name: string): Promise<void> => {
      const trimmedName = name.trim();
      if (!trimmedName) {
        const error = new Error('姓名不能为空');
        setSettingsError(error.message);
        throw error;
      }

      const previous = settings;
      setSettings((current) => ({ ...current, name: trimmedName }));
      setSettingsError(null);
      try {
        const nextSettings = await updateUserProfile(trimmedName, previous.avatarPath);
        setSettings(nextSettings);
      } catch (error: unknown) {
        const message = toChineseError(error, '用户资料保存失败，请稍后重试');
        setSettings(previous);
        setSettingsError(message);
        throw new Error(message);
      }
    },
    [settings],
  );

  const updateTheme = useCallback(
    async (nextTheme: ThemeSetting): Promise<void> => {
      const previous = settings;
      setSettings((current) => ({ ...current, theme: nextTheme }));
      setSettingsError(null);
      try {
        await setAppTheme(nextTheme);
      } catch (error: unknown) {
        const message = toChineseError(error, '主题保存失败，请稍后重试');
        setSettings(previous);
        setSettingsError(message);
        throw new Error(message);
      }
    },
    [settings],
  );

  const updateExportDir = useCallback(
    async (directory: string): Promise<void> => {
      const previous = settings;
      setSettings((current) => ({ ...current, exportDir: directory }));
      setSettingsError(null);
      try {
        await setExportDirectory(directory);
      } catch (error: unknown) {
        const message = toChineseError(error, '导出目录保存失败，请稍后重试');
        setSettings(previous);
        setSettingsError(message);
        throw new Error(message);
      }
    },
    [settings],
  );

  const [todos, setTodos] = useState<Todo[]>([]);
  const [todosLoading, setTodosLoading] = useState(true);
  const [todosError, setTodosError] = useState<string | null>(null);
  const [notes, setNotes] = useState<NoteSummary[]>([]);
  const [notesLoading, setNotesLoading] = useState(true);
  const [notesError, setNotesError] = useState<string | null>(null);
  const [links, setLinks] = useState<QuickLink[]>(initialLinks);

  const loadTodos = useCallback(async (showLoading: boolean): Promise<void> => {
    if (showLoading) setTodosLoading(true);
    try {
      setTodos(await listTodos());
      setTodosError(null);
    } catch (error: unknown) {
      setTodosError(toChineseError(error, '待办加载失败，请稍后重试'));
    } finally {
      if (showLoading) setTodosLoading(false);
    }
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeTodoChanged(() => {
      // 事件只表示发生变化，列表以 Go List 返回的权威数据为准。
      void loadTodos(false);
    });
    void loadTodos(true);
    return unsubscribe;
  }, [loadTodos]);

  const addTodo = useCallback(async (title: string): Promise<void> => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      const error = new Error('待办内容不能为空');
      setTodosError(error.message);
      throw error;
    }
    setTodosError(null);
    try {
      const created = await createTodo({ title: trimmedTitle });
      setTodos((current) => [...current.filter((todo) => todo.id !== created.id), created]);
    } catch (error: unknown) {
      const message = toChineseError(error, '待办添加失败，请稍后重试');
      setTodosError(message);
      throw new Error(message);
    }
  }, []);

  const setTodoDone = useCallback(async (id: number, done: boolean): Promise<void> => {
    setTodosError(null);
    try {
      const updated = await updateTodoDone(id, done);
      setTodos((current) => current.map((todo) => (todo.id === updated.id ? updated : todo)));
    } catch (error: unknown) {
      const message = toChineseError(error, '待办状态保存失败，请稍后重试');
      setTodosError(message);
      throw new Error(message);
    }
  }, []);

  const deleteTodo = useCallback(async (id: number): Promise<void> => {
    setTodosError(null);
    try {
      await removeTodo(id);
      setTodos((current) => current.filter((todo) => todo.id !== id));
    } catch (error: unknown) {
      const message = toChineseError(error, '待办删除失败，请稍后重试');
      setTodosError(message);
      throw new Error(message);
    }
  }, []);

  const loadNotes = useCallback(async (showLoading: boolean): Promise<void> => {
    if (showLoading) setNotesLoading(true);
    try {
      setNotes(await listNotes(true));
      setNotesError(null);
    } catch (error: unknown) {
      setNotesError(toChineseError(error, '信息记录加载失败，请稍后重试'));
    } finally {
      if (showLoading) setNotesLoading(false);
    }
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeNoteChanged(() => {
      void loadNotes(false);
    });
    void loadNotes(true);
    return unsubscribe;
  }, [loadNotes]);

  const addNote = useCallback(async (title = '未命名记录'): Promise<Note> => {
    setNotesError(null);
    try {
      const note = await createNote(title);
      setNotes((current) => [
        { id: note.id, title: note.title, archived: note.archived, createdAt: note.createdAt, updatedAt: note.updatedAt },
        ...current.filter((item) => item.id !== note.id),
      ]);
      return note;
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录创建失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const getNoteAction = useCallback(async (id: number): Promise<Note> => {
    try {
      return await getNote(id);
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录读取失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const saveNoteAction = useCallback(async (id: number, title: string, content: string): Promise<Note> => {
    setNotesError(null);
    try {
      const note = await saveNoteAPI(id, title, content);
      setNotes((current) => current.map((item) => item.id === note.id
        ? { id: note.id, title: note.title, archived: note.archived, createdAt: note.createdAt, updatedAt: note.updatedAt }
        : item));
      return note;
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录保存失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const setNoteArchivedAction = useCallback(async (id: number, archived: boolean): Promise<void> => {
    setNotesError(null);
    try {
      const note = await setNoteArchived(id, archived);
      setNotes((current) => current.map((item) => item.id === note.id
        ? { id: note.id, title: note.title, archived: note.archived, createdAt: note.createdAt, updatedAt: note.updatedAt }
        : item));
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录归档失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const deleteNoteAction = useCallback(async (id: number): Promise<void> => {
    setNotesError(null);
    try {
      await deleteNoteAPI(id);
      setNotes((current) => current.filter((item) => item.id !== id));
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录删除失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const exportNoteAction = useCallback(async (id: number): Promise<string> => {
    setNotesError(null);
    try {
      const result = await exportNotes([id]);
      const file = result.files[0];
      if (!file) throw new Error('导出未生成文件');
      return file;
    } catch (error: unknown) {
      const message = toChineseError(error, '信息记录导出失败，请稍后重试');
      setNotesError(message);
      throw new Error(message);
    }
  }, []);

  const doneToday = useMemo(() => {
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    return todos.filter((t) => t.completedAt !== null && new Date(t.completedAt) >= start).length;
  }, [todos]);

  const value: AppStore = {
    user: { name: settings.name, avatarPath: settings.avatarPath },
    renameUser,
    theme: settings.theme,
    setTheme: updateTheme,
    exportDir: settings.exportDir,
    dataDir: settings.dataDir,
    settingsLoading,
    settingsError,
    setExportDir: updateExportDir,
    todos,
    todosLoading,
    todosError,
    addTodo,
    setTodoDone,
    deleteTodo,
    notes,
    notesLoading,
    notesError,
    addNote,
    getNote: getNoteAction,
    saveNote: saveNoteAction,
    setNoteArchived: setNoteArchivedAction,
    deleteNote: deleteNoteAction,
    exportNote: exportNoteAction,
    links,
    addLink: (name, target, groupName) => {
      const kind = /^https?:\/\//i.test(target) ? ('url' as const) : ('path' as const);
      setLinks((prev) => [{ id: nextId++, name, target, kind, groupName }, ...prev]);
    },
    deleteLink: (id) => setLinks((prev) => prev.filter((l) => l.id !== id)),
    doneToday,
  };

  return <StoreContext.Provider value={value}>{children}</StoreContext.Provider>;
}

export function useStore(): AppStore {
  const ctx = useContext(StoreContext);
  if (!ctx) throw new Error('useStore 必须在 AppStoreProvider 内使用');
  return ctx;
}

/** 读取并应用当前状态中的主题；设置写入由 AppStoreProvider 负责。 */
export function useTheme(): [ThemeSetting, (setting: ThemeSetting) => Promise<void>] {
  const { theme, setTheme } = useStore();

  useEffect(() => {
    const apply = () => {
      document.documentElement.dataset.theme = resolveTheme(theme);
    };
    apply();
    if (theme !== 'system') return;
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    mediaQuery.addEventListener('change', apply);
    return () => mediaQuery.removeEventListener('change', apply);
  }, [theme]);

  return [theme, setTheme];
}

export const hostOf = (target: string): string => {
  try {
    return new URL(target).host;
  } catch {
    return target;
  }
};
