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
import { subscribeSettingsChanged } from '../realtime/settings-events';

export type { ThemeSetting } from '../api/app-settings';

/* ---------- 实体类型（T2 与 Go 侧对齐） ---------- */

export type Todo = {
  id: number;
  title: string;
  done: boolean;
  completedAt: string | null;
  createdAt: string;
};

export type Note = {
  id: number;
  title: string;
  content: string;
  archived: boolean;
  updatedAt: string;
};

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

const initialTodos: Todo[] = [
  { id: 1, title: '阅读 mygo bindings 文档', done: false, completedAt: null, createdAt: now() },
  { id: 2, title: '整理上周会议纪要', done: true, completedAt: now(), createdAt: now() },
  { id: 3, title: '给 BRD 补充 F5 项目工作细节', done: false, completedAt: null, createdAt: now() },
];

const initialNotes: Note[] = [
  {
    id: 10,
    title: 'Trap 架构想法',
    content: '工作流引擎与 AI 助手在引擎层汇合，Agent 可触发/编排工作流。\n\n- typed IPC 已验证\n- Milkdown T5 接入',
    archived: false,
    updatedAt: now(),
  },
  {
    id: 11,
    title: '灵感：看板指标',
    content: '待办完成趋势（日/周）、时间投入分布、项目进度总览。',
    archived: true,
    updatedAt: now(),
  },
];

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
  addTodo: (title: string) => void;
  setTodoDone: (id: number, done: boolean) => void;
  deleteTodo: (id: number) => void;
  notes: Note[];
  addNote: () => void;
  setNoteArchived: (id: number, archived: boolean) => void;
  deleteNote: (id: number) => void;
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

  const [todos, setTodos] = useState<Todo[]>(initialTodos);
  const [notes, setNotes] = useState<Note[]>(initialNotes);
  const [links, setLinks] = useState<QuickLink[]>(initialLinks);

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
    addTodo: (title) =>
      setTodos((prev) => [{ id: nextId++, title, done: false, completedAt: null, createdAt: now() }, ...prev]),
    setTodoDone: (id, done) =>
      setTodos((prev) =>
        prev.map((t) => (t.id === id ? { ...t, done, completedAt: done ? now() : null } : t)),
      ),
    deleteTodo: (id) => setTodos((prev) => prev.filter((t) => t.id !== id)),
    notes,
    addNote: () =>
      setNotes((prev) => [{ id: nextId++, title: '未命名记录', content: '', archived: false, updatedAt: now() }, ...prev]),
    setNoteArchived: (id, archived) =>
      setNotes((prev) => prev.map((n) => (n.id === id ? { ...n, archived, updatedAt: now() } : n))),
    deleteNote: (id) => setNotes((prev) => prev.filter((n) => n.id !== id)),
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
