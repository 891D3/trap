import { AppService, type Settings } from '../mygo';

/** 主题设置的合法取值，与 Go AppService 保持一致。 */
export type ThemeSetting = 'system' | 'light' | 'dark';

/** 前端使用的完整设置模型：归一化了 Go 端的 omitempty 缺省字段。 */
export type AppSettings = {
  name: string;
  avatarPath: string;
  theme: ThemeSetting;
  exportDir: string;
  dataDir: string;
};

/** 首次启动且 SQLite 尚无设置时的展示默认值，不会自动写回数据库。 */
export const DEFAULT_APP_SETTINGS: AppSettings = {
  name: 'Eric',
  avatarPath: '',
  theme: 'system',
  exportDir: '',
  dataDir: '',
};

/**
 * 将 Go typed IPC 的可选字段归一化为前端稳定模型。
 * 非法主题按 system 处理，避免错误数据导致主题副作用崩溃。
 */
export function normalizeSettings(settings: Settings): AppSettings {
  const theme = settings.theme === 'light' || settings.theme === 'dark' ? settings.theme : 'system';
  return {
    name: settings.name?.trim() || DEFAULT_APP_SETTINGS.name,
    avatarPath: settings.avatarPath ?? DEFAULT_APP_SETTINGS.avatarPath,
    theme,
    exportDir: settings.exportDir ?? DEFAULT_APP_SETTINGS.exportDir,
    dataDir: settings.dataDir ?? DEFAULT_APP_SETTINGS.dataDir,
  };
}

/** 将未知异常转换为中文用户提示；Go 端已有中文业务错误时保留原文。 */
export function toChineseError(error: unknown, fallback: string): string {
  const message = error instanceof Error ? error.message.trim() : '';
  return message && /[\u3400-\u9fff]/u.test(message) ? message : fallback;
}

/** 读取应用设置。 */
export async function getAppSettings(): Promise<AppSettings> {
  try {
    return normalizeSettings(await AppService.getSettings());
  } catch (error) {
    throw new Error(toChineseError(error, '设置加载失败，请稍后重试'));
  }
}

/** 保存姓名与头像路径；本轮头像路径只持久化，不处理文件上传。 */
export async function updateUserProfile(name: string, avatarPath: string): Promise<AppSettings> {
  try {
    return normalizeSettings(await AppService.updateProfile(name, avatarPath));
  } catch (error) {
    throw new Error(toChineseError(error, '用户资料保存失败，请稍后重试'));
  }
}

/** 保存主题。 */
export async function setAppTheme(theme: ThemeSetting): Promise<void> {
  try {
    await AppService.setTheme(theme);
  } catch (error) {
    throw new Error(toChineseError(error, '主题保存失败，请稍后重试'));
  }
}

/** 保存导出目录。 */
export async function setExportDirectory(directory: string): Promise<void> {
  try {
    await AppService.setExportDir(directory);
  } catch (error) {
    throw new Error(toChineseError(error, '导出目录保存失败，请稍后重试'));
  }
}
