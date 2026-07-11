import { request } from "./client";
import type { Me } from "./types";

/** Текущий пользователь. 401 — если авторизация включена, а токена нет/невалиден
 *  (в проде такой запрос не доходит: IAM-прокси перехватывает до приложения). */
export function getMe(signal?: AbortSignal): Promise<Me> {
  return request<Me>("/me", { signal });
}
