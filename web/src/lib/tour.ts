// Приветственный тур: показываем при первом визите (флаг в localStorage) и по
// запросу (событие td:tour — из меню оформления).
export const TOUR_KEY = "td-tour-done";

export function isTourDone(): boolean {
  return localStorage.getItem(TOUR_KEY) === "1";
}

export function markTourDone(): void {
  localStorage.setItem(TOUR_KEY, "1");
}

/** Запустить тур принудительно (перезапуск из меню). */
export function startTour(): void {
  window.dispatchEvent(new CustomEvent("td:tour"));
}
