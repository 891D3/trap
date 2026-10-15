import { events, type QuickLinkChangedPayload } from '../mygo';

/** quick_link.changed 事件订阅者；状态层是唯一调用方（AGENTS.md 4.2）。 */
export type QuickLinkChangedHandler = (payload: QuickLinkChangedPayload) => void;

/** 统一订阅 Go 侧快捷入口变化事件。 */
export function subscribeQuickLinkChanged(handler: QuickLinkChangedHandler): () => void {
  return events.quickLinkChanged.on(handler);
}
