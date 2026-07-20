import { useEffect, useState } from "react";
import { List, X } from "lucide-react";
import type { Heading } from "../lib/blocks";

const SCROLL_ROOT = "main-scroll";
const TOP_OFFSET = 96; // высота липкого топбара + отступ
const TOC_LS_KEY = "td-toc-open"; // по умолчанию скрыто

// Правое оглавление «На этой странице» с подсветкой активной секции (scroll-spy)
// и плавным переходом. Якоря — элементы BlockNote с data-id = id блока-заголовка.
// По умолчанию свёрнуто в кнопку; выбор запоминается в localStorage.
export function RightRail({ headings }: { headings: Heading[] }) {
  const [active, setActive] = useState<string | null>(null);
  const [open, setOpen] = useState(() => localStorage.getItem(TOC_LS_KEY) === "1");

  const toggle = (v: boolean) => {
    setOpen(v);
    localStorage.setItem(TOC_LS_KEY, v ? "1" : "0");
  };

  useEffect(() => {
    const root = document.getElementById(SCROLL_ROOT);
    if (!root || headings.length === 0) return;

    const els = headings
      .map((h) => root.querySelector<HTMLElement>(`[data-id="${h.id}"]`))
      .filter((e): e is HTMLElement => e != null);
    if (els.length === 0) return;

    // Держим карту видимых заголовков; активный — самый верхний из видимых.
    const visible = new Map<string, number>();
    const obs = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          const id = e.target.getAttribute("data-id");
          if (!id) continue;
          if (e.isIntersecting) visible.set(id, e.boundingClientRect.top);
          else visible.delete(id);
        }
        if (visible.size > 0) {
          const top = [...visible.entries()].sort((a, b) => a[1] - b[1])[0][0];
          setActive(top);
        }
      },
      { root, rootMargin: `-${TOP_OFFSET}px 0px -70% 0px`, threshold: 0 },
    );
    els.forEach((el) => obs.observe(el));
    return () => obs.disconnect();
  }, [headings]);

  const scrollTo = (id: string) => {
    const root = document.getElementById(SCROLL_ROOT);
    const el = root?.querySelector<HTMLElement>(`[data-id="${id}"]`);
    if (!root || !el) return;
    const top = el.getBoundingClientRect().top - root.getBoundingClientRect().top + root.scrollTop - TOP_OFFSET;
    root.scrollTo({ top, behavior: "smooth" });
    setActive(id);
  };

  if (headings.length === 0) return null;

  // Свёрнуто: компактная кнопка у правого края.
  if (!open) {
    return (
      <aside className="hidden w-10 shrink-0 xl:block">
        <div className="sticky top-24 flex justify-end">
          <button
            type="button"
            onClick={() => toggle(true)}
            title="На этой странице"
            className="rounded-md border border-line bg-card p-1.5 text-faint transition hover:border-faint hover:text-ink"
          >
            <List className="h-4 w-4" />
          </button>
        </div>
      </aside>
    );
  }

  return (
    <aside className="hidden w-52 shrink-0 xl:block">
      <div className="sticky top-24">
        <div className="mb-3 flex items-center justify-between">
          <span className="font-mono text-[11px] uppercase tracking-[0.08em] text-faint">
            На этой странице
          </span>
          <button
            type="button"
            onClick={() => toggle(false)}
            title="Скрыть оглавление"
            className="rounded p-0.5 text-faint transition hover:bg-line/60 hover:text-ink"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        <ul className="space-y-2 border-l border-line text-[13px]">
          {headings.map((h) => {
            const on = h.id === active;
            return (
              <li key={h.id}>
                <button
                  type="button"
                  onClick={() => scrollTo(h.id)}
                  style={{ paddingLeft: 12 + (h.level - 1) * 10 }}
                  className={
                    "-ml-px block w-full border-l-2 text-left transition " +
                    (on
                      ? "border-accent font-500 text-ink"
                      : "border-transparent text-muted hover:text-ink")
                  }
                >
                  {h.text}
                </button>
              </li>
            );
          })}
        </ul>
      </div>
    </aside>
  );
}
