// Действия над страницей, переиспользуемые из дерева и топбара. Все они читают
// текущую страницу (нужны content/version/icon для PUT) и работают через
// существующие эндпоинты — отдельных ручек на бэке не требуется.
import { createPage, getPage, updatePage } from "../api/pages";
import type { Page } from "../api/types";
import { blocksToMarkdown, deriveTitle, markdownToBlocks } from "./markdown";

/** Создать страницу из Markdown (файл или вставленный текст). Заголовок —
 *  первый H1 либо fallback. Возвращает сохранённую страницу. */
export async function createPageFromMarkdown(
  md: string,
  parentId: number | null,
  fallbackTitle: string,
): Promise<Page> {
  const blocks = await markdownToBlocks(md);
  const title = deriveTitle(md, fallbackTitle);
  const page = await createPage({ parentId, title });
  return updatePage(page.id, { title, icon: page.icon, content: blocks, version: page.version });
}

/** Переименовать: тянем страницу и сохраняем с новым заголовком. */
export async function renamePage(id: number, title: string): Promise<Page> {
  const p = await getPage(id);
  return updatePage(id, { title, icon: p.icon, content: p.content, version: p.version });
}

/** Дублировать (одну страницу, без поддерева) рядом с оригиналом. */
export async function duplicatePage(id: number): Promise<Page> {
  const p = await getPage(id);
  const copy = await createPage({
    parentId: p.parentId,
    title: `${p.title || "Без названия"} (копия)`,
  });
  return updatePage(copy.id, {
    title: copy.title,
    icon: p.icon,
    content: p.content,
    version: copy.version,
  });
}

/** Скачать страницу как .md. */
export async function exportPageMarkdown(id: number): Promise<void> {
  const p = await getPage(id);
  const md = await blocksToMarkdown(p.content);
  const heading = p.icon ? `${p.icon} ${p.title}` : p.title;
  const body = `# ${heading || "Без названия"}\n\n${md}`;
  const safe = (p.title || "page").replace(/[\\/:*?"<>|]+/g, "_").slice(0, 80);
  downloadText(`${safe}.md`, body);
}

function downloadText(filename: string, text: string) {
  const blob = new Blob([text], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
