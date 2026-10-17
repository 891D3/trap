// lunar.ts —— 中国农历换算（今日聚焦日期区显示用）。
// 实现：浏览器/WebView 内置 ICU（Intl.DateTimeFormat 的 chinese 历法），
// 零第三方依赖、数据随系统更新（闰月、大小月、干支年都由 ICU 处理）。
// 已验证：2026-02-17 / 2025-01-29（春节）→ 正月初一；2026-10-17 → 九月初八。
//
// 注：formatToParts 的 month 值形如「九月」「闰六月」（闰月 ICU 直接带「闰」前缀），
// day 为数字（初几取中文序数由本模块转换）。

const CN_NUM = ['一', '二', '三', '四', '五', '六', '七', '八', '九', '十'];
const CN_TEN = { 10: '十', 11: '十一', 12: '十二', 13: '十三', 14: '十四', 15: '十五', 16: '十六', 17: '十七', 18: '十八', 19: '十九' } as const;

/** 农历日数字 → 中文（初二、初十、十四、二十、廿一…三十）。 */
function lunarDayCN(day: number): string {
  if (day === 1) return '初一';
  if (day === 10) return '初十';
  if (day === 20) return '二十';
  if (day === 30) return '三十';
  if (day < 10) return `初${CN_NUM[day - 1] ?? day}`;
  if (day < 20) return CN_TEN[day as keyof typeof CN_TEN] ?? String(day);
  if (day < 30) return `廿${CN_NUM[day - 20 - 1] ?? day - 20}`;
  return String(day);
}

export type LunarInfo = { month: string; day: string };

/**
 * 公历 → 农历。ICU 不可用（极端环境）时返回 null，调用方隐藏农历区不降级报错。
 * @param date 公历日期（默认今天）
 */
export function lunarOf(date: Date = new Date()): LunarInfo | null {
  try {
    const parts = new Intl.DateTimeFormat('zh-CN-u-ca-chinese', {
      month: 'long',
      day: 'numeric',
    }).formatToParts(date);
    const month = parts.find((p) => p.type === 'month')?.value ?? '';
    const dayRaw = parts.find((p) => p.type === 'day')?.value ?? '';
    const dayNum = Number(dayRaw);
    if (!month || !Number.isFinite(dayNum)) return null;
    return { month, day: lunarDayCN(dayNum) };
  } catch {
    return null;
  }
}
