// Тонкая типизированная обёртка над fetch для /api. В dev запросы проксирует
// Vite (см. vite.config.ts) на Go-бэкенд :8080, в prod бинарь отдаёт всё сам.

const BASE = "/api";

/** Ошибка API: несёт HTTP-статус и сообщение из тела {"message": "..."}. */
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** 409 — конфликт оптимистичной блокировки при PUT /api/pages/:id. */
export class ConflictError extends ApiError {
  constructor(message: string) {
    super(409, message);
    this.name = "ConflictError";
  }
}

async function toError(res: Response): Promise<ApiError> {
  let message = res.statusText || `HTTP ${res.status}`;
  try {
    const body = (await res.json()) as { message?: string };
    if (body?.message) message = body.message;
  } catch {
    // тело не JSON — оставляем statusText
  }
  return res.status === 409 ? new ConflictError(message) : new ApiError(res.status, message);
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
}

/** JSON-запрос. Возвращает распарсенное тело; для 204 — undefined. */
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, signal } = opts;
  const res = await fetch(BASE + path, {
    method,
    signal,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw await toError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

/** Загрузка файла через multipart/form-data (поле `file`). */
export async function upload<T>(path: string, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  const res = await fetch(BASE + path, { method: "POST", body: form });
  if (!res.ok) throw await toError(res);
  return (await res.json()) as T;
}
