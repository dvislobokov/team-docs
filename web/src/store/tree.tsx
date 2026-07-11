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
import type { PageTreeNode } from "../api/types";
import { buildTree, type TreeItem } from "../lib/tree";

interface TreeContextValue {
  nodes: PageTreeNode[];
  tree: TreeItem[];
  loading: boolean;
  reload: () => Promise<void>;
  /** Переиндексировать детей родителя в заданном порядке (drag-n-drop). */
  moveTo: (parentId: number | null, orderedIds: number[]) => Promise<void>;
}

const TreeContext = createContext<TreeContextValue | null>(null);

export function TreeProvider({ children }: { children: ReactNode }) {
  const [nodes, setNodes] = useState<PageTreeNode[]>([]);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    try {
      setNodes(await getTree());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const moveTo = useCallback(
    async (parentId: number | null, orderedIds: number[]) => {
      // Позиции присваиваем по порядку; параллельные PATCH’и, затем рефетч.
      await Promise.all(orderedIds.map((id, i) => movePage(id, { parentId, position: i })));
      await reload();
    },
    [reload],
  );

  const tree = useMemo(() => buildTree(nodes), [nodes]);
  const value = useMemo<TreeContextValue>(
    () => ({ nodes, tree, loading, reload, moveTo }),
    [nodes, tree, loading, reload, moveTo],
  );

  return <TreeContext.Provider value={value}>{children}</TreeContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTree(): TreeContextValue {
  const ctx = useContext(TreeContext);
  if (!ctx) throw new Error("useTree must be used within <TreeProvider>");
  return ctx;
}
