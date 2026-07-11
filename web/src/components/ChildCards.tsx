import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { createPage } from "../api/pages";
import type { PageTreeNode } from "../api/types";
import { useTree } from "../store/tree";

interface ChildCardsProps {
  /** Дочерние страницы текущей страницы (уже отсортированы). */
  nodes: PageTreeNode[];
  /** Весь плоский список — чтобы посчитать вложенность у каждой карточки. */
  allNodes: PageTreeNode[];
  parentId: number;
}

function childCountLabel(n: number): string {
  if (n === 0) return "";
  const mod10 = n % 10;
  const mod100 = n % 100;
  const word =
    mod10 === 1 && mod100 !== 11
      ? "вложенная страница"
      : mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)
        ? "вложенные страницы"
        : "вложенных страниц";
  return `${n} ${word}`;
}

// Галерея дочерних страниц — для страниц-контейнеров (по сценарию: пустая
// страница, в которую складывают другие). Показывается и как навигация.
export function ChildCards({ nodes, allNodes, parentId }: ChildCardsProps) {
  const navigate = useNavigate();
  const { reload } = useTree();
  const [busy, setBusy] = useState(false);

  const count = (id: number) => allNodes.filter((c) => c.parentId === id).length;

  const add = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const page = await createPage({ parentId, title: "Новая страница" });
      await reload();
      navigate(`/pages/${page.id}`, { state: { isNew: true } });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <div className="mb-3 font-mono text-[11px] uppercase tracking-[0.08em] text-faint">
        Вложенные страницы
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {nodes.map((n) => (
          <button
            key={n.id}
            type="button"
            onClick={() => navigate(`/pages/${n.id}`)}
            className="flex items-start gap-3 rounded-xl border border-line bg-card px-4 py-3.5 text-left transition hover:border-faint hover:shadow-sm"
          >
            <span className="shrink-0 text-[22px] leading-none">{n.icon || "📄"}</span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-[15px] font-500 text-ink">
                {n.title || "Без названия"}
              </span>
              {count(n.id) > 0 && (
                <span className="block text-[12px] text-muted">{childCountLabel(count(n.id))}</span>
              )}
            </span>
          </button>
        ))}
        <button
          type="button"
          onClick={add}
          disabled={busy}
          className="flex items-center justify-center gap-2 rounded-xl border border-dashed border-line px-4 py-3.5 text-[13px] text-muted transition hover:border-faint hover:text-ink disabled:opacity-60"
        >
          <Plus className="h-4 w-4" />
          Добавить страницу
        </button>
      </div>
    </div>
  );
}
