// Сборка вложенного дерева из плоского списка (бэк отдаёт flat, отсортированный
// по parent_id NULLS FIRST, position, id — см. GetPageTree).
import type { PageTreeNode } from "../api/types";

export interface TreeItem extends PageTreeNode {
  children: TreeItem[];
}

/** Плоский список → массив корневых узлов с вложенными children. */
export function buildTree(nodes: PageTreeNode[]): TreeItem[] {
  const byId = new Map<number, TreeItem>();
  for (const n of nodes) byId.set(n.id, { ...n, children: [] });

  const roots: TreeItem[] = [];
  for (const n of nodes) {
    const item = byId.get(n.id)!;
    const parent = n.parentId != null ? byId.get(n.parentId) : undefined;
    if (parent) parent.children.push(item);
    else roots.push(item);
  }

  const sortRec = (items: TreeItem[]) => {
    items.sort((a, b) => a.position - b.position || a.id - b.id);
    items.forEach((i) => sortRec(i.children));
  };
  sortRec(roots);
  return roots;
}

/** Цепочка от корня до узла (для хлебных крошек), включая сам узел. */
export function pathToNode(nodes: PageTreeNode[], id: number): PageTreeNode[] {
  const byId = new Map<number, PageTreeNode>();
  for (const n of nodes) byId.set(n.id, n);
  const path: PageTreeNode[] = [];
  let cur = byId.get(id);
  while (cur) {
    path.unshift(cur);
    cur = cur.parentId != null ? byId.get(cur.parentId) : undefined;
  }
  return path;
}

/** Является ли maybeChild потомком ancestorId (для запрета переноса в себя). */
export function isDescendant(nodes: PageTreeNode[], ancestorId: number, maybeChild: number): boolean {
  const byId = new Map<number, PageTreeNode>();
  for (const n of nodes) byId.set(n.id, n);
  let cur = byId.get(maybeChild);
  while (cur) {
    if (cur.parentId === ancestorId) return true;
    cur = cur.parentId != null ? byId.get(cur.parentId) : undefined;
  }
  return false;
}

export type DropMode = "before" | "after" | "inside";

export interface MovePlan {
  parentId: number | null;
  /** Позиция вставки среди детей нового родителя (без учёта самой страницы). */
  position: number;
}

// Планирует перенос dragId относительно targetId. Возвращает нового родителя и
// позицию вставки; null — если перенос недопустим (в себя/в потомка/без
// изменений). Переиндексацию соседей делает сервер атомарно (PATCH move).
export function planMove(
  nodes: PageTreeNode[],
  dragId: number,
  targetId: number,
  mode: DropMode,
): MovePlan | null {
  if (dragId === targetId) return null;
  if (isDescendant(nodes, dragId, targetId)) return null; // нельзя в собственного потомка

  const byId = new Map<number, PageTreeNode>();
  for (const n of nodes) byId.set(n.id, n);
  const target = byId.get(targetId);
  const drag = byId.get(dragId);
  if (!target || !drag) return null;

  const childrenOf = (pid: number | null): number[] =>
    nodes
      .filter((n) => n.parentId === pid)
      .sort((a, b) => a.position - b.position || a.id - b.id)
      .map((n) => n.id);

  let parentId: number | null;
  let siblings: number[];
  let insertAt: number;

  if (mode === "inside") {
    parentId = targetId;
    siblings = childrenOf(parentId).filter((id) => id !== dragId);
    insertAt = siblings.length; // в конец
  } else {
    parentId = target.parentId;
    siblings = childrenOf(parentId).filter((id) => id !== dragId);
    const ti = siblings.indexOf(targetId);
    insertAt = mode === "before" ? ti : ti + 1;
  }

  // Без изменений — если родитель тот же и порядок совпал.
  const orderedIds = [...siblings.slice(0, insertAt), dragId, ...siblings.slice(insertAt)];
  const before = childrenOf(parentId);
  if (drag.parentId === parentId && before.join(",") === orderedIds.join(",")) return null;

  return { parentId, position: insertAt };
}
