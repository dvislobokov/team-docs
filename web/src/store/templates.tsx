// Стор шаблонов текущего проекта: секция в сайдбаре + обновление после
// переименования/удаления шаблона на его странице.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getTemplates } from "../api/pages";
import type { TemplateItem } from "../api/types";
import { useTree } from "./tree";

interface TemplatesContextValue {
  templates: TemplateItem[];
  reload: () => Promise<void>;
}

const TemplatesContext = createContext<TemplatesContextValue | null>(null);

export function TemplatesProvider({ children }: { children: ReactNode }) {
  const { project } = useTree();
  const [templates, setTemplates] = useState<TemplateItem[]>([]);

  const reload = useCallback(async () => {
    if (!project) return;
    try {
      setTemplates(await getTemplates(project.id));
    } catch {
      setTemplates([]);
    }
  }, [project]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const value = useMemo<TemplatesContextValue>(
    () => ({ templates, reload }),
    [templates, reload],
  );

  return <TemplatesContext.Provider value={value}>{children}</TemplatesContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTemplates(): TemplatesContextValue {
  const ctx = useContext(TemplatesContext);
  if (!ctx) throw new Error("useTemplates must be used within <TemplatesProvider>");
  return ctx;
}
