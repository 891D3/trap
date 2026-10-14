import { events, type TodoChangedPayload } from '../mygo';

/** todo.changed 事件订阅者；状态层收到后重新读取权威列表。 */
export type TodoChangedHandler = (payload: TodoChangedPayload) => void;

/** 统一订阅 Go 侧待办变化事件，组件不直接连接 mygo Events。 */
export function subscribeTodoChanged(handler: TodoChangedHandler): () => void {
  return events.todoChanged.on(handler);
}
