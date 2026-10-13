import { events } from '../mygo';
import { normalizeSettings, type AppSettings } from '../api/app-settings';

/** 设置变化订阅者；状态层是唯一调用方，组件不直接连接 mygo Events。 */
export type SettingsChangedHandler = (settings: AppSettings) => void;

/**
 * 订阅 Go 侧 settings.changed 事件。
 * mygo-runtime 返回的清理函数由 AppStoreProvider 在卸载时调用。
 */
export function subscribeSettingsChanged(handler: SettingsChangedHandler): () => void {
  return events.settingsChanged.on((settings) => {
    handler(normalizeSettings(settings));
  });
}
