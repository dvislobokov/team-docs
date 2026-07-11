// Обёртки над эндпоинтами страниц/поиска/загрузки (README «API»).
import { request, upload } from "./client";
import type {
  CreatePageRequest,
  MovePageRequest,
  Page,
  PageTreeNode,
  Revision,
  RevisionDetail,
  SearchHit,
  UpdatePageRequest,
  UploadResponse,
} from "./types";

export function getTree(signal?: AbortSignal): Promise<PageTreeNode[]> {
  return request<PageTreeNode[]>("/pages/tree", { signal });
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

export function deletePage(id: number): Promise<void> {
  return request<void>(`/pages/${id}`, { method: "DELETE" });
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
