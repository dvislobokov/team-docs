// Общий стор дерева страниц: один источник правды для сайдбара, хлебных крошек
// и палитры. Держит плоский список из API и умеет перезагружать его после
// создания/удаления/переименования страниц.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getTree, movePage } from "../api/pages";
import { listProjects, type Project } from "../api/projects";
import type { PageTreeNode } from "../api/types";
import { buildTree, type TreeItem } from "../lib/tree";

const PROJECT_LS_KEY = "td_project";

interface TreeContextValue {
  nodes: PageTreeNode[];
  tree: TreeItem[];
  loading: boolean;
  /** Доступные проекты и текущий (дерево показывает его страницы). */
  projects: Project[];
  project: Project | null;
  setProject: (p: Project) => void;
  /** Перечитать список проектов (после создания/удаления в админке). */
  reloadProjects: () => Promise<void>;
  reload: () => Promise<void>;
  /** Перенести страницу к родителю на позицию (drag-n-drop); соседей переиндексирует сервер. */
  moveTo: (id: number, parentId: number | null, position: number) => Promise<void>;
}

const TreeContext = createContext<TreeContextValue | null>(null);

export function TreeProvider({ children }: { children: ReactNode }) {
  const [nodes, setNodes] = useState<PageTreeNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [projects, setProjects] = useState<Project[]>([]);
  const [project, setProjectState] = useState<Project | null>(null);

  // Текущий проект — из localStorage (fallback: первый доступный).
  const reloadProjects = useCallback(async () => {
    try {
      const list = await listProjects();
      setProjects(list);
      const savedKey = localStorage.getItem(PROJECT_LS_KEY);
      setProjectState((cur) => {
        const wanted = cur?.key ?? savedKey;
        return list.find((p) => p.key === wanted) ?? list[0] ?? null;
      });
    } catch {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reloadProjects();
  }, [reloadProjects]);

  const reload = useCallback(async () => {
    if (!project) return;
    try {
      setNodes(await getTree(project.id));
    } finally {
      setLoading(false);
    }
  }, [project]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const setProject = useCallback((p: Project) => {
    localStorage.setItem(PROJECT_LS_KEY, p.key);
    setProjectState(p);
  }, []);

  const moveTo = useCallback(
    async (id: number, parentId: number | null, position: number) => {
      // Один PATCH: сервер атомарно проверяет цикл и переиндексирует соседей.
      await movePage(id, { parentId, position });
      await reload();
    },
    [reload],
  );

  const tree = useMemo(() => buildTree(nodes), [nodes]);
  const value = useMemo<TreeContextValue>(
    () => ({ nodes, tree, loading, projects, project, setProject, reloadProjects, reload, moveTo }),
    [nodes, tree, loading, projects, project, setProject, reloadProjects, reload, moveTo],
  );

  return <TreeContext.Provider value={value}>{children}</TreeContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTree(): TreeContextValue {
  const ctx = useContext(TreeContext);
  if (!ctx) throw new Error("useTree must be used within <TreeProvider>");
  return ctx;
}
