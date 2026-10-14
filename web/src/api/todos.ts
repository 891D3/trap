import { TodoService, type Todo as GoTodo } from '../mygo';
import { toChineseError } from './app-settings';

/** 前端待办模型：把 Go 端 omitempty 的 completedAt 归一化为 null。 */
export type Todo = {
  id: number;
  title: string;
  done: boolean;
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
    completedAt: todo.completedAt ?? null,
    createdAt: todo.createdAt,
    updatedAt: todo.updatedAt,
  };
}

/** 读取全部待办。 */
export async function listTodos(): Promise<Todo[]> {
  try {
    return (await TodoService.list()).map(normalizeTodo);
  } catch (error) {
    throw new Error(toChineseError(error, '待办加载失败，请稍后重试'));
  }
}

/** 新增待办。 */
export async function createTodo(title: string): Promise<Todo> {
  try {
    return normalizeTodo(await TodoService.add(title));
  } catch (error) {
    throw new Error(toChineseError(error, '待办添加失败，请稍后重试'));
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
