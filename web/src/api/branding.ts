import { request } from "./client";
import type { Branding } from "./types";

/** Брендинг и палитра (публичный роут, без авторизации). */
export function getBranding(signal?: AbortSignal): Promise<Branding> {
  return request<Branding>("/branding", { signal });
}
