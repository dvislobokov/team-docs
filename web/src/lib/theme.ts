// Управление темой: класс `dark` на <html>, сохранение в localStorage и
// уважение системной темы при первом запуске. Логика вынесена из inline-скрипта
// макета (design/mockup.html).
import { useSyncExternalStore } from "react";

export type Theme = "light" | "dark";

const STORAGE_KEY = "td-theme";

function systemPrefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function stored(): Theme | null {
  const v = localStorage.getItem(STORAGE_KEY);
  return v === "light" || v === "dark" ? v : null;
}

/** Текущая тема, исходя из класса на <html>. */
export function currentTheme(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function apply(theme: Theme) {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

/** Применить сохранённую/системную тему до первого рендера (без мигания). */
export function initTheme(): void {
  apply(stored() ?? (systemPrefersDark() ? "dark" : "light"));
}

const listeners = new Set<() => void>();

function notify() {
  listeners.forEach((l) => l());
}

/** Установить конкретную тему и запомнить выбор. */
export function setTheme(theme: Theme): void {
  apply(theme);
  localStorage.setItem(STORAGE_KEY, theme);
  notify();
}

/** Переключить тему и запомнить выбор. */
export function toggleTheme(): void {
  setTheme(currentTheme() === "dark" ? "light" : "dark");
}

// Хук: перерисовывает компонент при смене темы, возвращает "light" | "dark".
function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function useTheme(): Theme {
  return useSyncExternalStore(subscribe, currentTheme, () => "light");
}
