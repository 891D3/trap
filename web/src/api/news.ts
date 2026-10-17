// news.ts —— 热点资讯 API 封装（超时 + 错误中文化，与其它 api/* 同约定）。
import { NewsService, type NewsItem } from '../mygo';
import { toChineseError } from './app-settings';

export type { NewsItem };

/** 渠道定义（标签页）：key 与 Go 侧 newsPlatforms 一致。 */
export const NEWS_CHANNELS = [
  { key: 'tenxunwang', label: '腾讯网' },
  { key: 'douyin', label: '抖音' },
  { key: 'eastmoney', label: '东方财经' },
  { key: 'xueqiu', label: '雪球' },
] as const;

export type NewsChannelKey = (typeof NEWS_CHANNELS)[number]['key'];

/** 拉取指定渠道热点。force=true 绕过缓存强制刷新（手动刷新按钮）。 */
export async function fetchNews(platform: NewsChannelKey, force = false): Promise<NewsItem[]> {
  try {
    return await NewsService.fetch(platform, force);
  } catch (error: unknown) {
    throw new Error(toChineseError(error, '资讯加载失败，请稍后重试'));
  }
}
