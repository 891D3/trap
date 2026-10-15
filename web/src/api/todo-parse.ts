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

  // 2) 时刻：HH:mm / HH点(半)?；无日期但有时刻 → 今天。
  const timeMatch = text.match(/(?:\s|^)(?:(\d{1,2}):(\d{1,2})|(\d{1,2})点(半)?)/);
  let hh = 9;
  let mm = 0;
  let timeLabel = '';
  if (timeMatch) {
    const hhNum = Number(timeMatch[1] ?? timeMatch[3]);
    if (hhNum >= 0 && hhNum <= 23) {
      hh = hhNum;
      mm = timeMatch[2] ? Number(timeMatch[2]) : timeMatch[4] ? 30 : 0;
      timeLabel = `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`;
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
    return { title: normalizeSpaces(text), due: { at: withTime(today(), hh, mm), label: `今天 ${timeLabel}` } };
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
  return { title: normalizeSpaces(raw.replace(found[0], ' ')), projectId: hit?.id ?? null };
}

/** `#` 后的查询词（联想浮层用）。 */
export function projectQuery(raw: string): string | null {
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
