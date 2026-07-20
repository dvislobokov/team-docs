// Обёртки над эндпоинтами страниц/поиска/загрузки (README «API»).
import { request, upload } from "./client";
import type {
  CreatePageRequest,
  FavoriteItem,
  MovePageRequest,
  Page,
  PageTreeNode,
  Revision,
  RevisionDetail,
  SearchHit,
  TemplateItem,
  TrashItem,
  UpdatePageRequest,
  UploadResponse,
} from "./types";

export function getTree(projectId?: number, signal?: AbortSignal): Promise<PageTreeNode[]> {
  const qs = projectId ? `?project=${projectId}` : "";
  return request<PageTreeNode[]>(`/pages/tree${qs}`, { signal });
}

export function getPage(id: number, signal?: AbortSignal): Promise<Page> {
  return request<Page>(`/pages/${id}`, { signal });
}

export function createPage(body: CreatePageRequest): Promise<Page> {
  return request<Page>("/pages", { method: "POST", body });
}

/** Сохранить страницу. Может бросить ConflictError (409). */
export function updatePage(id: number, body: UpdatePageRequest): Promise<Page> {
  return request<Page>(`/pages/${id}`, { method: "PUT", body });
}

export function movePage(id: number, body: MovePageRequest): Promise<void> {
  return request<void>(`/pages/${id}/move`, { method: "PATCH", body });
}

/** Мягкое удаление: страница с поддеревом уходит в корзину. */
export function deletePage(id: number): Promise<void> {
  return request<void>(`/pages/${id}`, { method: "DELETE" });
}

export function getTrash(signal?: AbortSignal): Promise<TrashItem[]> {
  return request<TrashItem[]>("/trash", { signal });
}

export function restorePage(id: number): Promise<void> {
  return request<void>(`/pages/${id}/restore`, { method: "POST" });
}

/** Окончательное удаление из корзины (безвозвратно). */
export function purgePage(id: number): Promise<void> {
  return request<void>(`/pages/${id}/purge`, { method: "DELETE" });
}

export function getRevisions(id: number, signal?: AbortSignal): Promise<Revision[]> {
  return request<Revision[]>(`/pages/${id}/revisions`, { signal });
}

export function getRevision(
  id: number,
  revId: number,
  signal?: AbortSignal,
): Promise<RevisionDetail> {
  return request<RevisionDetail>(`/pages/${id}/revisions/${revId}`, { signal });
}

export function search(q: string, signal?: AbortSignal): Promise<SearchHit[]> {
  if (!q.trim()) return Promise.resolve([]);
  return request<SearchHit[]>(`/search?q=${encodeURIComponent(q)}`, { signal });
}

export function uploadFile(file: File): Promise<UploadResponse> {
  return upload<UploadResponse>("/upload", file);
}

/** Недавно обновлённые страницы по доступным проектам (лента на главной). */
export interface RecentPage {
  id: number;
  title: string;
  icon: string;
  projectId: number;
  updatedAt: string;
  updatedByName: string | null;
}

export function getRecentPages(signal?: AbortSignal): Promise<RecentPage[]> {
  return request<RecentPage[]>("/pages/recent", { signal });
}

/** Избранное текущего пользователя (по всем доступным проектам). */
export function getFavorites(signal?: AbortSignal): Promise<FavoriteItem[]> {
  return request<FavoriteItem[]>("/favorites", { signal });
}

export function addFavorite(pageId: number): Promise<void> {
  return request<void>(`/pages/${pageId}/favorite`, { method: "PUT" });
}

export function removeFavorite(pageId: number): Promise<void> {
  return request<void>(`/pages/${pageId}/favorite`, { method: "DELETE" });
}

/** Шаблоны проекта (редакторам; читателям бэкенд отдаёт пустой список). */
export function getTemplates(projectId?: number, signal?: AbortSignal): Promise<TemplateItem[]> {
  const qs = projectId ? `?project=${projectId}` : "";
  return request<TemplateItem[]>(`/templates${qs}`, { signal });
}
