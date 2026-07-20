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

export interface Group {
  id: number;
  name: string;
  members: number;
}

export function listGroups(): Promise<Group[]> {
  return request<Group[]>("/admin/groups");
}

export function createGroup(name: string): Promise<Group> {
  return request<Group>("/admin/groups", { method: "POST", body: { name } });
}

export function deleteGroup(id: number): Promise<void> {
  return request<void>(`/admin/groups/${id}`, { method: "DELETE" });
}

export function listGroupMembers(id: number): Promise<AdminUser[]> {
  return request<AdminUser[]>(`/admin/groups/${id}/members`);
}

export function addGroupMember(id: number, userId: number): Promise<void> {
  return request<void>(`/admin/groups/${id}/members/${userId}`, { method: "PUT" });
}

export function removeGroupMember(id: number, userId: number): Promise<void> {
  return request<void>(`/admin/groups/${id}/members/${userId}`, { method: "DELETE" });
}

export interface SweepResult {
  trashPurged: number;
  revisionsPruned: number;
  filesRemoved: number;
}

/** Ручной запуск уборки: корзина, старые ревизии, осиротевшие файлы. */
export function runCleanup(): Promise<SweepResult> {
  return request<SweepResult>("/admin/maintenance/cleanup", { method: "POST" });
}

/** Диагностика конфигурации авторизации (JWKS, провайдеры, ключ Apple). */
export function authCheck(): Promise<Record<string, unknown>> {
  return request<Record<string, unknown>>("/admin/auth/check");
}

/** Секция действующих настроек авторизации (read-only, секреты замаскированы). */
export interface AuthSettingsSection {
  title: string;
  items: { label: string; value: string }[];
}

export function authSettings(signal?: AbortSignal): Promise<AuthSettingsSection[]> {
  return request<AuthSettingsSection[]>("/admin/auth/settings", { signal });
}

/** Настройка приложения: значение + источник (env/yaml заблокированы). */
export interface Setting {
  key: string;
  label: string;
  kind: "string" | "int";
  value: string | number;
  source: "env" | "yaml" | "db" | "default";
  editable: boolean;
}

export function listSettings(signal?: AbortSignal): Promise<Setting[]> {
  return request<Setting[]>("/admin/settings", { signal });
}

export function saveSetting(key: string, value: string | number): Promise<void> {
  return request<void>("/admin/settings", { method: "PUT", body: { key, value } });
}
