import { useEffect, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { getTrash, purgePage, restorePage } from "../api/pages";
import type { TrashItem } from "../api/types";
import { relativeTime } from "../lib/format";
import { useConfirm } from "../store/confirm";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";

interface TrashDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

// Корзина: корни удалённых поддеревьев. Восстановление возвращает всё
// поддерево (если родитель тоже удалён — в корень дерева); окончательное
// удаление безвозвратно. Автоочистка на бэке — 30 дней.
export function TrashDialog({ open, onOpenChange }: TrashDialogProps) {
  const [items, setItems] = useState<TrashItem[] | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);
  const { reload } = useTree();
  const confirm = useConfirm();
  const toast = useToast();

  const load = () => {
    getTrash()
      .then(setItems)
      .catch(() => setItems([]));
  };

  useEffect(() => {
    if (!open) return;
    setItems(null);
    load();
  }, [open]);

  const restore = async (item: TrashItem) => {
    setBusyId(item.id);
    try {
      await restorePage(item.id);
      await reload();
      toast("Страница восстановлена", "success");
      load();
    } finally {
      setBusyId(null);
    }
  };

  const purge = async (item: TrashItem) => {
    const ok = await confirm({
      title: "Удалить безвозвратно?",
      message: `«${item.title || "Без названия"}» и все вложенные страницы будут удалены окончательно.`,
      confirmLabel: "Удалить навсегда",
      danger: true,
    });
    if (!ok) return;
    setBusyId(item.id);
    try {
      await purgePage(item.id);
      toast("Страница удалена окончательно", "success");
      load();
    } finally {
      setBusyId(null);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="anim-overlay fixed inset-0 z-30 bg-ink/20 backdrop-blur-[2px]" />
        <Dialog.Content className="anim-pop fixed left-1/2 top-1/2 z-40 w-[92%] max-w-[480px] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-2xl border border-line bg-card shadow-2xl">
          <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
            <Dialog.Title className="text-[15px] font-600 text-ink">Корзина</Dialog.Title>
            <Dialog.Close className="rounded p-1 text-faint transition hover:bg-line/60 hover:text-ink">
              <X className="h-4 w-4" />
            </Dialog.Close>
          </div>
          <div className="max-h-[60vh] overflow-y-auto p-2">
            {items === null && (
              <div className="px-3 py-4 text-[13px] text-faint">Загрузка…</div>
            )}
            {items?.length === 0 && (
              <div className="px-3 py-4 text-[13px] text-faint">
                Корзина пуста. Удалённые страницы хранятся здесь 30 дней.
              </div>
            )}
            {items?.map((item) => (
              <div
                key={item.id}
                className="group flex items-center gap-3 rounded-lg px-3 py-2.5 hover:bg-line/50"
              >
                <span className="shrink-0 text-[18px] leading-none">{item.icon || "📄"}</span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-[14px] text-ink">
                    {item.title || "Без названия"}
                  </span>
                  <span className="block text-[12px] text-muted">
                    Удалена {relativeTime(item.deletedAt)}
                  </span>
                </span>
                <span className="flex shrink-0 items-center gap-1.5 opacity-0 transition group-hover:opacity-100">
                  <button
                    type="button"
                    onClick={() => restore(item)}
                    disabled={busyId != null}
                    className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body transition hover:border-faint hover:bg-card disabled:opacity-50"
                  >
                    {busyId === item.id ? "…" : "Восстановить"}
                  </button>
                  <button
                    type="button"
                    onClick={() => purge(item)}
                    disabled={busyId != null}
                    className="rounded-md px-2 py-1 text-[12px] text-red-500 transition hover:bg-red-500/10 disabled:opacity-50"
                  >
                    Навсегда
                  </button>
                </span>
              </div>
            ))}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
