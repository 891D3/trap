// T3 阶段的本地状态与 mock 数据；T2 数据层完成后，本模块的动作将改为
// 经 web/src/api/ 调用 Go 服务，事件经 mygo Events 同步（届时本文件删除或收敛为适配层）。
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

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

export type ThemeSetting = 'system' | 'light' | 'dark';

export type ViewId = 'dashboard' | 'todo' | 'note' | 'link' | 'settings';

/* ---------- mock 数据 ---------- */

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

const THEME_KEY = 'trap.theme';

function resolveTheme(setting: ThemeSetting): 'light' | 'dark' {
  if (setting !== 'system') return setting;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

export function useTheme(): [ThemeSetting, (s: ThemeSetting) => void] {
  const [setting, setSetting] = useState<ThemeSetting>(() => {
    const saved = localStorage.getItem(THEME_KEY);
    return saved === 'light' || saved === 'dark' || saved === 'system' ? saved : 'system';
  });

  useEffect(() => {
    const apply = () => {
      document.documentElement.dataset.theme = resolveTheme(setting);
    };
    apply();
    if (setting !== 'system') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    mq.addEventListener('change', apply);
    return () => mq.removeEventListener('change', apply);
  }, [setting]);

  const update = useCallback((s: ThemeSetting) => {
    localStorage.setItem(THEME_KEY, s);
    setSetting(s);
  }, []);

  return [setting, update];
}

/* ---------- 应用状态 ---------- */

type AppStore = {
  user: { name: string };
  renameUser: (name: string) => void;
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
  const [user, setUser] = useState({ name: 'Eric' });
  const [todos, setTodos] = useState<Todo[]>(initialTodos);
  const [notes, setNotes] = useState<Note[]>(initialNotes);
  const [links, setLinks] = useState<QuickLink[]>(initialLinks);

  const doneToday = useMemo(() => {
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    return todos.filter((t) => t.completedAt !== null && new Date(t.completedAt) >= start).length;
  }, [todos]);

  const value: AppStore = {
    user,
    renameUser: (name) => setUser({ name }),
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

export const hostOf = (target: string): string => {
  try {
    return new URL(target).host;
  } catch {
    return target;
  }
};