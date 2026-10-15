import {
  TodoService,
  ProjectService,
  type Todo as GoTodo,
  type TodoInput as GoTodoInput,
  type TodoFilter,
  type DashboardStats,
} from '../mygo';
import { toChineseError } from './app-settings';

/** 前端待办模型：把 Go 端 omitempty 的可选字段归一化为 null。 */
export type Todo = {
  id: number;
  title: string;
  done: boolean;
  category: string; // '' | personal | work | study（v2.0，个人/工作/学习）
  dueDate: string | null; // UTC RFC3339；null = 未安排
  projectId: number | null;
  completedAt: string | null;
  createdAt: string;
  updatedAt: string;
};

/** Go Todo DTO 转为前端稳定模型。 */
export function normalizeTodo(todo: GoTodo): Todo {
  return {
    id: todo.id,
    title: todo.title,
    done: todo.done,
    category: todo.category ?? '',
    dueDate: todo.dueDate ?? null,
    projectId: todo.projectId ?? null,
    completedAt: todo.completedAt ?? null,
    createdAt: todo.createdAt,
    updatedAt: todo.updatedAt,
  };
}

/** 构造一个「零过滤 = 不过滤」的 TodoFilter（字段必填，见生成的类型）。 */
function emptyFilter(): TodoFilter {
  return {
    Done: null,
    Category: '',
    HasProjectID: false,
    ProjectID: null,
    FilterNoProject: false,
    DueFrom: null,
    DueTo: null,
    NoDue: false,
  };
}

/** 读取得办（filter 缺省 = 全量，v1 行为不变；v2 UI 仍在前端分组）。 */
export async function listTodos(filter?: Partial<TodoFilter>): Promise<Todo[]> {
  try {
    const f = { ...emptyFilter(), ...filter };
    return (await TodoService.list(f)).map(normalizeTodo);
  } catch (error) {
    throw new Error(toChineseError(error, '待办加载失败，请稍后重试'));
  }
}

/** 创建字段（v2.0：可带分类/日期/项目；仅标题时等价 v1 快速添加）。 */
export type CreateTodoInput = GoTodoInput;

/** 新增待办（入参为可编辑字段集合）。 */
export async function createTodo(input: CreateTodoInput): Promise<Todo> {
  try {
    return normalizeTodo(await TodoService.add(input));
  } catch (error) {
    throw new Error(toChineseError(error, '待办添加失败，请稍后重试'));
  }
}

/** 行内编辑态保存：全量更新可编辑字段。 */
export async function updateTodo(id: number, input: CreateTodoInput): Promise<Todo> {
  try {
    return normalizeTodo(await TodoService.update(id, input));
  } catch (error) {
    throw new Error(toChineseError(error, '待办保存失败，请稍后重试'));
  }
}

/** 看板聚合（v2.0）。 */
export async function dashboardStats(): Promise<DashboardStats> {
  try {
    return await TodoService.stats();
  } catch (error) {
    throw new Error(toChineseError(error, '看板数据加载失败，请稍后重试'));
  }
}

/** 修改完成状态。 */
export async function updateTodoDone(id: number, done: boolean): Promise<Todo> {
  try {
    return normalizeTodo(await TodoService.setDone(id, done));
  } catch (error) {
    throw new Error(toChineseError(error, '待办状态保存失败，请稍后重试'));
  }
}

/** 删除待办。 */
export async function removeTodo(id: number): Promise<void> {
  try {
    await TodoService.delete(id);
  } catch (error) {
    throw new Error(toChineseError(error, '待办删除失败，请稍后重试'));
  }
}
