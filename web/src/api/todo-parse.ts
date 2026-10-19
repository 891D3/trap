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

  // 1) 日期：显式「M月D日(号)」优先（复测反馈：「10月17日（本周六）」被周X 规则
  //    抢走误判为下周六）；无显式日期再看相对词/周X。年份取当前（跨年场景罕见，
  //    若结果在过去超过 1 天则视为明年）。
  //    「本周X」= 本周的周X（含今天）；裸「周X」= 下一个周X（同名含今天，
  //    与旧行为一致：nextWeekday 同名归下周，但带「本周」前缀时不跳下周）。
  let weekdayText: string | null = null; // 待剥离的原文（日期或周X）
  const mdMatch = text.match(/(\d{1,2})月(\d{1,2})[日号]/);
  const thisWeekMatch = text.match(/本(?:星期|礼拜|周)([一二三四五六日天])/);
  if (mdMatch) {
    const mon = Number(mdMatch[1]);
    const day = Number(mdMatch[2]);
    if (mon >= 1 && mon <= 12 && day >= 1 && day <= 31) {
      const now = today();
      const d = new Date(now.getFullYear(), mon - 1, day);
      // 已过去超过 1 天 → 视为明年（12 月输入「1月5日」场景）。
      if (d.getTime() < now.getTime() - 86400000) d.setFullYear(d.getFullYear() + 1);
      dayBase = d;
      weekdayText = mdMatch[0];
    }
  } else if (thisWeekMatch) {
    const weekday = WEEKDAY_CODE[thisWeekMatch[1] ?? ''];
    if (weekday !== undefined) {
      // 本周（周一为一周起点）：本周一 = today - ((getDay()+6)%7)，目标 = 本周一 + (weekday-1)。
      // 含今天与已过去的本周X（周六说「本周一」指 5 天前）。
      const now = today();
      const weekStart = shiftDays(now, -((now.getDay() + 6) % 7));
      dayBase = shiftDays(weekStart, (weekday + 6) % 7);
      weekdayText = thisWeekMatch[0];
    }
  } else {
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
      weekdayText = dayMatch[0];
    }
  }
  // 日期词剥离策略（丢字 bug 的根源在此，三轮修复收敛）:
  //   a. 字段值里的日期（如「【活动日期】10月17日（本周六）」）是正文内容，不剥
  //      （否则残留空洞）。
  //   b. 句中日期词（后面还有正文且非时刻短语，如「明天下午，去学校…」）也视为
  //      正文保留——单剥日期词会把句子打断成「下午，去学校…」这种悬空短语。
  //   c. 仅当日期词是「安排前缀」才剥：其后紧邻时刻短语（「明天 15:00 测试解析」
  //      「明天下午2点钟，…」）或后接标点/句尾（「明天，去学校」「明天交报告」）；
  //      剥离时随带消费其后的一个标点，避免残留「，去学校」这种前导标点。
  const PUNCT = '，,。！？；;、!?';
  if (weekdayText) {
    const dayAt = text.indexOf(weekdayText);
    const beforeDay = text.slice(0, dayAt);
    const afterDay = text.slice(dayAt + weekdayText.length);
    // 仅「紧邻」时刻短语才算安排前缀（如「明天 15:00」「明天6:20交」「明天下午3点」）；
    // 时刻在句子更后面（「明天下午，去学校，从6:20开始」）不算，日期词保留在正文。
    const followedByTime = /^\s*(?:\d{1,2}(?::\d{1,2}|点)|(?:上午|早上|凌晨|中午|下午|晚上)\s*\d)/.test(afterDay);
    const atTail = /^\s*$/.test(afterDay) || /^[，,。！？；;、!?\s]/.test(afterDay);
    const inFieldValue = /[：:】]\s*$/.test(beforeDay);
    if (!inFieldValue && (followedByTime || atTail)) {
      let stripEnd = dayAt + weekdayText.length;
      if (PUNCT.includes(text[stripEnd] ?? '')) stripEnd += 1; // 随带消费一个标点
      text = text.slice(0, dayAt) + ' ' + text.slice(stripEnd);
    }
  }

  // 2) 时刻：HH:mm / HH点(半)(钟)? / 上午|下午|晚上 N点(半)(钟)?。
  //    bug #13：不再要求数字前有空白（「下午6点」紧邻也能命中），
  //    并支持上午/下午/晚上的 12 小时制换算（下午6点 → 18:00）。
  //    bug #16（复测反馈）：日期词与句中时刻都被剥掉时标题会出现语义空洞
  //    （「明天下午…从6:20开始」→「下午…从开始」）。因此句中时刻只有在位于
  //    句首/句尾/紧跟日期词时才剥离；位于句子中间（两侧都是正文，如「从
  //    6:20开始」）时只建 chip、不剥正文——chip 已表达该时间，正文保留原文。
  let hh = 9;
  let mm = 0;
  let timeLabel = '';
  let timeConsumed: string | null = null; // 命中的原文（用于剥离，避免误删其它正文）
  let timeMatchMeridiem = ''; // 普通时刻命中的午前/后缀（区分孤立裸时刻与明确短语）

  // 2a) 标签时间「(开始)时间：HH:mm(-HH:mm)?」（如「# 开始时间：14:00」
  //     「# 签到时间：13:30-13:55」）：用户明确意图，优先于裸时刻；
  //     同文多个标签时优先「开始时间」（如签到 13:30 vs 开始 14:00）。
  //     复测反馈：长正文含「开始时间：14:00」未生成日期时间 → 现在识别并剥离该字段。
  // 标签分隔符：冒号或「】」（复测反馈：「【签到时间】13:30—14:00」这种方头括号格式）；
  // 标签名优先级：开始时间 > 活动时间 > 其它时间标签（活动=事项主时间）。
  const labelRe = /(开始|活动)?时间[：:】]\s*(\d{1,2}):(\d{2})(?:[-—–](\d{1,2}):(\d{2}))?/g;
  let labelHit: RegExpExecArray | null = null;
  for (let m = labelRe.exec(text); m !== null; m = labelRe.exec(text)) {
    // 优先级：开始(2) > 活动(1) > 第一个命中(0)。
    const pri = m[1] === '开始' ? 2 : m[1] === '活动' ? 1 : 0;
    const curPri = labelHit === null ? -1 : labelHit[1] === '开始' ? 2 : labelHit[1] === '活动' ? 1 : 0;
    if (pri > curPri) labelHit = m;
  }
  if (labelHit) {
    const lhh = Number(labelHit[2]);
    const lmm = Number(labelHit[3]);
    if (lhh >= 0 && lhh <= 23 && lmm >= 0 && lmm <= 59) {
      hh = lhh;
      mm = lmm;
      timeLabel = `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`;
      timeConsumed = labelHit[0];
      // 复测反馈：正文里「# 开始时间：14:00」是内容的一部分（与「签到时间」等字段
      // 并列，剥离后正文语义残缺），标签时间只建 chip、不剥正文。
    }
  }

  // 2b) 普通时刻（无标签命中时才看，避免「签到时间：13:30」这类其它字段被误当安排）。
  if (!timeConsumed) {
    // 「钟」是时刻词的一部分（「2点钟」）：不匹配会残留悬空「钟」（丢字 bug 第二轮）。
    const timeRe = /(上午|早上|凌晨|中午|下午|晚上)?\s*(?:(\d{1,2}):(\d{1,2})|(\d{1,2})点(半)?(钟)?)/;
    const timeMatch = text.match(timeRe);
    if (timeMatch) {
      timeMatchMeridiem = timeMatch[1] ?? '';
      const meridiem = timeMatchMeridiem;
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
        timeConsumed = timeMatch[0];
        // 句中时刻只建 chip 不剥正文：命中位置两侧都是正文文字（如「从6:20开始」的
        // 「从」「开始」）时，剥离会把句子打断成空洞；句首/句尾/紧跟日期词或标点的
        // 时刻短语（「下午6点」「明天6:20」）是安排意图，仍剥离；剥离时随带消费
        // 其后的一个标点，避免残留「，开会」这种前导标点。
        const at = text.indexOf(timeMatch[0]);
        const before = text.slice(0, at);
        const after = text.slice(at + timeMatch[0].length);
        const atBoundary =
          /^\s*$/.test(before) ||
          /^\s*$/.test(after) ||
          /(?:明天|今天|后天|大后天|下周|本周|[，,。！？；;、!?]|\s)\s*$/.test(before);
        if (atBoundary) {
          let stripEnd = at + timeMatch[0].length;
          if (PUNCT.includes(text[stripEnd] ?? '')) stripEnd += 1; // 随带消费一个标点
          text = text.slice(0, at) + ' ' + text.slice(stripEnd);
        }
      }
    }
  }

  if (dayBase) {
    const dLabel = dayLabel(dayBase);
    return {
      // 剥离后标题为空（如只输入「明天下午2点。」）→ 回退原文，不丢内容也不误报「内容为空」。
      title: normalizeSpaces(text) || normalizeSpaces(raw),
      due: { at: withTime(dayBase, hh, mm), label: timeLabel ? `${dLabel} ${timeLabel}` : dLabel },
    };
  }
  if (timeConsumed && timeLabel) {
    // 标签时间「(开始)时间：HH:mm」是用户明确意图 → 无日期词时默认今天。
    if (labelHit) {
      return { title: normalizeSpaces(text) || normalizeSpaces(raw), due: { at: withTime(today(), hh, mm), label: `今天 ${timeLabel}` } };
    }
    // bug #5 加固：无日期词时，仅「上午/下午/晚上 N点」这类明确时间短语才默认今天；
    // 孤立的「18:00」「6点」（如标题里的「# 开始时间：18:00」）不再被误当作安排。
    if (timeMatchMeridiem) {
      return { title: normalizeSpaces(text) || normalizeSpaces(raw), due: { at: withTime(today(), hh, mm), label: `今天 ${timeLabel}` } };
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

/** 折叠多余空白但保留换行（复测反馈：多行内容在删除确认/列表里挤成一行）。 */
function normalizeSpaces(text: string): string {
  return text
    .split('\n')
    .map((line) => line.replace(/[ \t\f\v]+/g, ' ').trim())
    .filter((line) => line !== '')
    .join('\n');
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
