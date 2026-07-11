import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { isTourDone, markTourDone } from "../lib/tour";

interface Step {
  /** CSS-селектор подсвечиваемого элемента; null — карточка по центру. */
  sel: string | null;
  title: string;
  text: string;
}

const STEPS: Step[] = [
  {
    sel: null,
    title: "Добро пожаловать 👋",
    text: "team-docs — вики для команды. Пройдёмся по основным местам за полминуты.",
  },
  {
    sel: '[data-tour="search"]',
    title: "Поиск",
    text: "Мгновенный поиск по всем страницам — по кнопке или ⌘K из любого места.",
  },
  {
    sel: '[data-tour="new-page"]',
    title: "Новые страницы",
    text: "Создавайте страницы и вложенные подстраницы. В дереве их можно перетаскивать.",
  },
  {
    sel: '[data-tour="tree"]',
    title: "Дерево пространства",
    text: "Вся структура здесь. Правый клик по странице — переименовать, дублировать, экспорт.",
  },
  {
    sel: '[data-tour="editor"]',
    title: "Редактор",
    text: "Пишите как в Notion. Наберите «/» или «{{» для вставки блоков, «@» — упоминание страницы.",
  },
  {
    sel: '[data-tour="settings"]',
    title: "Оформление",
    text: "Тема (светлая/тёмная), ширина контента и цветовая схема — Dracula, Nord, Tokyo и другие.",
  },
  {
    sel: null,
    title: "Готово! 🎉",
    text: "Приятной работы. Тур можно перезапустить в меню оформления.",
  },
];

const CARD_W = 340;
const CARD_H = 176;
const GAP = 12;
const PAD = 8;

interface Rect {
  top: number;
  left: number;
  width: number;
  height: number;
}

function visibleRect(sel: string): Rect | null {
  const el = document.querySelector(sel);
  if (!el) return null;
  const r = el.getBoundingClientRect();
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const visible = r.width > 0 && r.height > 0 && r.right > 4 && r.left < vw - 4 && r.bottom > 4 && r.top < vh - 4;
  return visible ? { top: r.top, left: r.left, width: r.width, height: r.height } : null;
}

export function Tour() {
  const [active, setActive] = useState(false);
  const [index, setIndex] = useState(0);
  const [rect, setRect] = useState<Rect | null>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const [cardPos, setCardPos] = useState<{ top: number; left: number } | null>(null);

  const step = STEPS[index];

  const finish = useCallback(() => {
    markTourDone();
    setActive(false);
  }, []);

  const go = useCallback((next: number) => {
    if (next >= STEPS.length) {
      markTourDone();
      setActive(false);
      return;
    }
    setIndex(Math.max(0, next));
  }, []);

  // Автозапуск при первом визите + запуск по событию (перезапуск из меню).
  useEffect(() => {
    let t: ReturnType<typeof setTimeout> | null = null;
    if (!isTourDone()) {
      t = setTimeout(() => {
        setIndex(0);
        setActive(true);
      }, 700);
    }
    const onStart = () => {
      setIndex(0);
      setActive(true);
    };
    window.addEventListener("td:tour", onStart);
    return () => {
      if (t) clearTimeout(t);
      window.removeEventListener("td:tour", onStart);
    };
  }, []);

  // Esc закрывает тур.
  useEffect(() => {
    if (!active) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") finish();
      if (e.key === "ArrowRight" || e.key === "Enter") go(index + 1);
      if (e.key === "ArrowLeft") go(index - 1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, index, finish, go]);

  // Вычисляем прямоугольник цели; если её нет — пропускаем шаг.
  useLayoutEffect(() => {
    if (!active) return;
    if (step.sel === null) {
      setRect(null);
      return;
    }
    const r = visibleRect(step.sel);
    if (!r) {
      // цель недоступна (напр. сайдбар скрыт на мобильном) — следующий шаг
      go(index + 1);
      return;
    }
    setRect(r);
  }, [active, index, step.sel, go]);

  // Позиция карточки относительно цели.
  useLayoutEffect(() => {
    if (!active) return;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    if (!rect || step.sel === null) {
      setCardPos({ top: (vh - CARD_H) / 2, left: (vw - Math.min(CARD_W, vw - 24)) / 2 });
      return;
    }
    const below = rect.top + rect.height + GAP;
    const above = rect.top - CARD_H - GAP;
    let top = below;
    if (below + CARD_H > vh && above >= 8) top = above;
    else if (below + CARD_H > vh) top = Math.max(8, vh - CARD_H - 8);
    let left = rect.left;
    if (rect.left + rect.width / 2 > vw / 2) left = rect.left + rect.width - CARD_W; // цель справа
    left = Math.min(Math.max(12, left), vw - CARD_W - 12);
    setCardPos({ top, left });
  }, [active, rect, index, step.sel]);

  if (!active || !cardPos) return null;

  const isLast = index === STEPS.length - 1;

  return (
    <div className="fixed inset-0 z-[70]">
      {/* Ловим клики по приложению во время тура. */}
      <div className="absolute inset-0" onClick={() => finish()} />

      {/* Спотлайт: прозрачное окно вокруг цели, тень затемняет остальное. */}
      {rect ? (
        <div
          className="pointer-events-none absolute rounded-xl ring-2 ring-accent transition-all duration-200"
          style={{
            top: rect.top - PAD,
            left: rect.left - PAD,
            width: rect.width + PAD * 2,
            height: rect.height + PAD * 2,
            boxShadow: "0 0 0 9999px rgb(0 0 0 / 0.55)",
          }}
        />
      ) : (
        <div className="absolute inset-0 bg-ink/40 backdrop-blur-[1px]" />
      )}

      {/* Карточка шага. */}
      <div
        ref={cardRef}
        className="animate-fade-in absolute w-[340px] max-w-[calc(100vw-24px)] rounded-2xl border border-line bg-card p-4 shadow-2xl"
        style={{ top: cardPos.top, left: cardPos.left }}
      >
        <div className="font-display text-[17px] font-500 text-ink">{step.title}</div>
        <p className="mt-1.5 text-[13.5px] leading-relaxed text-muted">{step.text}</p>

        <div className="mt-4 flex items-center gap-1.5">
          {STEPS.map((_, i) => (
            <span
              key={i}
              className={
                "h-1.5 rounded-full transition-all " +
                (i === index ? "w-4 bg-accent" : "w-1.5 bg-line")
              }
            />
          ))}
          <div className="ml-auto flex items-center gap-2">
            {!isLast && (
              <button
                type="button"
                onClick={finish}
                className="rounded-md px-2 py-1 text-[12.5px] text-muted transition hover:text-ink"
              >
                Пропустить
              </button>
            )}
            {index > 0 && (
              <button
                type="button"
                onClick={() => go(index - 1)}
                className="rounded-md border border-line px-2.5 py-1 text-[12.5px] text-body transition hover:border-faint"
              >
                Назад
              </button>
            )}
            <button
              type="button"
              onClick={() => go(index + 1)}
              className="rounded-md bg-accent px-3 py-1 text-[12.5px] font-500 text-white transition hover:bg-accent/90"
            >
              {isLast ? "Готово" : "Далее"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
