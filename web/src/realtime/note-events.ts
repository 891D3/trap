import { events, type NoteChangedPayload } from '../mygo';

/** note.changed 事件订阅者；状态层收到后重新读取权威列表。 */
export type NoteChangedHandler = (payload: NoteChangedPayload) => void;

/** 统一订阅 Go 侧信息记录变化事件。 */
export function subscribeNoteChanged(handler: NoteChangedHandler): () => void {
  return events.noteChanged.on(handler);
}
