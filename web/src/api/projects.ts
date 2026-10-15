// projects.ts：项目 typed IPC 的前端调用封装（v2.0 R1/R3+）。
// 统一带超时与错误中文化；数据形状即 Go 端 DTO（mygo.ts 生成）。

import { ProjectService, type Project } from '../mygo';
export type { Project };
import { toChineseError } from './app-settings';

/** 读全部项目（携带待办计数）。 */
export async function listProjects(): Promise<Project[]> {
  try {
    return await ProjectService.list();
  } catch (error) {
    throw new Error(toChineseError(error, '项目加载失败，请稍后重试'));
  }
}

/** 新增项目（color 缺省由 Go 端作主色处理）。 */
export async function createProject(name: string, color = ''): Promise<Project> {
  try {
    return await ProjectService.add(name, color);
  } catch (error) {
    throw new Error(toChineseError(error, '项目添加失败，请稍后重试'));
  }
}

/** 更新项目（名称/颜色/排序）。 */
export async function updateProject(
  id: number,
  name: string,
  color: string,
  sortOrder: number,
): Promise<Project> {
  try {
    return await ProjectService.update(id, name, color, sortOrder);
  } catch (error) {
    throw new Error(toChineseError(error, '项目保存失败，请稍后重试'));
  }
}

/** 删除项目（关联待办由 Go 端置空；前端另需重新拉取待办）。 */
export async function removeProject(id: number): Promise<void> {
  try {
    await ProjectService.delete(id);
  } catch (error) {
    throw new Error(toChineseError(error, '项目删除失败，请稍后重试'));
  }
}
