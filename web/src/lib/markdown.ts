import { BlockNoteEditor } from "@blocknote/core";
import type { PageContent } from "../api/types";
import { schema } from "./editorSchema";

// Парсинг Markdown → блоки BlockNote через безголовый экземпляр редактора
// (DOM не нужен, конвертация идёт через remark). Схема — общая с редактором,
// чтобы кастомные блоки (callout/mention) корректно (де)сериализовались.
export async function markdownToBlocks(md: string): Promise<PageContent> {
  const editor = BlockNoteEditor.create({ schema });
  const blocks = await editor.tryParseMarkdownToBlocks(md);
  return blocks as PageContent;
}

/** Документ BlockNote → Markdown (lossy) через безголовый редактор. */
export async function blocksToMarkdown(content: PageContent): Promise<string> {
  const editor = BlockNoteEditor.create({ schema });
  return await editor.blocksToMarkdownLossy(content as Parameters<typeof editor.blocksToMarkdownLossy>[0]);
}

/** Заголовок для импортируемой страницы: первый H1 или имя файла без расширения. */
export function deriveTitle(md: string, filename: string): string {
  const h1 = md.match(/^\s*#\s+(.+?)\s*$/m);
  if (h1) return h1[1].trim();
  return filename.replace(/\.(md|markdown|txt)$/i, "").trim() || "Импорт";
}
