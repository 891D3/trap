// links.ts：快捷入口 typed IPC 的前端调用封装（v2.0 R6，替代 T3 的 mock）。
// 统一错误中文化；数据形状为 mygo.ts 生成的 QuickLink DTO。

import { QuickLinkService, type QuickLink } from '../mygo';
import { toChineseError } from './app-settings';

export type { QuickLink };

/** 读全部快捷入口（sort_order 升序）。 */
export async function listLinks(): Promise<QuickLink[]> {
  try {
    return await QuickLinkService.list();
  } catch (error) {
    throw new Error(toChineseError(error, '快捷入口加载失败，请稍后重试'));
  }
}

/** 新增（kind 由 Go 侧按 target 形态判定）。 */
export async function createLink(name: string, target: string, groupName: string): Promise<QuickLink> {
  try {
    return await QuickLinkService.add(name, target, groupName);
  } catch (error) {
    throw new Error(toChineseError(error, '快捷入口添加失败，请稍后重试'));
  }
}

/** 全量更新（重判 kind；sortOrder 保留原值由 store 层传回）。 */
export async function saveLink(
  id: number,
  name: string,
  target: string,
  groupName: string,
  sortOrder: number,
): Promise<QuickLink> {
  try {
    return await QuickLinkService.update(id, name, target, groupName, sortOrder);
  } catch (error) {
    throw new Error(toChineseError(error, '快捷入口保存失败，请稍后重试'));
  }
}

/** 删除。 */
export async function removeLink(id: number): Promise<void> {
  try {
    await QuickLinkService.delete(id);
  } catch (error) {
    throw new Error(toChineseError(error, '快捷入口删除失败，请稍后重试'));
  }
}

/** 用系统默认方式打开（Go 侧白名单命令实现）。 */
export async function openLink(id: number): Promise<void> {
  try {
    await QuickLinkService.open(id);
  } catch (error) {
    throw new Error(toChineseError(error, '打开失败，请检查目标是否有效'));
  }
}
