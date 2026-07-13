// Расширенная схема BlockNote: выноски (callout), inline-упоминания страниц
// (@mention), статус-бейджи, блок Mermaid-диаграмм и многоколоночность
// (withMultiColumn). Toggle-блоки — уже в дефолтной схеме BlockNote.
// Схема одна на весь проект — её же используют read-only рендер и безголовые
// редакторы (импорт/экспорт MD).
import { useEffect, useRef, useState } from "react";
import {
  BlockNoteSchema,
  type BlockNoteEditor,
  defaultBlockSpecs,
  defaultInlineContentSpecs,
} from "@blocknote/core";
import {
  createReactBlockSpec,
  createReactInlineContentSpec,
  type DefaultReactSuggestionItem,
} from "@blocknote/react";
import { withMultiColumn } from "@blocknote/xl-multi-column";
import { useNavigate } from "react-router-dom";
import { Braces, Workflow } from "lucide-react";
import type { PageTreeNode } from "../api/types";
import { useTheme } from "../lib/theme";
import { useTree } from "../store/tree";
import { OpenApiBlock, OPENAPI_SAMPLE } from "../components/OpenApiBlock";

// ─── Callout с вариантами (панели info/warning/…) ───
type Variant = "info" | "success" | "warning" | "danger" | "note";

const VARIANTS: Record<Variant, { label: string; cls: string; icon: string }> = {
  info: { label: "Инфо", cls: "border-blue-400/30 bg-blue-400/10", icon: "ℹ️" },
  success: { label: "Успех", cls: "border-emerald-400/30 bg-emerald-400/10", icon: "✅" },
  warning: { label: "Предупреждение", cls: "border-amber-400/30 bg-amber-400/10", icon: "⚠️" },
  danger: { label: "Ошибка", cls: "border-red-400/30 bg-red-400/10", icon: "⛔" },
  note: { label: "Заметка", cls: "border-line bg-line/50", icon: "📝" },
};
const VARIANT_KEYS = Object.keys(VARIANTS) as Variant[];

const calloutSpec = createReactBlockSpec(
  {
    type: "callout",
    propSchema: { variant: { default: "info" }, emoji: { default: "" } },
    content: "inline",
  },
  {
    render: ({ block, contentRef }) => {
      const v = VARIANTS[(block.props.variant as Variant) ?? "info"] ?? VARIANTS.info;
      const emoji = block.props.emoji || v.icon;
      return (
        <div className={"my-1 flex gap-3 rounded-xl border px-4 py-3.5 " + v.cls}>
          <span contentEditable={false} className="select-none text-[18px] leading-relaxed">
            {emoji}
          </span>
          <div ref={contentRef} className="flex-1 text-[15px] leading-relaxed text-ink" />
        </div>
      );
    },
  },
);

// ─── Inline: упоминание страницы (@) ───
interface MentionProps {
  inlineContent: { props: { pageId: string; label: string; icon: string } };
}

function MentionView({ inlineContent }: MentionProps) {
  const { pageId, label, icon } = inlineContent.props;
  const navigate = useNavigate();
  const { nodes } = useTree();
  const node = nodes.find((n) => String(n.id) === String(pageId));
  const title = node?.title || label || "страница";
  const ic = node?.icon || icon;
  return (
    <span
      contentEditable={false}
      onClick={() => pageId && navigate(`/pages/${pageId}`)}
      className="cursor-pointer rounded px-1 font-500 text-accent transition hover:bg-accentSoft"
      title="Открыть страницу"
    >
      {ic ? `${ic} ` : "@"}
      {title}
    </span>
  );
}

const mentionSpec = createReactInlineContentSpec(
  {
    type: "mention",
    propSchema: { pageId: { default: "" }, label: { default: "" }, icon: { default: "" } },
    content: "none",
  },
  { render: (props) => <MentionView {...props} /> },
);

// ─── Inline: статус-бейдж ───
type StatusColor = "done" | "doing" | "wait" | "blocked" | "review" | "idle";

const STATUS: Record<StatusColor, { label: string; cls: string }> = {
  done: { label: "Готово", cls: "bg-emerald-400/15 text-emerald-600 ring-emerald-400/30 dark:text-emerald-300" },
  doing: { label: "В работе", cls: "bg-blue-400/15 text-blue-600 ring-blue-400/30 dark:text-blue-300" },
  wait: { label: "Ожидает", cls: "bg-amber-400/15 text-amber-600 ring-amber-400/30 dark:text-amber-300" },
  blocked: { label: "Заблокировано", cls: "bg-red-400/15 text-red-600 ring-red-400/30 dark:text-red-300" },
  review: { label: "На ревью", cls: "bg-purple-400/15 text-purple-600 ring-purple-400/30 dark:text-purple-300" },
  idle: { label: "Не начато", cls: "bg-line/70 text-muted ring-line" },
};
const STATUS_KEYS = Object.keys(STATUS) as StatusColor[];

interface StatusProps {
  inlineContent: { props: { color: string; label: string } };
}

function StatusView({ inlineContent }: StatusProps) {
  const { color, label } = inlineContent.props;
  const s = STATUS[(color as StatusColor) ?? "idle"] ?? STATUS.idle;
  return (
    <span
      contentEditable={false}
      className={
        "inline-flex select-none items-center gap-1.5 rounded-full px-2 py-0.5 align-middle text-[12px] font-500 ring-1 ring-inset " +
        s.cls
      }
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" />
      {label || s.label}
    </span>
  );
}

const statusSpec = createReactInlineContentSpec(
  {
    type: "status",
    propSchema: { color: { default: "idle" }, label: { default: "" } },
    content: "none",
  },
  { render: (props) => <StatusView {...props} /> },
);

// ─── Блок Mermaid-диаграммы ───
const MERMAID_DEFAULT = "graph TD;\n  A[Старт] --> B[Готово];";

function MermaidBlock({
  block,
  editor,
}: {
  block: { id: string; props: { code: string } };
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  editor: { isEditable: boolean; updateBlock: (b: any, u: any) => void };
}) {
  const code = block.props.code || MERMAID_DEFAULT;
  const theme = useTheme();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(code);
  const [svg, setSvg] = useState("");
  const [err, setErr] = useState("");
  const seq = useRef(0);

  useEffect(() => setDraft(code), [code]);

  const source = editing ? draft : code;
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const mermaid = (await import("mermaid")).default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: theme === "dark" ? "dark" : "default",
        });
        const id = `mmd-${block.id.replace(/[^a-zA-Z0-9]/g, "")}-${++seq.current}`;
        const { svg } = await mermaid.render(id, source);
        if (!cancelled) {
          setSvg(svg);
          setErr("");
        }
      } catch (e) {
        if (!cancelled) setErr(e instanceof Error ? e.message : "Ошибка рендеринга диаграммы");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [source, theme, block.id]);

  const commit = () => {
    setEditing(false);
    if (draft !== code) editor.updateBlock(block, { props: { code: draft } });
  };

  return (
    <div contentEditable={false} className="my-1 overflow-hidden rounded-xl border border-line bg-card">
      <div className="flex items-center gap-2 border-b border-line px-3 py-1.5">
        <Workflow className="h-3.5 w-3.5 text-faint" />
        <span className="font-mono text-[11px] uppercase tracking-wide text-faint">Mermaid</span>
        {editor.isEditable && (
          <button
            type="button"
            onClick={() => (editing ? commit() : setEditing(true))}
            className="ml-auto rounded px-2 py-0.5 text-[12px] text-muted transition hover:bg-line/60 hover:text-ink"
          >
            {editing ? "Готово" : "Изменить"}
          </button>
        )}
      </div>

      {editing && (
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.stopPropagation()}
          onBlur={commit}
          spellCheck={false}
          className="scroll block h-40 w-full resize-none border-b border-line bg-paper px-3 py-2 font-mono text-[12.5px] leading-relaxed text-ink outline-none"
        />
      )}

      {err ? (
        <pre className="whitespace-pre-wrap px-4 py-3 font-mono text-[12px] text-red-500">{err}</pre>
      ) : (
        <div
          className="flex justify-center px-4 py-4 [&_svg]:max-w-full"
          dangerouslySetInnerHTML={{ __html: svg }}
        />
      )}
    </div>
  );
}

const mermaidSpec = createReactBlockSpec(
  {
    type: "mermaid",
    propSchema: { code: { default: MERMAID_DEFAULT } },
    content: "none",
  },
  { render: (props) => <MermaidBlock block={props.block} editor={props.editor} /> },
);

// ─── Блок OpenAPI (рендер спеки; см. OpenApiBlock.tsx) ───
const openApiSpec = createReactBlockSpec(
  {
    type: "openapi",
    propSchema: { source: { default: OPENAPI_SAMPLE } },
    content: "none",
  },
  { render: (props) => <OpenApiBlock block={props.block} editor={props.editor} /> },
);

// ─── Схема (+ многоколоночность) ───
export const schema = withMultiColumn(
  BlockNoteSchema.create({
    blockSpecs: {
      ...defaultBlockSpecs,
      callout: calloutSpec(),
      mermaid: mermaidSpec(),
      openapi: openApiSpec(),
    },
    inlineContentSpecs: { ...defaultInlineContentSpecs, mention: mentionSpec, status: statusSpec },
  }),
);

export type TDEditor = BlockNoteEditor<
  typeof schema.blockSchema,
  typeof schema.inlineContentSchema,
  typeof schema.styleSchema
>;

// ─── Пункты меню ───
function insertCallout(editor: TDEditor, variant: Variant) {
  const { block } = editor.getTextCursorPosition();
  const empty = block.type === "paragraph" && (block.content?.length ?? 0) === 0;
  const nb = { type: "callout" as const, props: { variant } };
  if (empty) editor.updateBlock(block, nb);
  else editor.insertBlocks([nb], block, "after");
}

export function calloutMenuItems(editor: TDEditor, prefix = ""): DefaultReactSuggestionItem[] {
  return VARIANT_KEYS.map((v) => ({
    title: `${prefix}${VARIANTS[v].label}`,
    subtext: "Цветная выноска-панель",
    aliases: [v, "callout", "выноска", "панель", "panel", VARIANTS[v].label.toLowerCase()],
    group: "Выноски",
    icon: <span className="text-[16px] leading-none">{VARIANTS[v].icon}</span>,
    onItemClick: () => insertCallout(editor, v),
  }));
}

export function statusMenuItems(editor: TDEditor): DefaultReactSuggestionItem[] {
  return STATUS_KEYS.map((k) => ({
    title: `Статус: ${STATUS[k].label}`,
    aliases: ["status", "статус", "бейдж", "badge", STATUS[k].label.toLowerCase()],
    group: "Статусы",
    icon: <span className="text-[14px]">🔖</span>,
    onItemClick: () =>
      editor.insertInlineContent([
        { type: "status", props: { color: k, label: STATUS[k].label } },
        " ",
      ]),
  }));
}

// Пресеты диаграмм и графиков Mermaid — вставляются со стартовым шаблоном.
const MERMAID_TEMPLATES: { label: string; group: string; aliases: string[]; code: string }[] = [
  {
    label: "Блок-схема",
    group: "Диаграммы",
    aliases: ["flowchart", "graph", "блок-схема"],
    code: "graph TD;\n  A[Начало] --> B{Условие?};\n  B -->|Да| C[Действие];\n  B -->|Нет| D[Конец];\n  C --> D;",
  },
  {
    label: "Столбцы",
    group: "Графики",
    aliases: ["bar", "график", "chart", "столбцы", "гистограмма"],
    code: 'xychart-beta\n  title "Активность по месяцам"\n  x-axis ["Янв", "Фев", "Мар", "Апр", "Май", "Июн"]\n  y-axis "Страниц" 0 --> 100\n  bar [20, 35, 45, 60, 80, 95]',
  },
  {
    label: "Линия",
    group: "Графики",
    aliases: ["line", "график", "chart", "линия", "тренд"],
    code: 'xychart-beta\n  title "Тренд"\n  x-axis ["Q1", "Q2", "Q3", "Q4"]\n  y-axis "Значение" 0 --> 100\n  line [30, 50, 45, 80]',
  },
  {
    label: "Круговая",
    group: "Графики",
    aliases: ["pie", "круговая", "доли", "chart"],
    code: 'pie title Распределение\n  "Готово" : 50\n  "В работе" : 30\n  "Ожидает" : 20',
  },
  {
    label: "Sequence",
    group: "Диаграммы",
    aliases: ["sequence", "последовательность"],
    code: "sequenceDiagram\n  participant K as Клиент\n  participant S as Сервер\n  K->>S: Запрос\n  S-->>K: Ответ",
  },
  {
    label: "Gantt",
    group: "Диаграммы",
    aliases: ["gantt", "гант", "план"],
    code: "gantt\n  title План работ\n  dateFormat YYYY-MM-DD\n  section Этап 1\n  Задача A :a1, 2024-01-01, 5d\n  Задача B :after a1, 4d",
  },
  {
    label: "Class",
    group: "Диаграммы",
    aliases: ["class", "класс", "uml"],
    code: "classDiagram\n  class Page {\n    +int id\n    +string title\n    +save()\n  }\n  Page --> Revision : has",
  },
  {
    label: "ER (сущности)",
    group: "Диаграммы",
    aliases: ["er", "erd", "сущности", "база"],
    code: 'erDiagram\n  PAGE ||--o{ REVISION : "has"\n  PAGE {\n    int id\n    string title\n  }',
  },
  {
    label: "State (состояния)",
    group: "Диаграммы",
    aliases: ["state", "состояния"],
    code: "stateDiagram-v2\n  [*] --> Черновик\n  Черновик --> Ревью\n  Ревью --> Опубликовано\n  Опубликовано --> [*]",
  },
  {
    label: "Mindmap",
    group: "Диаграммы",
    aliases: ["mindmap", "карта", "интеллект"],
    code: "mindmap\n  root((team-docs))\n    Страницы\n    Поиск\n    Диаграммы\n    Статусы",
  },
];

function insertMermaid(editor: TDEditor, code: string) {
  const { block } = editor.getTextCursorPosition();
  const empty = block.type === "paragraph" && (block.content?.length ?? 0) === 0;
  const nb = { type: "mermaid" as const, props: { code } };
  if (empty) editor.updateBlock(block, nb);
  else editor.insertBlocks([nb], block, "after");
}

export function mermaidMenuItems(editor: TDEditor): DefaultReactSuggestionItem[] {
  return MERMAID_TEMPLATES.map((t) => ({
    title: `${t.group === "Графики" ? "График" : "Диаграмма"}: ${t.label}`,
    subtext: "Mermaid",
    aliases: ["mermaid", "диаграмма", "график", "diagram", "chart", "схема", ...t.aliases],
    group: t.group,
    icon: <Workflow size={18} />,
    onItemClick: () => insertMermaid(editor, t.code),
  }));
}

function insertOpenApi(editor: TDEditor) {
  const { block } = editor.getTextCursorPosition();
  const empty = block.type === "paragraph" && (block.content?.length ?? 0) === 0;
  const nb = { type: "openapi" as const, props: { source: OPENAPI_SAMPLE } };
  if (empty) editor.updateBlock(block, nb);
  else editor.insertBlocks([nb], block, "after");
}

export function openApiMenuItems(editor: TDEditor): DefaultReactSuggestionItem[] {
  return [
    {
      title: "OpenAPI (Swagger)",
      subtext: "Рендер спецификации API из URL или YAML/JSON",
      aliases: ["openapi", "swagger", "api", "спека", "спецификация", "rest"],
      group: "Диаграммы",
      icon: <Braces size={18} />,
      onItemClick: () => insertOpenApi(editor),
    },
  ];
}

export function mentionMenuItems(
  editor: TDEditor,
  nodes: PageTreeNode[],
  query: string,
): DefaultReactSuggestionItem[] {
  const q = query.trim().toLowerCase();
  return nodes
    .filter((n) => !q || (n.title || "").toLowerCase().includes(q))
    .slice(0, 8)
    .map((n) => ({
      title: n.title || "Без названия",
      icon: <span className="text-[15px] leading-none">{n.icon || "📄"}</span>,
      onItemClick: () => {
        editor.insertInlineContent([
          { type: "mention", props: { pageId: String(n.id), label: n.title, icon: n.icon } },
          " ",
        ]);
      },
    }));
}
