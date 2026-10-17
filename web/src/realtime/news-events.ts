// news-events.ts —— 订阅 Go 侧 news.changed 事件（热点资讯定时/手动刷新广播）。
// 与 settings-events 同构：状态层/组件在挂载时订阅，卸载时调用清理函数。
import { events } from '../mygo';
import type { NewsChangedPayload } from '../mygo';

/** 新闻刷新订阅者：payload = { platform, count }。 */
export type NewsChangedHandler = (payload: NewsChangedPayload) => void;

/** 订阅 news.changed 事件；返回清理函数。 */
export function subscribeNewsChanged(handler: NewsChangedHandler): () => void {
  return events.newsChanged.on((payload) => {
    handler(payload);
  });
}
