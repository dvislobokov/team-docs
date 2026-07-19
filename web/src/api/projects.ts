// Проекты (пространства) и участники (§10).
import { request } from "./client";

export interface Project {
  id: number;
  key: string;
  name: string;
  icon: string;
  visibility: "public" | "internal" | "private";
  myRole: "reader" | "editor" | "admin";
  createdAt: string;
}

export interface ProjectMember {
  userId: number;
  role: string;
  name: string;
  username: string;
  email: string;
}

export function listProjects(signal?: AbortSignal): Promise<Project[]> {
  return request<Project[]>("/projects", { signal });
}

export function createProject(body: {
  key: string;
  name: string;
  icon?: string;
  visibility?: string;
}): Promise<Project> {
  return request<Project>("/projects", { method: "POST", body });
}

export function updateProject(
  id: number,
  body: { name?: string; icon?: string; visibility?: string },
): Promise<Project> {
  return request<Project>(`/projects/${id}`, { method: "PUT", body });
}

export function deleteProject(id: number): Promise<void> {
  return request<void>(`/projects/${id}`, { method: "DELETE" });
}

export function listMembers(id: number): Promise<ProjectMember[]> {
  return request<ProjectMember[]>(`/projects/${id}/members`);
}

export function setMember(id: number, userId: number, role: string): Promise<void> {
  return request<void>(`/projects/${id}/members/${userId}`, { method: "PUT", body: { role } });
}

export function removeMember(id: number, userId: number): Promise<void> {
  return request<void>(`/projects/${id}/members/${userId}`, { method: "DELETE" });
}
