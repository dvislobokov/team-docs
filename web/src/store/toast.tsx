// Простая система тостов: провайдер держит очередь, useToast() добавляет.
// Автоскрытие через таймер; клик закрывает вручную.
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { Check, Info, X } from "lucide-react";

type ToastKind = "success" | "error" | "info";

interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
  leaving?: boolean;
}

interface ToastContextValue {
  toast: (message: string, kind?: ToastKind) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const seq = useRef(0);

  // Помечаем «уходящим» → проигрываем exit-анимацию → удаляем.
  const dismiss = useCallback((id: number) => {
    setToasts((cur) => cur.map((t) => (t.id === id ? { ...t, leaving: true } : t)));
    setTimeout(() => setToasts((cur) => cur.filter((t) => t.id !== id)), 170);
  }, []);

  const toast = useCallback(
    (message: string, kind: ToastKind = "info") => {
      const id = ++seq.current;
      setToasts((cur) => [...cur, { id, kind, message }]);
      setTimeout(() => dismiss(id), 3400);
    },
    [dismiss],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed bottom-5 right-5 z-50 flex w-[320px] max-w-[calc(100vw-2rem)] flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={
              "pointer-events-auto flex items-start gap-2.5 rounded-xl border border-line bg-card px-3.5 py-3 shadow-xl " +
              (t.leaving ? "anim-toast-out" : "anim-toast-in")
            }
          >
            <span className="mt-0.5 shrink-0">
              {t.kind === "success" ? (
                <Check className="h-4 w-4 text-accent" />
              ) : t.kind === "error" ? (
                <X className="h-4 w-4 text-red-500" />
              ) : (
                <Info className="h-4 w-4 text-muted" />
              )}
            </span>
            <span className="flex-1 text-[13.5px] leading-snug text-body">{t.message}</span>
            <button
              type="button"
              onClick={() => dismiss(t.id)}
              className="shrink-0 rounded p-0.5 text-faint transition hover:bg-line/60 hover:text-ink"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast(): (message: string, kind?: ToastKind) => void {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within <ToastProvider>");
  return ctx.toast;
}
