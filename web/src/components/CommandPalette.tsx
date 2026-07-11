import { useEffect, useMemo, useRef, useState } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import { useNavigate } from "react-router-dom";
import { FileText, Search } from "lucide-react";
import { search as searchApi } from "../api/pages";
import type { SearchHit } from "../api/types";
import { getRecents } from "../lib/recents";
import { usePalette } from "../store/palette";
import { useTree } from "../store/tree";

const SEARCH_DEBOUNCE_MS = 200;

// Командная палитра / поиск (⌘K). Radix Dialog + запрос к /api/search;
// snippet приходит как HTML-фрагмент ts_headline — рендерим через
// dangerouslySetInnerHTML (источник — свой бэкенд).
export function CommandPalette() {
  const { open, setOpen } = usePalette();
  const { nodes } = useTree();
  const navigate = useNavigate();

  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<SearchHit[]>([]);
  const [active, setActive] = useState(0);
  const seq = useRef(0);

  // Недавно открытые (из localStorage), резолвим в узлы дерева за title/icon.
  // Пересчитываем при каждом открытии палитры.
  const recent = useMemo(() => {
    if (!open) return [];
    const byId = new Map(nodes.map((n) => [n.id, n]));
    return getRecents()
      .map((rid) => byId.get(rid))
      .filter((n): n is (typeof nodes)[number] => Boolean(n));
  }, [nodes, open]);

  // Сбрасываем ввод при каждом открытии.
  useEffect(() => {
    if (open) {
      setQuery("");
      setHits([]);
      setActive(0);
    }
  }, [open]);

  // Дебаунс-поиск.
  useEffect(() => {
    if (!query.trim()) {
      setHits([]);
      return;
    }
    const my = ++seq.current;
    const t = setTimeout(() => {
      searchApi(query)
        .then((res) => {
          if (my === seq.current) {
            setHits(res);
            setActive(0);
          }
        })
        .catch(() => {
          if (my === seq.current) setHits([]);
        });
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [query]);

  const go = (id: number) => {
    setOpen(false);
    navigate(`/pages/${id}`);
  };

  const showingResults = query.trim().length > 0;
  const list = showingResults ? hits : recent;

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, list.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const item = list[active];
      if (item) go(item.id);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={setOpen}>
      <Dialog.Portal>
        <Dialog.Overlay className="anim-overlay fixed inset-0 z-30 bg-ink/20 backdrop-blur-[2px]" />
        <Dialog.Content
          onKeyDown={onKeyDown}
          className="fixed left-1/2 top-[18vh] z-40 w-[92%] max-w-[560px] -translate-x-1/2 overflow-hidden rounded-2xl border border-line bg-card shadow-2xl"
        >
          <Dialog.Title className="sr-only">Поиск по документации</Dialog.Title>
          <div className="flex items-center gap-3 border-b border-line px-4 py-3.5">
            <Search className="h-4 w-4 text-faint" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Поиск по документации…"
              className="w-full bg-transparent text-[15px] text-ink outline-none placeholder:text-faint"
            />
            <span className="rounded border border-line px-1.5 py-0.5 font-mono text-[10px] text-faint">
              esc
            </span>
          </div>

          <div className="max-h-[52vh] overflow-y-auto p-2">
            <div className="px-2 pb-1 pt-2 font-mono text-[10px] uppercase tracking-[0.08em] text-faint">
              {showingResults ? "Результаты" : "Недавние"}
            </div>

            {showingResults && hits.length === 0 && (
              <div className="px-3 py-4 text-[13px] text-faint">Ничего не найдено</div>
            )}
            {!showingResults && recent.length === 0 && (
              <div className="px-3 py-4 text-[13px] text-faint">Пока нет страниц</div>
            )}

            {list.map((item, i) => {
              const isActive = i === active;
              const hit = showingResults ? (item as SearchHit) : null;
              return (
                <button
                  key={item.id}
                  type="button"
                  onMouseEnter={() => setActive(i)}
                  onClick={() => go(item.id)}
                  className={
                    "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left " +
                    (isActive ? "bg-accentSoft" : "hover:bg-line/50")
                  }
                >
                  {item.icon ? (
                    <span className="w-4 shrink-0 text-center text-[15px] leading-none">
                      {item.icon}
                    </span>
                  ) : (
                    <FileText className="h-4 w-4 shrink-0 text-faint" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[14px] font-500 text-ink">
                      {item.title || "Без названия"}
                    </span>
                    {hit?.snippet && (
                      <span
                        className="block truncate text-[12px] text-muted [&_b]:bg-marker [&_b]:px-0.5 [&_b]:font-500 [&_b]:text-ink"
                        // snippet — доверенный HTML из ts_headline (<b> вокруг совпадений)
                        dangerouslySetInnerHTML={{ __html: hit.snippet }}
                      />
                    )}
                  </span>
                  {isActive && <span className="ml-auto font-mono text-[11px] text-faint">↵</span>}
                </button>
              );
            })}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
