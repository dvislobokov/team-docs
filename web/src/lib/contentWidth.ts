// Ширина колонки контента: 3 пресета, выбор сохраняется в localStorage.
// Подписка через useSyncExternalStore — без провайдера (как в theme.ts).
import { useSyncExternalStore } from "react";

export type ContentWidth = "narrow" | "medium" | "wide";

const KEY = "td-content-width";

export const WIDTH_LABEL: Record<ContentWidth, string> = {
  narrow: "Узкая",
  medium: "Средняя",
  wide: "Широкая",
};

export const WIDTH_CLASS: Record<ContentWidth, string> = {
  narrow: "max-w-[660px]",
  medium: "max-w-[820px]",
  wide: "max-w-[1080px]",
};

function load(): ContentWidth {
  const v = localStorage.getItem(KEY);
  return v === "narrow" || v === "medium" || v === "wide" ? v : "medium";
}

let current: ContentWidth = load();
const listeners = new Set<() => void>();

export function getContentWidth(): ContentWidth {
  return current;
}

export function setContentWidth(w: ContentWidth): void {
  current = w;
  localStorage.setItem(KEY, w);
  listeners.forEach((l) => l());
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function useContentWidth(): ContentWidth {
  return useSyncExternalStore(subscribe, getContentWidth, () => "medium");
}
