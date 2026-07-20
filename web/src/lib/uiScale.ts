// Масштаб интерфейса: 3 пресета, выбор сохраняется в localStorage.
// Применяется через CSS zoom на <html> — масштабирует и текст, и отступы
// (вёрстка на px-значениях, поэтому rem-подход не сработал бы).
// Подписка через useSyncExternalStore — без провайдера (как в theme.ts).
import { useSyncExternalStore } from "react";

export type UIScale = "small" | "medium" | "large";

const KEY = "td-ui-scale";

export const SCALE_LABEL: Record<UIScale, string> = {
  small: "Мелкий",
  medium: "Средний",
  large: "Крупный",
};

const SCALE_ZOOM: Record<UIScale, string> = {
  small: "0.9",
  medium: "1",
  large: "1.15",
};

function load(): UIScale {
  const v = localStorage.getItem(KEY);
  return v === "small" || v === "medium" || v === "large" ? v : "medium";
}

function apply(s: UIScale): void {
  const el = document.documentElement;
  el.style.zoom = SCALE_ZOOM[s];
  // 100vh внутри zoom масштабируется вместе со всем остальным: при >1 появляется
  // прокрутка, при <1 — зазор под футером. Полноэкранные контейнеры используют
  // высоту calc(100vh / var(--ui-zoom)) — она в точности равна вьюпорту.
  el.style.setProperty("--ui-zoom", SCALE_ZOOM[s]);
}

let current: UIScale = load();
apply(current);

const listeners = new Set<() => void>();

export function getUIScale(): UIScale {
  return current;
}

export function setUIScale(s: UIScale): void {
  current = s;
  localStorage.setItem(KEY, s);
  apply(s);
  listeners.forEach((l) => l());
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function useUIScale(): UIScale {
  return useSyncExternalStore(subscribe, getUIScale, () => "medium");
}
