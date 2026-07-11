import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import * as Dialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { createPageFromMarkdown } from "../lib/pageActions";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";

interface PasteMarkdownProps {
  parentId?: number | null;
  className?: string;
  title?: string;
  children: ReactNode;
}

// Импорт Markdown из вставленного текста: диалог с textarea. Триггер —
// переданные children (кнопка/иконка).
export function PasteMarkdown({ parentId = null, className, title, children }: PasteMarkdownProps) {
  const { reload } = useTree();
  const navigate = useNavigate();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);

  const doImport = async () => {
    if (!text.trim() || busy) return;
    setBusy(true);
    try {
      const page = await createPageFromMarkdown(text, parentId, "Импорт");
      await reload();
      toast(`Импортировано: ${page.title}`, "success");
      setText("");
      setOpen(false);
      navigate(`/pages/${page.id}`);
    } catch (err) {
      toast("Не удалось импортировать: " + (err instanceof Error ? err.message : err), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Trigger asChild>
        <button type="button" className={className} title={title}>
          {children}
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="anim-overlay fixed inset-0 z-40 bg-ink/20 backdrop-blur-[2px]" />
        <Dialog.Content className="anim-pop fixed left-1/2 top-1/2 z-50 flex max-h-[86vh] w-[92%] max-w-[620px] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-2xl border border-line bg-card shadow-2xl">
          <div className="flex items-center justify-between border-b border-line px-5 py-3.5">
            <Dialog.Title className="text-[15px] font-600 text-ink">Импорт из Markdown</Dialog.Title>
            <Dialog.Close className="rounded p-1 text-faint transition hover:bg-line/60 hover:text-ink">
              <X className="h-4 w-4" />
            </Dialog.Close>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-3 p-5">
            <p className="text-[13px] text-muted">
              Вставьте текст в формате Markdown — заголовок возьмётся из первого{" "}
              <span className="font-mono">#</span> либо будет «Импорт».
            </p>
            <textarea
              autoFocus
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={"# Заголовок\n\nТекст, **списки**, `код`, таблицы…"}
              className="scroll h-64 w-full flex-1 resize-none rounded-xl border border-line bg-paper px-3.5 py-3 font-mono text-[13px] leading-relaxed text-ink outline-none placeholder:text-faint focus:border-faint"
            />
          </div>
          <div className="flex justify-end gap-2 border-t border-line px-5 py-3.5">
            <Dialog.Close asChild>
              <button
                type="button"
                className="rounded-lg border border-line bg-card px-3.5 py-2 text-[13px] text-body transition hover:border-faint"
              >
                Отмена
              </button>
            </Dialog.Close>
            <button
              type="button"
              onClick={doImport}
              disabled={busy || !text.trim()}
              className="rounded-lg bg-accent px-3.5 py-2 text-[13px] font-500 text-white transition hover:bg-accent/90 disabled:opacity-50"
            >
              {busy ? "Импорт…" : "Импортировать"}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
