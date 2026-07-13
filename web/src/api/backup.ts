// Полный экспорт/импорт БД (страницы, ревизии, файлы, диаграммы).
// Экспорт — скачивание .json; импорт — ПОЛНАЯ замена содержимого БД.
import { ApiError } from "./client";

export interface ImportResult {
  status: string;
  pages: number;
  revisions: number;
  files: number;
  diagrams: number;
}

/** Триггерит скачивание дампа БД (имя файла задаёт сервер). */
export function downloadBackup(): void {
  const a = document.createElement("a");
  a.href = "/api/backup/export";
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}

/** Загружает дамп на сервер и ПОЛНОСТЬЮ заменяет содержимое БД. */
export async function importBackup(file: File): Promise<ImportResult> {
  const res = await fetch("/api/backup/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: file,
  });
  if (!res.ok) {
    let message = res.statusText || `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { message?: string };
      if (body?.message) message = body.message;
    } catch {
      // тело не JSON — оставляем statusText
    }
    throw new ApiError(res.status, message);
  }
  return (await res.json()) as ImportResult;
}
