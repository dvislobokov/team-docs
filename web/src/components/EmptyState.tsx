import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { createPage } from "../api/pages";
import { useTree } from "../store/tree";
import { ImportMarkdown } from "./ImportMarkdown";
import { PasteMarkdown } from "./PasteMarkdown";

// Экран «чистого листа» из макета — показывается, когда страниц ещё нет.
export function EmptyState() {
  const { reload } = useTree();
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);

  const newPage = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const page = await createPage({ parentId: null, title: "Новая страница" });
      await reload();
      navigate(`/pages/${page.id}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto flex max-w-[560px] flex-col items-center px-8 py-32 text-center">
      <div className="mb-6 text-[56px]">🪶</div>
      <h2 className="font-display text-[30px] font-500 tracking-[-0.01em] text-ink">
        Чистый лист
      </h2>
      <p className="mt-3 text-[15.5px] leading-relaxed text-muted">
        Здесь пока ничего нет. Создай первую страницу — или начни печатать,
        документ сохранится сам.
      </p>
      <div className="mt-7 flex items-center gap-3">
        <button
          type="button"
          onClick={newPage}
          disabled={busy}
          className="rounded-lg bg-accent px-4 py-2.5 text-[14px] font-500 text-white transition hover:bg-accent/90 disabled:opacity-60"
        >
          + Новая страница
        </button>
        <ImportMarkdown className="cursor-pointer rounded-lg border border-line bg-card px-4 py-2.5 text-[14px] text-body transition hover:border-faint">
          Импорт из файла
        </ImportMarkdown>
        <PasteMarkdown className="cursor-pointer rounded-lg border border-line bg-card px-4 py-2.5 text-[14px] text-body transition hover:border-faint">
          Вставить Markdown
        </PasteMarkdown>
      </div>
    </div>
  );
}
