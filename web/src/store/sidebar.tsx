// Состояние сайдбара: открыт/закрыт. На десктопе — сворачивание (запоминаем
// выбор), на мобильном — выезжающая панель поверх контента.
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

interface SidebarContextValue {
  open: boolean;
  setOpen: (v: boolean) => void;
  toggle: () => void;
}

const SidebarContext = createContext<SidebarContextValue | null>(null);

const STORAGE_KEY = "td-sidebar-open";
const MOBILE = "(max-width: 767px)";

function initialOpen(): boolean {
  if (window.matchMedia(MOBILE).matches) return false; // на мобильном по умолчанию скрыт
  const saved = localStorage.getItem(STORAGE_KEY);
  return saved == null ? true : saved === "1";
}

export function SidebarProvider({ children }: { children: ReactNode }) {
  const [open, setOpenState] = useState(initialOpen);

  const setOpen = (v: boolean) => {
    setOpenState(v);
    if (!window.matchMedia(MOBILE).matches) localStorage.setItem(STORAGE_KEY, v ? "1" : "0");
  };

  // Реакция на смену ширины экрана (поворот, ресайз).
  useEffect(() => {
    const mq = window.matchMedia(MOBILE);
    const onChange = () => setOpenState(mq.matches ? false : localStorage.getItem(STORAGE_KEY) !== "0");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const value = useMemo<SidebarContextValue>(
    () => ({ open, setOpen, toggle: () => setOpen(!open) }),
    [open],
  );

  return <SidebarContext.Provider value={value}>{children}</SidebarContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useSidebar(): SidebarContextValue {
  const ctx = useContext(SidebarContext);
  if (!ctx) throw new Error("useSidebar must be used within <SidebarProvider>");
  return ctx;
}
