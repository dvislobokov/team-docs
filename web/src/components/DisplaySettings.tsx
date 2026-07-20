import { useEffect, useRef, useState } from "react";
import { Check, Compass, Moon, SlidersHorizontal, Sun } from "lucide-react";
import { setContentWidth, useContentWidth, WIDTH_LABEL, type ContentWidth } from "../lib/contentWidth";
import { setTheme, useTheme, type Theme } from "../lib/theme";
import { SCALE_LABEL, setUIScale, useUIScale, type UIScale } from "../lib/uiScale";
import { startTour } from "../lib/tour";
import { useBranding } from "../store/branding";

const WIDTHS: ContentWidth[] = ["narrow", "medium", "wide"];
const SCALES: UIScale[] = ["small", "medium", "large"];

// Кнопка в топбаре с сабменю: оформление (светло/тёмно), ширина контента и
// цветовая схема (из /api/branding, выбор сохраняется в localStorage).
// Экспорт/импорт БД — в /admin, секция «Данные».
export function DisplaySettings() {
  const theme = useTheme();
  const width = useContentWidth();
  const scale = useUIScale();
  const { themes, schemeId, setScheme } = useBranding();
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div ref={wrapRef} className="relative">
      <button
        type="button"
        data-tour="settings"
        onClick={() => setOpen((v) => !v)}
        className="rounded-md p-1.5 text-muted transition hover:bg-line/60 hover:text-ink"
        title="Оформление"
      >
        <SlidersHorizontal className="h-[17px] w-[17px]" />
      </button>

      {open && (
        <div className="animate-fade-in absolute right-0 top-full z-50 mt-1 w-64 overflow-hidden rounded-xl border border-line bg-card p-3 shadow-xl">
          <Section title="Оформление">
            <div className="grid grid-cols-2 gap-1.5">
              <Segment active={theme === "light"} onClick={() => setTheme("light" as Theme)}>
                <Sun className="h-4 w-4" /> Светлое
              </Segment>
              <Segment active={theme === "dark"} onClick={() => setTheme("dark" as Theme)}>
                <Moon className="h-4 w-4" /> Тёмное
              </Segment>
            </div>
          </Section>

          <Section title="Размер интерфейса">
            <div className="grid grid-cols-3 gap-1.5">
              {SCALES.map((s) => (
                <Segment key={s} active={scale === s} onClick={() => setUIScale(s)}>
                  {SCALE_LABEL[s]}
                </Segment>
              ))}
            </div>
          </Section>

          <Section title="Ширина контента">
            <div className="grid grid-cols-3 gap-1.5">
              {WIDTHS.map((w) => (
                <Segment key={w} active={width === w} onClick={() => setContentWidth(w)}>
                  {WIDTH_LABEL[w]}
                </Segment>
              ))}
            </div>
          </Section>

          <Section title="Цветовая схема">
            <div className="-mx-1 max-h-56 overflow-y-auto">
              {themes.map((t) => {
                const accent = t.palette[theme].accent;
                const on = t.id === schemeId;
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => setScheme(t.id)}
                    className={
                      "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-[13px] transition hover:bg-line/60 " +
                      (on ? "text-ink" : "text-body")
                    }
                  >
                    <span
                      className="h-3.5 w-3.5 shrink-0 rounded-full ring-1 ring-inset ring-black/10"
                      style={{ backgroundColor: `rgb(${accent})` }}
                    />
                    <span className="flex-1">{t.label}</span>
                    {on && <Check className="h-3.5 w-3.5 text-accent" />}
                  </button>
                );
              })}
            </div>
          </Section>

          {/* Экспорт/импорт БД переехали в /admin, секция «Данные» (§9). */}
          <div className="mt-1 border-t border-line pt-2">
            <button
              type="button"
              onClick={() => {
                setOpen(false);
                startTour();
              }}
              className="flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-[13px] text-body transition hover:bg-line/60"
            >
              <Compass className="h-4 w-4 shrink-0 text-faint" />
              Показать тур
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-2.5 last:mb-0">
      <div className="mb-1.5 px-1 font-mono text-[10px] uppercase tracking-[0.08em] text-faint">
        {title}
      </div>
      {children}
    </div>
  );
}

function Segment({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex items-center justify-center gap-1.5 rounded-lg border px-2 py-1.5 text-[12.5px] transition " +
        (active
          ? "border-accent bg-accentSoft font-500 text-ink"
          : "border-line text-body hover:border-faint")
      }
    >
      {children}
    </button>
  );
}
