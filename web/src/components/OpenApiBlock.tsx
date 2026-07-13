// Полнофункциональный рендер OpenAPI внутри страницы через Redoc
// (трёхколоночная документация, схемы, примеры, разрешение $ref).
// Redoc тяжёлый — грузится лениво (отдельный чанк, только на страницах с блоком).
// Источник — URL (Redoc фетчит сам) или вставленный YAML/JSON (парсим в объект).
/* eslint-disable @typescript-eslint/no-explicit-any */
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Braces, Maximize2, Minimize2 } from "lucide-react";
import { useTheme } from "../lib/theme";

// Ленивая загрузка Redoc: тянется только когда блок реально рендерится.
const RedocStandalone = lazy(async () => {
  const mod = await import("redoc");
  return { default: mod.RedocStandalone };
});

export const OPENAPI_SAMPLE = `openapi: 3.0.3
info:
  title: Пример API
  version: 1.0.0
  description: Замените на свою спецификацию (URL или YAML/JSON).
servers:
  - url: https://api.example.com/v1
paths:
  /ping:
    get:
      summary: Проверка живости
      tags: [Служебное]
      responses:
        '200':
          description: Сервис доступен
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pong'
components:
  schemas:
    Pong:
      type: object
      properties:
        status:
          type: string
          description: Всегда "ok"
      required: [status]
`;

const isUrl = (s: string) => /^https?:\/\/\S+$/i.test(s.trim());

// Парсит вставленную спеку (JSON нативно, YAML — лениво через js-yaml).
async function parseInline(text: string): Promise<any> {
  const t = text.trim();
  if (!t) throw new Error("Пустая спецификация");
  try {
    return JSON.parse(t);
  } catch {
    const mod: any = await import("js-yaml");
    const load = mod.load ?? mod.default?.load;
    return load(t);
  }
}

// Читает CSS-токен вида "251 250 248" → "rgb(251 250 248)".
function tokenRGB(name: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v ? `rgb(${v})` : fallback;
}

// Тема Redoc из токенов приложения (адаптирует акцент, шрифты и тёмный режим).
function redocTheme(dark: boolean) {
  const accent = tokenRGB("--c-accent", "rgb(59 130 246)");
  const ink = tokenRGB("--c-ink", dark ? "rgb(237 237 240)" : "rgb(20 20 25)");
  const body = tokenRGB("--c-body", dark ? "rgb(200 200 210)" : "rgb(70 70 80)");
  const card = tokenRGB("--c-card", dark ? "rgb(32 32 40)" : "rgb(255 255 255)");
  const paper = tokenRGB("--c-paper", dark ? "rgb(24 24 30)" : "rgb(251 250 248)");
  const line = tokenRGB("--c-line", dark ? "rgb(60 60 72)" : "rgb(230 228 224)");

  return {
    colors: {
      primary: { main: accent },
      text: { primary: ink, secondary: body },
      border: { light: line, dark: line },
      http: {
        get: "rgb(59 130 246)",
        post: "rgb(16 185 129)",
        put: "rgb(245 158 11)",
        patch: "rgb(168 85 247)",
        delete: "rgb(239 68 68)",
      },
    },
    typography: {
      fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
      fontSize: "14px",
      headings: { fontFamily: "Inter, ui-sans-serif, sans-serif", fontWeight: "600" },
      code: {
        fontFamily: 'JetBrains Mono, ui-monospace, monospace',
        fontSize: "12.5px",
        color: dark ? "rgb(230 230 240)" : "rgb(30 30 40)",
        backgroundColor: dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.04)",
      },
    },
    sidebar: {
      backgroundColor: paper,
      textColor: body,
      activeTextColor: accent,
      arrow: { color: body },
    },
    rightPanel: {
      backgroundColor: dark ? "rgb(18 18 24)" : "rgb(38 40 52)",
      textColor: "rgb(240 240 245)",
    },
    schema: {
      nestedBackground: dark ? "rgba(255,255,255,0.03)" : "rgba(0,0,0,0.02)",
      typeNameColor: accent,
    },
    // Для тёмной темы окрашиваем основную панель.
    ...(dark ? { background: card } : {}),
  };
}

export function OpenApiBlock({
  block,
  editor,
}: {
  block: { id: string; props: { source: string } };
  editor: { isEditable: boolean; updateBlock: (b: any, u: any) => void };
}) {
  const source = block.props.source || OPENAPI_SAMPLE;
  const theme = useTheme();
  const dark = theme === "dark";
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(source);
  const [spec, setSpec] = useState<any>(null); // распарсенный объект для inline
  const [err, setErr] = useState("");
  const [full, setFull] = useState(false); // полноэкранный режим
  const [bo, setBo] = useState<{ width: number; ml: number } | null>(null); // «вырывание» по ширине
  const flowRef = useRef<HTMLDivElement>(null);

  useEffect(() => setDraft(source), [source]);

  // Расширяем блок из колонки контента до ширины всей области (flex-1 без
  // правого оглавления). Измеряем flowRef (остаётся в колонке) и регион-родителя.
  useEffect(() => {
    const flow = flowRef.current;
    const region = flow?.closest("article")?.parentElement as HTMLElement | null;
    if (!flow || !region) return;
    const measure = () => {
      const fr = flow.getBoundingClientRect();
      const gr = region.getBoundingClientRect();
      const width = region.clientWidth;
      // Сдвигаем влево так, чтобы левый край блока совпал с левым краем региона.
      setBo({ width, ml: gr.left - fr.left });
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(region);
    ro.observe(flow); // ширина колонки меняется (narrow/medium/wide) → пересчёт сдвига
    window.addEventListener("resize", measure);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, []);

  // Esc закрывает полноэкранный режим; блокируем скролл фона.
  useEffect(() => {
    if (!full) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setFull(false);
    document.addEventListener("keydown", onKey);
    const prev = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = prev;
    };
  }, [full]);

  const active = editing ? draft : source;
  const asUrl = isUrl(active);

  // Для inline-спеки парсим в объект; для URL отдаём его Redoc напрямую.
  useEffect(() => {
    if (asUrl) {
      setSpec(null);
      setErr("");
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const parsed = await parseInline(active);
        if (cancelled) return;
        if (!parsed || typeof parsed !== "object") throw new Error("Не удалось разобрать спецификацию");
        setSpec(parsed);
        setErr("");
      } catch (e) {
        if (!cancelled) {
          setErr(e instanceof Error ? e.message : "Ошибка разбора спецификации");
          setSpec(null);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [active, asUrl]);

  const options = useMemo(
    () => ({
      hideDownloadButton: true,
      nativeScrollbars: true,
      expandResponses: "200,201",
      scrollYOffset: 0,
      menuToggle: true,
      theme: redocTheme(dark) as any,
    }),
    [dark],
  );

  const commit = () => {
    setEditing(false);
    if (draft !== source) editor.updateBlock(block, { props: { source: draft } });
  };

  const loading = <div className="px-4 py-8 text-center text-[13px] text-muted">Загрузка Redoc…</div>;

  // Сам рендер Redoc (общий для inline и полноэкранного режима).
  const redoc = (keyPrefix: string) => (
    <Suspense fallback={loading}>
      {asUrl ? (
        <RedocStandalone key={`${keyPrefix}-u-${dark}`} specUrl={active.trim()} options={options} />
      ) : (
        <RedocStandalone key={`${keyPrefix}-s-${dark}`} spec={spec} options={options} />
      )}
    </Suspense>
  );

  const hasSpec = asUrl || spec;

  return (
    <div ref={flowRef} contentEditable={false} className="my-1">
      {/* Карта «вырвана» из колонки на всю ширину области контента. */}
      <div
        className="overflow-hidden rounded-xl border border-line bg-card"
        style={bo ? { width: bo.width, marginLeft: bo.ml } : undefined}
      >
        <div className="flex items-center gap-2 border-b border-line px-3 py-1.5">
          <Braces className="h-3.5 w-3.5 text-faint" />
          <span className="font-mono text-[11px] uppercase tracking-wide text-faint">OpenAPI · Redoc</span>
          {asUrl && !editing && (
            <span className="truncate font-mono text-[11px] text-muted">· {active.trim()}</span>
          )}
          <div className="ml-auto flex items-center gap-1">
            {hasSpec && !editing && (
              <button
                type="button"
                onClick={() => setFull(true)}
                className="flex items-center gap-1 rounded px-2 py-0.5 text-[12px] text-muted transition hover:bg-line/60 hover:text-ink"
                title="Открыть на весь экран"
              >
                <Maximize2 className="h-3.5 w-3.5" /> На весь экран
              </button>
            )}
            {editor.isEditable && (
              <button
                type="button"
                onClick={() => (editing ? commit() : setEditing(true))}
                className="rounded px-2 py-0.5 text-[12px] text-muted transition hover:bg-line/60 hover:text-ink"
              >
                {editing ? "Готово" : "Изменить"}
              </button>
            )}
          </div>
        </div>

        {editing && (
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => e.stopPropagation()}
            onBlur={commit}
            spellCheck={false}
            placeholder="URL спецификации или YAML/JSON…"
            className="scroll block h-56 w-full resize-none border-b border-line bg-paper px-3 py-2 font-mono text-[12.5px] leading-relaxed text-ink outline-none"
          />
        )}

        {err ? (
          <pre className="whitespace-pre-wrap px-4 py-3 font-mono text-[12px] text-red-500">{err}</pre>
        ) : full ? (
          <div className="px-4 py-10 text-center text-[13px] text-muted">Открыто на весь экран…</div>
        ) : hasSpec ? (
          <div className="td-redoc overflow-auto bg-paper" style={{ height: "78vh", contain: "layout paint" }}>
            {redoc("inline")}
          </div>
        ) : (
          loading
        )}
      </div>

      {/* Полноэкранный оверлей. */}
      {full &&
        createPortal(
          <div className="fixed inset-0 z-[100] flex flex-col bg-paper">
            <div className="flex items-center gap-2 border-b border-line bg-card px-4 py-2">
              <Braces className="h-4 w-4 text-faint" />
              <span className="font-mono text-[12px] uppercase tracking-wide text-faint">
                OpenAPI · Redoc
              </span>
              {asUrl && <span className="truncate font-mono text-[12px] text-muted">· {active.trim()}</span>}
              <button
                type="button"
                onClick={() => setFull(false)}
                className="ml-auto flex items-center gap-1.5 rounded-md px-2.5 py-1 text-[13px] text-body transition hover:bg-line/60 hover:text-ink"
                title="Свернуть (Esc)"
              >
                <Minimize2 className="h-4 w-4" /> Свернуть
              </button>
            </div>
            <div className="td-redoc min-h-0 flex-1 overflow-auto bg-paper">{redoc("full")}</div>
          </div>,
          document.body,
        )}
    </div>
  );
}
