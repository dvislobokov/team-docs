// Рендер OpenAPI внутри страницы через Swagger UI (swagger-ui-dist, без
// React-обёртки — бандл фреймворк-независимый и не отстаёт от React).
// Swagger UI тяжёлый — грузится лениво (отдельный чанк, только на страницах
// с блоком). Источник — URL (Swagger фетчит сам) или вставленный YAML/JSON
// (парсим в объект). Тёмная тема — CSS-оверрайды .td-swagger в index.css.
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Braces, Maximize2, Minimize2 } from "lucide-react";

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

// Однократная ленивая загрузка бандла и его CSS.
let bundlePromise: Promise<any> | null = null;
function loadSwagger(): Promise<any> {
  bundlePromise ??= Promise.all([
    import("swagger-ui-dist/swagger-ui-es-bundle"),
    import("swagger-ui-dist/swagger-ui.css"),
  ]).then(([m]) => m.default);
  return bundlePromise;
}

// Монтирует Swagger UI в неуправляемый React'ом div.
function SwaggerView({ spec, url }: { spec?: unknown; url?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    let cancelled = false;
    void loadSwagger().then((SwaggerUIBundle) => {
      if (cancelled) return;
      setLoading(false);
      SwaggerUIBundle({
        domNode: node,
        ...(url ? { url } : { spec }),
        deepLinking: false,
        docExpansion: "list",
        defaultModelsExpandDepth: 1,
        displayRequestDuration: true,
        validatorUrl: null,
      });
    });
    return () => {
      cancelled = true;
      node.innerHTML = "";
    };
  }, [spec, url]);

  return (
    <>
      {loading && (
        <div className="px-4 py-8 text-center text-[13px] text-muted">Загрузка Swagger UI…</div>
      )}
      <div ref={ref} />
    </>
  );
}

export function OpenApiBlock({
  block,
  editor,
}: {
  block: { id: string; props: { source: string } };
  editor: { isEditable: boolean; updateBlock: (b: any, u: any) => void };
}) {
  const source = block.props.source || OPENAPI_SAMPLE;
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
    ro.observe(flow);
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

  // Для inline-спеки парсим в объект; URL отдаём Swagger UI напрямую.
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

  const commit = () => {
    setEditing(false);
    if (draft !== source) editor.updateBlock(block, { props: { source: draft } });
  };

  // Сам рендер Swagger UI (общий для inline и полноэкранного режима).
  const swagger = (keyPrefix: string) =>
    asUrl ? (
      <SwaggerView key={`${keyPrefix}-u`} url={active.trim()} />
    ) : (
      <SwaggerView key={`${keyPrefix}-s`} spec={spec} />
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
          <span className="font-mono text-[11px] uppercase tracking-wide text-faint">OpenAPI · Swagger</span>
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
          <div className="td-swagger overflow-auto bg-paper" style={{ height: "78vh", contain: "layout paint" }}>
            {swagger("inline")}
          </div>
        ) : (
          <div className="px-4 py-8 text-center text-[13px] text-muted">Загрузка…</div>
        )}
      </div>

      {/* Полноэкранный оверлей. */}
      {full &&
        createPortal(
          <div className="fixed inset-0 z-[100] flex flex-col bg-paper">
            <div className="flex items-center gap-2 border-b border-line bg-card px-4 py-2">
              <Braces className="h-4 w-4 text-faint" />
              <span className="font-mono text-[12px] uppercase tracking-wide text-faint">
                OpenAPI · Swagger
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
            <div className="td-swagger min-h-0 flex-1 overflow-auto bg-paper">{swagger("full")}</div>
          </div>,
          document.body,
        )}
    </div>
  );
}
