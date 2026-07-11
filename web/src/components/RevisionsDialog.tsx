import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { getRevisions } from "../api/pages";
import type { Revision } from "../api/types";
import { relativeTime } from "../lib/format";

interface RevisionsDialogProps {
  pageId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Откатить страницу к версии (сохранит её контент как новую версию). */
  onRestore: (revId: number) => Promise<void>;
}

// История версий: снапшоты (не чаще раза в 2 мин на странице, см. бэкенд).
export function RevisionsDialog({ pageId, open, onOpenChange, onRestore }: RevisionsDialogProps) {
  const [revisions, setRevisions] = useState<Revision[] | null>(null);
  const [restoringId, setRestoringId] = useState<number | null>(null);

  const restore = async (revId: number) => {
    setRestoringId(revId);
    try {
      await onRestore(revId);
    } finally {
      setRestoringId(null);
    }
  };

  useEffect(() => {
    if (!open) return;
    const ctrl = new AbortController();
    setRevisions(null);
    getRevisions(pageId, ctrl.signal)
      .then(setRevisions)
      .catch(() => setRevisions([]));
    return () => ctrl.abort();
  }, [open, pageId]);

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="anim-overlay fixed inset-0 z-30 bg-ink/20 backdrop-blur-[2px]" />
        <Dialog.Content className="anim-pop fixed left-1/2 top-1/2 z-40 w-[92%] max-w-[440px] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-2xl border border-line bg-card shadow-2xl">
          <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
            <Dialog.Title className="text-[15px] font-600 text-ink">
              История версий
            </Dialog.Title>
            <Dialog.Close className="rounded p-1 text-faint transition hover:bg-line/60 hover:text-ink">
              <X className="h-4 w-4" />
            </Dialog.Close>
          </div>
          <div className="max-h-[60vh] overflow-y-auto p-2">
            {revisions === null && (
              <div className="px-3 py-4 text-[13px] text-faint">Загрузка…</div>
            )}
            {revisions?.length === 0 && (
              <div className="px-3 py-4 text-[13px] text-faint">Пока нет сохранённых версий</div>
            )}
            {revisions?.map((r) => (
              <div
                key={r.id}
                className="group flex items-center gap-3 rounded-lg px-3 py-2.5 hover:bg-line/50"
              >
                <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-accentSoft font-mono text-[11px] text-accent">
                  v{r.version}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14px] text-ink">
                    {r.title || "Без названия"}
                  </span>
                  <span className="block text-[12px] text-muted">
                    {relativeTime(r.createdAt)}
                  </span>
                </span>
                <button
                  type="button"
                  onClick={() => restore(r.id)}
                  disabled={restoringId != null}
                  className="shrink-0 rounded-md border border-line px-2.5 py-1 text-[12px] text-body opacity-0 transition hover:border-faint hover:bg-card group-hover:opacity-100 disabled:opacity-50"
                >
                  {restoringId === r.id ? "…" : "Восстановить"}
                </button>
              </div>
            ))}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
