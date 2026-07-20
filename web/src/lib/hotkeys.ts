// Горячие клавиши приложения. Матчинг по e.code (физическая клавиша), чтобы
// работало и в русской раскладке; Ctrl на Windows/Linux ≡ Cmd на macOS.
// Два слоя: лёгкие Ctrl+<key> — частые действия, Ctrl+Alt+<key> — команды.
// Alt-слой не срабатывает при фокусе в полях ввода и редакторе: на Windows
// AltGr = Ctrl+Alt, и в типографских раскладках эти сочетания печатают символы.
import { useEffect, useRef } from "react";

export interface Hotkey {
  code: string; // KeyboardEvent.code, например "KeyK"
  alt?: boolean;
  shift?: boolean;
  /** Срабатывать и при фокусе в input/textarea/contenteditable. */
  allowInEditable?: boolean;
}

function isEditableTarget(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement)) return false;
  return t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName);
}

export function hotkeyMatches(e: KeyboardEvent, h: Hotkey): boolean {
  return (
    e.code === h.code &&
    (e.metaKey || e.ctrlKey) &&
    e.altKey === !!h.alt &&
    e.shiftKey === !!h.shift &&
    (!!h.allowInEditable || !isEditableTarget(e.target))
  );
}

/** Глобальный хоткей: preventDefault + handler. Хендлер можно не мемоизировать. */
export function useHotkey(h: Hotkey, handler: () => void): void {
  const ref = useRef(handler);
  ref.current = handler;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!hotkeyMatches(e, h)) return;
      e.preventDefault();
      ref.current();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [h.code, h.alt, h.shift, h.allowInEditable]);
}

const isMac = /Mac|iPhone|iPad/.test(navigator.platform);

/** Подпись сочетания для UI: Ctrl+Alt+N / ⌘⌥N. */
export function combo(key: string, opts?: { alt?: boolean; shift?: boolean }): string {
  const parts = [isMac ? "⌘" : "Ctrl"];
  if (opts?.alt) parts.push(isMac ? "⌥" : "Alt");
  if (opts?.shift) parts.push(isMac ? "⇧" : "Shift");
  parts.push(key);
  return parts.join(isMac ? "" : "+");
}

/** Таблица для справки (Ctrl+/). Единственное место с полным списком. */
export const HOTKEYS_HELP: { keys: string; label: string }[] = [
  { keys: combo("K"), label: "Поиск и команды" },
  { keys: combo("E"), label: "Редактировать / готово" },
  { keys: combo("S"), label: "Сохранить страницу" },
  { keys: combo("N", { alt: true }), label: "Новая страница" },
  { keys: combo("S", { alt: true }), label: "В избранное / из избранного" },
  { keys: combo("D", { alt: true }), label: "Создать страницу из шаблона" },
  { keys: combo("P", { alt: true }), label: "Экспорт страницы в Markdown" },
  { keys: combo("T", { alt: true }), label: "Свернуть / развернуть сайдбар" },
  { keys: combo("A", { alt: true }), label: "Администрирование" },
  { keys: combo("/"), label: "Эта справка" },
];
