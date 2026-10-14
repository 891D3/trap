import { NoteService, type Note as GoNote, type NoteSummary as GoNoteSummary } from '../mygo';
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
