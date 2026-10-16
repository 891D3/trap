import {
  NoteService,
  type PromoteSparkResult as GoPromoteResult,
  type Note as GoNote,
  type NoteSummary as GoNoteSummary,
  type NoteV2 as GoNoteV2,
  type NoteSummaryV2 as GoNoteSummaryV2,
  type PromoteSparkResult,
} from '../mygo';

/** 前端 v2 归一模型（null 兼容 Go 端 omitempty 可选字段）。 */
export type NoteV2 = Omit<GoNoteV2, 'archivedAt' | 'projectId'> & {
  archivedAt: string | null;
  projectId: number | null;
};
export type NoteSummaryV2 = Omit<GoNoteSummaryV2, 'archivedAt' | 'projectId'> & {
  archivedAt: string | null;
  projectId: number | null;
};
import { toChineseError } from './app-settings';

export type Note = {
  id: number;
  title: string;
  content: string;
  archived: boolean;
  createdAt: string;
  updatedAt: string;
};

export type NoteSummary = Omit<Note, 'content'>;

export type ExportResult = {
  files: string[];
};

export function normalizeNote(note: GoNote): Note {
  return {
    id: note.id,
    title: note.title,
    content: note.content,
    archived: note.archived,
    createdAt: note.createdAt,
    updatedAt: note.updatedAt,
  };
}

export function normalizeNoteSummary(note: GoNoteSummary): NoteSummary {
  return {
    id: note.id,
    title: note.title,
    archived: note.archived,
    createdAt: note.createdAt,
    updatedAt: note.updatedAt,
  };
}

export async function listNotes(includeArchived: boolean): Promise<NoteSummary[]> {
  try {
    return (await NoteService.list(includeArchived)).map(normalizeNoteSummary);
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录加载失败，请稍后重试'));
  }
}

export async function getNote(id: number): Promise<Note> {
  try {
    return normalizeNote(await NoteService.get(id));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录读取失败，请稍后重试'));
  }
}

export async function createNote(title: string): Promise<Note> {
  try {
    return normalizeNote(await NoteService.create(title));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录创建失败，请稍后重试'));
  }
}

export async function saveNote(id: number, title: string, content: string): Promise<Note> {
  try {
    return normalizeNote(await NoteService.save(id, title, content));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录保存失败，请稍后重试'));
  }
}

export async function setNoteArchived(id: number, archived: boolean): Promise<Note> {
  try {
    return normalizeNote(await NoteService.setArchived(id, archived));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录归档失败，请稍后重试'));
  }
}

export async function deleteNote(id: number): Promise<void> {
  try {
    await NoteService.delete(id);
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录删除失败，请稍后重试'));
  }
}

export async function exportNotes(ids: number[]): Promise<ExportResult> {
  try {
    return await NoteService.export(ids);
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录导出失败，请稍后重试'));
  }
}

/* ---------- 信息记录 v2（M0.5：灵感速记 × 笔记文档） ---------- */

export type SparkPromoteResult = {
  note: NoteV2 | null;
  todo: { id: number; title?: string } | null;
};

/** getNoteV2 按 id 读取 v2 实体（编辑态）。 */
export async function getNoteV2(id: number): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.getNoteV2(id));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录读取失败，请稍后重试'));
  }
}


function toEntityV2(n: GoNoteV2): NoteV2 {
  return { ...n, archivedAt: n.archivedAt ?? null, projectId: n.projectId ?? null };
}
/** 归一化 v2 摘要（projectId 归一为 null）。 */
function toSummaryV2(n: GoNoteSummaryV2): NoteSummaryV2 {
  return { ...n, archivedAt: n.archivedAt ?? null, projectId: n.projectId ?? null };
}

/** 保存 v2 全量字段（标题/正文/项目关联）。 */
export async function saveNoteV2(
  id: number,
  title: string,
  content: string,
  projectId: number | null,
): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.saveNoteV2(id, title, content, projectId));
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录保存失败，请稍后重试'));
  }
}

/** 更新项目关联（chip 选择浮层保存）。 */
export async function linkNoteProject(id: number, projectId: number | null): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.linkProject(id, projectId));
  } catch (error) {
    throw new Error(toChineseError(error, '项目关联保存失败，请稍后重试'));
  }
}

/** 新建灵感（content 纯文本）。 */
export async function createSpark(content: string, projectId: number | null): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.createSpark(content, projectId));
  } catch (error) {
    throw new Error(toChineseError(error, '灵感保存失败，请稍后重试'));
  }
}

/** 灵感流（includeArchived=false → 未处理；true → 归档）。 */
export async function listSparks(includeArchived: boolean): Promise<NoteSummaryV2[]> {
  try {
    return (await NoteService.listSparks(includeArchived)).map(toSummaryV2);
  } catch (error) {
    throw new Error(toChineseError(error, '灵感加载失败，请稍后重试'));
  }
}

/** 笔记列表（v2 排除灵感；项目/标签过滤）。 */
export async function listNotesV2(projectId: number | null, tag: string): Promise<NoteSummaryV2[]> {
  try {
    return (await NoteService.listNotesV2(projectId, tag)).map(toSummaryV2);
  } catch (error) {
    throw new Error(toChineseError(error, '信息记录加载失败，请稍后重试'));
  }
}

/** 转笔记（保留原灵感）/ 转待办。 */
export async function promoteSpark(sparkID: number, as: 'note' | 'todo'): Promise<GoPromoteResult> {
  try {
    return await NoteService.promoteSpark(sparkID, as);
  } catch (error) {
    throw new Error(toChineseError(error, as === 'note' ? '转笔记失败，请稍后重试' : '转待办失败，请稍后重试'));
  }
}

/** 灵感归档 / 恢复。 */
export async function archiveSpark(id: number): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.archiveSpark(id));
  } catch (error) {
    throw new Error(toChineseError(error, '灵感归档失败，请稍后重试'));
  }
}

export async function restoreSpark(id: number): Promise<NoteV2> {
  try {
    return toEntityV2(await NoteService.restoreSpark(id));
  } catch (error) {
    throw new Error(toChineseError(error, '灵感恢复失败，请稍后重试'));
  }
}

/** 自动归档清扫（进入灵感 Tab 触发；启动时由 Go 端自行执行）。 */
export async function sweepSparks(): Promise<void> {
  try {
    await NoteService.sweepSparks();
  } catch (error) {
    // sweep 失败不阻塞 UI（下次进入再触发）；仅记录。
    console.error('灵感清扫失败', error);
  }
}

/** 复制为富文本（剪贴板 text/html + text/plain）。 */
export async function copyNoteRichText(id: number): Promise<void> {
  try {
    await NoteService.copyRichText(id);
  } catch (error) {
    throw new Error(toChineseError(error, '复制失败，请稍后重试'));
  }
}
