// Промис-ориентированное подтверждение поверх Radix AlertDialog:
// `const ok = await confirm({ ... })`. Заменяет window.confirm.
import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from "react";
import * as AlertDialog from "@radix-ui/react-alert-dialog";

interface ConfirmOptions {
  title: string;
  message?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

type ConfirmFn = (opts: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [opts, setOpts] = useState<ConfirmOptions | null>(null);
  const resolveRef = useRef<((v: boolean) => void) | null>(null);

  const confirm = useCallback<ConfirmFn>((options) => {
    setOpts(options);
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
    });
  }, []);

  const settle = (v: boolean) => {
    resolveRef.current?.(v);
    resolveRef.current = null;
    setOpts(null);
  };

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <AlertDialog.Root open={opts != null} onOpenChange={(o) => !o && settle(false)}>
        <AlertDialog.Portal>
          <AlertDialog.Overlay className="anim-overlay fixed inset-0 z-40 bg-ink/20 backdrop-blur-[2px]" />
          <AlertDialog.Content className="anim-pop fixed left-1/2 top-1/2 z-50 w-[92%] max-w-[420px] -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-line bg-card p-5 shadow-2xl">
            <AlertDialog.Title className="text-[16px] font-600 text-ink">
              {opts?.title}
            </AlertDialog.Title>
            {opts?.message && (
              <AlertDialog.Description className="mt-2 text-[13.5px] leading-relaxed text-muted">
                {opts.message}
              </AlertDialog.Description>
            )}
            <div className="mt-5 flex justify-end gap-2">
              <AlertDialog.Cancel asChild>
                <button
                  type="button"
                  className="rounded-lg border border-line bg-card px-3.5 py-2 text-[13px] text-body transition hover:border-faint"
                >
                  {opts?.cancelLabel ?? "Отмена"}
                </button>
              </AlertDialog.Cancel>
              <AlertDialog.Action asChild>
                <button
                  type="button"
                  onClick={() => settle(true)}
                  className={
                    "rounded-lg px-3.5 py-2 text-[13px] font-500 text-white transition " +
                    (opts?.danger ? "bg-red-500 hover:bg-red-500/90" : "bg-accent hover:bg-accent/90")
                  }
                >
                  {opts?.confirmLabel ?? "Подтвердить"}
                </button>
              </AlertDialog.Action>
            </div>
          </AlertDialog.Content>
        </AlertDialog.Portal>
      </AlertDialog.Root>
    </ConfirmContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useConfirm(): ConfirmFn {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error("useConfirm must be used within <ConfirmProvider>");
  return ctx;
}
