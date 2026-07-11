// Глобальное состояние командной палитры (поиск ⌘K): открыть можно из сайдбара,
// из хоткея и отовсюду. Держим отдельно, чтобы не тащить пропсы через дерево.
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

interface PaletteContextValue {
  open: boolean;
  setOpen: (v: boolean) => void;
}

const PaletteContext = createContext<PaletteContextValue | null>(null);

export function PaletteProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const value = useMemo(() => ({ open, setOpen }), [open]);
  return <PaletteContext.Provider value={value}>{children}</PaletteContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function usePalette(): PaletteContextValue {
  const ctx = useContext(PaletteContext);
  if (!ctx) throw new Error("usePalette must be used within <PaletteProvider>");
  return ctx;
}
