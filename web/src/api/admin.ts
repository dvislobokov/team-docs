// Админ-API: пользователи и роли (доступно только role=admin).
import { request } from "./client";

export interface AdminUser {
  id: number;
  subject: string;
  username: string;
  name: string;
  email: string;
  role: string; // reader | editor | admin
  lastSeenAt: string;
}

export function listUsers(signal?: AbortSignal): Promise<AdminUser[]> {
  return request<AdminUser[]>("/admin/users", { signal });
}

export function setUserRole(id: number, role: string): Promise<void> {
  return request<void>(`/admin/users/${id}/role`, { method: "PUT", body: { role } });
}
