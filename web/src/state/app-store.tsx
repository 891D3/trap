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
import { dashboardStats, listTodos, createTodo, updateTodo, updateTodoDone, removeTodo, type CreateTodoInput, type Todo } from '../api/todos';
import {
  createLink,
  listLinks,
  openLink,
  removeLink,
  saveLink,
  type QuickLink,
} from '../api/links';
import type { DashboardStats } from '../mygo';
import {
  createProject,
  listProjects,
  removeProject,
  updateProject,
  type Project,
} from '../api/projects';
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
import { subscribeProjectChanged } from '../realtime/project-events';
import { subscribeQuickLinkChanged } from '../realtime/quicklink-events';

export type { ThemeSetting } from '../api/app-settings';
export type { Todo } from '../api/todos';
export type { Note, NoteSummary } from '../api/notes';
export type { Project } from '../api/projects';

/* ---------- 实体类型（T2 与 Go 侧对齐） ---------- */

export type { QuickLink } from '../api/links';

export type ViewId = 'dashboard' | 'todo' | 'note' | 'stock' | 'link' | 'settings';

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
  /** 当前视图（v3 默认数据看板；视图切换需要跨组件，R4 起入 store）。 */
  view: ViewId;
  setView: (v: ViewId) => void;
  /** 数据看板聚合（PRD v2.0 §4.2；事件驱动刷新）。 */
  stats: DashboardStats | null;
  todos: Todo[];
  todosLoading: boolean;
  todosError: string | null;
  addTodo: (title: string) => Promise<void>;
  /** v2.0：带完整字段创建（分类/日期/项目），供待办快速创建卡使用。 */
  addTodoFull: (input: CreateTodoInput) => Promise<Todo>;
  /** v2.0：行内编辑保存（全量更新可编辑字段）。 */
  editTodo: (id: number, input: CreateTodoInput) => Promise<Todo>;
  /** 月历/按属性 点击的过滤态（辅助区 ↔ 列表卡联动；null = 清除）。 */
  auxDateFilter: string | null; // yyyy-mm-dd（本地日期）
  setAuxDateFilter: (d: string | null) => void;
  auxCategoryFilter: string;
  setAuxCategoryFilter: (c: string) => void;
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
  /** v2.0 R6：快捷入口真实服务（CRUD/分组/打开）。 */
  links: QuickLink[];
  linksLoading: boolean;
  linksError: string | null;
  addLink: (name: string, target: string, groupName: string) => Promise<void>;
  editLink: (id: number, name: string, target: string, groupName: string, sortOrder: number) => Promise<void>;
  deleteLink: (id: number) => Promise<void>;
  openLink: (id: number) => Promise<void>;
  doneToday: number;
  /** v2.0：项目列表（含待办计数）与 CRUD。 */
  projects: Project[];
  projectsLoading: boolean;
  projectsError: string | null;
  addProject: (name: string, color?: string) => Promise<void>;
  editProject: (id: number, name: string, color: string) => Promise<void>;
  deleteProject: (id: number) => Promise<void>;
  /** 按项目过滤（点击侧栏项目进入待办过滤态；null = 不过滤）。 */
  projectFilter: number | null;
  openProject: (id: number) => void;
  clearProjectFilter: () => void;
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

  // 当前视图（默认数据看板 = PRD v2.0 拟定决策 1；深链接仍由 App 初始化读取）。
  const [view, setView] = useState<ViewId>('dashboard');
  const [stats, setStats] = useState<DashboardStats | null>(null);

  const loadStats = useCallback(async (): Promise<void> => {
    try {
      setStats(await dashboardStats());
    } catch (error: unknown) {
      // 看板聚合失败不阻塞视图（KPI 显示零值 + toast 由视图层处理）。
      console.error('看板聚合加载失败', error);
    }
  }, []);

  const [todos, setTodos] = useState<Todo[]>([]);
  const [todosLoading, setTodosLoading] = useState(true);
  const [todosError, setTodosError] = useState<string | null>(null);
  const [notes, setNotes] = useState<NoteSummary[]>([]);
  const [notesLoading, setNotesLoading] = useState(true);
  const [notesError, setNotesError] = useState<string | null>(null);
  const [links, setLinks] = useState<QuickLink[]>([]);
  const [linksLoading, setLinksLoading] = useState(true);
  const [linksError, setLinksError] = useState<string | null>(null);

  const loadLinks = useCallback(async (showLoading: boolean): Promise<void> => {
    if (showLoading) setLinksLoading(true);
    try {
      setLinks(await listLinks());
      setLinksError(null);
    } catch (error: unknown) {
      setLinksError(toChineseError(error, '快捷入口加载失败，请稍后重试'));
    } finally {
      if (showLoading) setLinksLoading(false);
    }
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeQuickLinkChanged(() => {
      void loadLinks(false);
    });
    void loadLinks(true);
    return unsubscribe;
  }, [loadLinks]);

  const addLinkAction = useCallback(
    async (name: string, target: string, groupName: string): Promise<void> => {
      try {
        const created = await createLink(name, target, groupName);
        setLinks((current) => [...current, created]);
      } catch (error: unknown) {
        const message = toChineseError(error, '快捷入口添加失败，请稍后重试');
        throw new Error(message);
      }
    },
    [],
  );

  const editLinkAction = useCallback(
    async (id: number, name: string, target: string, groupName: string, sortOrder: number): Promise<void> => {
      try {
        const updated = await saveLink(id, name, target, groupName, sortOrder);
        setLinks((current) => current.map((l) => (l.id === updated.id ? updated : l)));
      } catch (error: unknown) {
        const message = toChineseError(error, '快捷入口保存失败，请稍后重试');
        throw new Error(message);
      }
    },
    [],
  );

  const deleteLinkAction = useCallback(async (id: number): Promise<void> => {
    try {
      await removeLink(id);
      setLinks((current) => current.filter((l) => l.id !== id));
    } catch (error: unknown) {
      const message = toChineseError(error, '快捷入口删除失败，请稍后重试');
      throw new Error(message);
    }
  }, []);

  const openLinkAction = useCallback(async (id: number): Promise<void> => {
    await openLink(id);
  }, []);
  const [projects, setProjects] = useState<Project[]>([]);
  const [projectsLoading, setProjectsLoading] = useState(true);
  const [projectsError, setProjectsError] = useState<string | null>(null);
  const [projectFilter, setProjectFilter] = useState<number | null>(null);

  const loadProjects = useCallback(async (showLoading: boolean): Promise<void> => {
    if (showLoading) setProjectsLoading(true);
    try {
      setProjects(await listProjects());
      setProjectsError(null);
    } catch (error: unknown) {
      setProjectsError(toChineseError(error, '项目加载失败，请稍后重试'));
    } finally {
      if (showLoading) setProjectsLoading(false);
    }
  }, []);


  const addProjectAction = useCallback(
    async (name: string, color = ''): Promise<void> => {
      if (!name.trim()) {
        setProjectsError('项目名称不能为空');
        throw new Error('项目名称不能为空');
      }
      try {
        const created = await createProject(name, color);
        setProjects((current) => [...current, created]);
      } catch (error: unknown) {
        const message = toChineseError(error, '项目添加失败，请稍后重试');
        setProjectsError(message);
        throw new Error(message);
      }
    },
    [],
  );

  const editProjectAction = useCallback(
    async (id: number, name: string, color: string): Promise<void> => {
      try {
        const sortOrder = projects.find((p) => p.id === id)?.sortOrder ?? 0;
        const updated = await updateProject(id, name, color, sortOrder);
        setProjects((current) => current.map((p) => (p.id === updated.id ? updated : p)));
      } catch (error: unknown) {
        const message = toChineseError(error, '项目保存失败，请稍后重试');
        throw new Error(message);
      }
    },
    [projects],
  );

  const deleteProjectAction = useCallback(async (id: number): Promise<void> => {
    try {
      await removeProject(id);
      setProjects((current) => current.filter((p) => p.id !== id));
    } catch (error: unknown) {
      const message = toChineseError(error, '项目删除失败，请稍后重试');
      throw new Error(message);
    }
  }, []);

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
      void loadStats();
    });
    void loadTodos(true);
    void loadStats();
    return unsubscribe;
  }, [loadTodos, loadStats]);

  // 记录变化同样影响看板聚合（今日信息时间轴、灵感 KPI）。
  useEffect(() => {
    const unsubscribe = subscribeNoteChanged(() => {
      void loadStats();
    });
    return unsubscribe;
  }, [loadStats]);

  useEffect(() => {
    const unsubscribe = subscribeProjectChanged((payload) => {
      void loadProjects(false);
      void loadStats();
      if (payload.action === 'deleted') {
        void listTodos().then(setTodos).catch(() => undefined);
        setProjectFilter((current) => (current === payload.projectId ? null : current));
      }
    });
    return unsubscribe;
  }, [loadProjects, loadStats]);

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

  const addTodoFullAction = useCallback(
    async (input: CreateTodoInput): Promise<Todo> => {
      const created = await createTodo(input);
      setTodos((current) => [...current.filter((todo) => todo.id !== created.id), created]);
      setTodosError(null);
      return created;
    },
    [],
  );

  const editTodoAction = useCallback(
    async (id: number, input: CreateTodoInput): Promise<Todo> => {
      const updated = await updateTodo(id, input);
      setTodos((current) => current.map((todo) => (todo.id === updated.id ? updated : todo)));
      setTodosError(null);
      return updated;
    },
    [],
  );

  const [auxDateFilter, setAuxDateFilter] = useState<string | null>(null);
  const [auxCategoryFilter, setAuxCategoryFilter] = useState('');

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
    view,
    setView,
    stats,
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
    addTodoFull: addTodoFullAction,
    editTodo: editTodoAction,
    auxDateFilter,
    setAuxDateFilter,
    auxCategoryFilter,
    setAuxCategoryFilter,
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
    projects,
    projectsLoading,
    projectsError: projectsError,
    addProject: addProjectAction,
    editProject: editProjectAction,
    deleteProject: deleteProjectAction,
    projectFilter,
    openProject: (id: number) => setProjectFilter(id),
    clearProjectFilter: () => setProjectFilter(null),
    links,
    linksLoading,
    linksError: linksError,
    addLink: addLinkAction,
    editLink: editLinkAction,
    deleteLink: deleteLinkAction,
    openLink: openLinkAction,
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
