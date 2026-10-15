import { events, type ProjectChangedPayload } from '../mygo';

/** project.changed 事件订阅者；状态层是唯一调用方（AGENTS.md 4.2）。 */
export type ProjectChangedHandler = (payload: ProjectChangedPayload) => void;

/** 统一订阅 Go 侧项目变化事件。 */
export function subscribeProjectChanged(handler: ProjectChangedHandler): () => void {
  return events.projectChanged.on(handler);
}
