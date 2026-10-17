// todo-parse.ts —— 待办快速创建卡的输入解析（PRD v2.0 §10 拟定决策 2）：
//   时间短语：今天/明天/后天/大后天/周X（一~日，含「星期X」「礼拜X」）+
//   可选「HH:mm」或「HH点(半)?」；只写时间默认今天；无法识别不建 due_date。
//   项目：文本中 `#词` 与项目名精确/前缀匹配则得出 projectId（联想浮层由视图层出）。
// 注：短语解析结果同时给出 chip 文案（如「明天 15:00」），chip 展示与删除在视图层。

export type ParsedDue = { at: Date; label: string };

const WEEKDAY_CODE: Record<string, number> = {
  一: 1, 二: 2, 三: 3, 四: 4, 五: 5, 六: 6, 日: 0, 天: 0,
};

/** 提取时间短语：返回削除短语后的标题与 due（可能为 null = 未安排）。 */
export function extractDue(raw: string): { title: string; due: ParsedDue | null } {
  let text = raw;
  let dayBase: Date | null = null;

  // 1) 日期修饰词（取第一个）。
  const dayMatch =
    text.match(/大后天/) ??
    text.match(/后天/) ??
    text.match(/明天/) ??
    text.match(/今天/) ??
    text.match(/(?:星期|礼拜|周)([一二三四五六日天])/);
  if (dayMatch) {
    if (dayMatch[0] === '大后天') dayBase = shiftDays(today(), 3);
    else if (dayMatch[0] === '后天') dayBase = shiftDays(today(), 2);
    else if (dayMatch[0] === '明天') dayBase = shiftDays(today(), 1);
    else if (dayMatch[0] === '今天') dayBase = today();
    else {
      const weekday = WEEKDAY_CODE[dayMatch[1] ?? ''];
      if (weekday === undefined) return { title: normalizeSpaces(text), due: null };
      dayBase = nextWeekday(today(), weekday);
    }
    text = text.replace(dayMatch[0], ' ');
  }

  // 2) 时刻：HH:mm / HH点(半)? / 上午|下午|晚上 N点(半)?。
  //    bug #13：不再要求数字前有空白（「下午6点」紧邻也能命中），
  //    并支持上午/下午/晚上的 12 小时制换算（下午6点 → 18:00）。
  const timeRe = /(上午|早上|凌晨|中午|下午|晚上)?\s*(?:(\d{1,2}):(\d{1,2})|(\d{1,2})点(半)?)/;
  const timeMatch = text.match(timeRe);
  let hh = 9;
  let mm = 0;
  let timeLabel = '';
  if (timeMatch) {
    const meridiem = timeMatch[1] ?? '';
    const rawHh = Number(timeMatch[2] ?? timeMatch[4]);
    let hhNum = rawHh;
    // 12 小时制换算：下午/晚上 1-11 点 +12；中午/凌晨 12 点分别归 12/0。
    if ((meridiem === '下午' || meridiem === '晚上') && rawHh >= 1 && rawHh <= 11) hhNum = rawHh + 12;
    if (meridiem === '凌晨' && rawHh === 12) hhNum = 0;
    if (hhNum >= 0 && hhNum <= 23) {
      hh = hhNum;
      mm = timeMatch[3] ? Number(timeMatch[3]) : timeMatch[5] ? 30 : 0;
      timeLabel = meridiem
        ? `${meridiem}${rawHh}点${timeMatch[5] ? '半' : ''}`
        : `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`;
      text = text.replace(timeMatch[0], ' ');
    }
  }

  if (dayBase) {
    const dLabel = dayLabel(dayBase);
    return {
      title: normalizeSpaces(text),
      due: { at: withTime(dayBase, hh, mm), label: timeLabel ? `${dLabel} ${timeLabel}` : dLabel },
    };
  }
  if (timeMatch && timeLabel) {
    // bug #5 加固：无日期词时，仅「上午/下午/晚上 N点」这类明确时间短语才默认今天；
    // 孤立的「18:00」「6点」（如标题里的「# 开始时间：18:00」）不再被误当作安排。
    if (timeMatch[1]) {
      return { title: normalizeSpaces(text), due: { at: withTime(today(), hh, mm), label: `今天 ${timeLabel}` } };
    }
    // 裸 HH:mm / N点 → 不安排，把已剥离的时刻还原（避免丢字）。
    return { title: normalizeSpaces(raw), due: null };
  }
  return { title: normalizeSpaces(text), due: null };
}

/** 提取 `#词`：返回剥离 token 的标题与命中的项目 id（无命中/无 token = null）。 */
export function extractProjectToken(
  raw: string,
  projects: { id: number; name: string }[],
): { title: string; projectId: number | null } {
  const found = raw.match(/#([^\s#]+)/);
  if (!found) return { title: normalizeSpaces(raw), projectId: null };
  const name = (found[1] ?? '').toLowerCase();
  const hit =
    projects.find((p) => p.name.toLowerCase().replace(/\s+/g, '') === name) ??
    projects.find((p) => p.name.replace(/\s+/g, '').toLowerCase().startsWith(name) && name.length > 0);
  // bug #5 加固：#token 未命中任何项目时不从标题剥离（如「# 开始时间：18:00」
  // 这类正文引用），避免误删用户内容；仅命中（精确/前缀）才算项目引用。
  if (!hit) return { title: normalizeSpaces(raw), projectId: null };
  return { title: normalizeSpaces(raw.replace(found[0], ' ')), projectId: hit.id };
}

/**
 * `#` 后的查询词（联想浮层用）。bug #1 修复：感知光标位置——光标在某个
 * `#` 所在行的 token 范围内（同一行、位于 # 之后）即返回该 # 后的查询词
 * （到空白/行尾为止），文本中间新起一行输入 # 也能弹浮层；无 # 或光标
 * 在 # 之前返回 null。
 * @param raw 输入全文
 * @param caret 光标偏移（textarea.selectionStart）；不传时退化为旧行为（仅文末）
 */
export function projectQuery(raw: string, caret?: number): string | null {
  if (caret !== undefined) {
    const before = raw.slice(0, caret);
    const lineStart = before.lastIndexOf('\n') + 1;
    const line = before.slice(lineStart);
    const idx = line.lastIndexOf('#');
    if (idx === -1) return null;
    // 查询词：# 后到行尾/空白，不含后续的 #（多 token 场景取光标前最近的一个）。
    const rest = line.slice(idx + 1);
    const end = rest.search(/[\s#]/);
    return end === -1 ? rest : rest.slice(0, end);
  }
  const found = raw.match(/#([^\s#]*)$/);
  return found ? (found[1] ?? '') : null;
}

/** 受控提交前统一走这里：组合字段→ CreateTodoInput 所需值。 */
export function dueToRFC3339(at: Date): string {
  return at.toISOString();
}

function normalizeSpaces(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

function today(): Date {
  const d = new Date();
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function shiftDays(base: Date, offset: number): Date {
  const d = new Date(base.getFullYear(), base.getMonth(), base.getDate());
  d.setDate(d.getDate() + offset);
  return d;
}

function nextWeekday(base: Date, weekday: number): Date {
  const d = new Date(base.getFullYear(), base.getMonth(), base.getDate());
  const delta = (weekday - d.getDay() + 7) % 7 || 7; // 同名 → 下周
  d.setDate(d.getDate() + delta);
  return d;
}

function withTime(base: Date, hh: number, mm: number): Date {
  return new Date(base.getFullYear(), base.getMonth(), base.getDate(), hh, mm, 0, 0);
}

function dayLabel(day: Date): string {
  const diff = Math.round((day.getTime() - today().getTime()) / 86400000);
  if (diff === 0) return '今天';
  if (diff === 1) return '明天';
  if (diff === 2) return '后天';
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  return `${day.getMonth() + 1}/${day.getDate()} ${week[day.getDay()] ?? ''}`.trim();
}
