// Утилиты для работы с документом BlockNote на клиенте: извлечение простого
// текста (для оценки времени чтения) и заголовков (для оглавления «На этой
// странице»). Работаем по структуре блоков, не поднимая редактор.
import type { PartialBlock } from "@blocknote/core";

type InlineLike = { type?: string; text?: string; content?: unknown };

function inlineText(content: unknown): string {
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";
  return content
    .map((c) => {
      const inline = c as InlineLike;
      if (typeof inline.text === "string") return inline.text;
      if (inline.content) return inlineText(inline.content);
      return "";
    })
    .join("");
}

/** Весь плоский текст документа (рекурсивно по вложенным блокам). */
export function blocksToText(blocks: PartialBlock[] | undefined): string {
  if (!blocks) return "";
  const parts: string[] = [];
  const walk = (list: PartialBlock[]) => {
    for (const b of list) {
      parts.push(inlineText((b as { content?: unknown }).content));
      const children = (b as { children?: PartialBlock[] }).children;
      if (Array.isArray(children)) walk(children);
    }
  };
  walk(blocks);
  return parts.join(" ");
}

export interface Heading {
  id: string;
  text: string;
  level: number;
}

/** Заголовки верхнего уровня документа — для правого оглавления. */
export function extractHeadings(blocks: PartialBlock[] | undefined): Heading[] {
  if (!blocks) return [];
  const out: Heading[] = [];
  for (const b of blocks) {
    if (b.type === "heading") {
      const text = inlineText((b as { content?: unknown }).content).trim();
      if (text) {
        const level = ((b as { props?: { level?: number } }).props?.level ?? 1) as number;
        out.push({ id: String(b.id ?? out.length), text, level });
      }
    }
  }
  return out;
}

/** Пустой ли документ: нет блоков либо только пустые абзацы (страница-контейнер). */
export function isEmptyDoc(blocks: PartialBlock[] | undefined): boolean {
  if (!blocks || blocks.length === 0) return true;
  const onlyEmptyParagraphs = blocks.every((b) => {
    const type = b.type ?? "paragraph";
    const children = (b as { children?: PartialBlock[] }).children;
    return type === "paragraph" && !(Array.isArray(children) && children.length > 0);
  });
  return onlyEmptyParagraphs && blocksToText(blocks).trim() === "";
}

/** Оценка времени чтения в минутах (≈200 слов/мин, минимум 1). */
export function readingMinutes(blocks: PartialBlock[] | undefined): number {
  const words = blocksToText(blocks).trim().split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.round(words / 200));
}
