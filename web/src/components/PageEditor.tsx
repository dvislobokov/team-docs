import {
  getDefaultReactSlashMenuItems,
  SuggestionMenuController,
  useCreateBlockNote,
} from "@blocknote/react";
import { BlockNoteView } from "@blocknote/mantine";
import { filterSuggestionItems, type PartialBlock } from "@blocknote/core";
import { ru } from "@blocknote/core/locales";
import {
  getMultiColumnSlashMenuItems,
  multiColumnDropCursor,
  locales as multiColumnLocales,
} from "@blocknote/xl-multi-column";
import { autoUpdate, offset, shift, size, type Middleware } from "@floating-ui/react";
import { uploadFile } from "../api/pages";
import type { PageContent } from "../api/types";
import {
  calloutMenuItems,
  mentionMenuItems,
  mermaidMenuItems,
  schema,
  statusMenuItems,
} from "../lib/editorSchema";
import type { Theme } from "../lib/theme";
import { useTree } from "../store/tree";

// Открываем меню вверх, если места снизу меньше порога (а сверху — больше).
// Это раньше стандартного flip: не ждём, пока меню упрётся в край, а уводим его
// вверх, когда внизу остаётся мало места.
const FLIP_THRESHOLD_PX = 440;
const preferTopWhenLow: Middleware = {
  name: "preferTopWhenLow",
  fn(state) {
    const ref = state.elements.reference.getBoundingClientRect();
    const spaceBelow = window.innerHeight - ref.bottom;
    const isTop = state.placement.startsWith("top");
    const wantTop = spaceBelow < FLIP_THRESHOLD_PX && ref.top > spaceBelow;
    if (wantTop && !isTop) return { reset: { placement: "top-start" } };
    if (!wantTop && isTop) return { reset: { placement: "bottom-start" } };
    return {};
  },
};

// Позиционирование меню подсказок: у нижнего края экрана оно переворачивается
// вверх, не вылезает по горизонтали (shift) и ограничивается по высоте
// доступным местом (size) — чтобы всегда можно было доскроллить до конца.
const suggestionFloatingOptions = {
  useFloatingOptions: {
    placement: "bottom-start" as const,
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(6),
      preferTopWhenLow,
      shift({ padding: 8 }),
      size({
        padding: 12,
        apply({ availableHeight, elements }: { availableHeight: number; elements: { floating: HTMLElement } }) {
          // Ограничиваем высоту скроллируемого меню доступным местом (inline —
          // чтобы перебить любые CSS-правила), внутри — свой скролл.
          const menu = elements.floating.querySelector<HTMLElement>(".bn-suggestion-menu");
          const target = menu ?? elements.floating;
          target.style.maxHeight = `${Math.max(160, availableHeight)}px`;
          target.style.overflowY = "auto";
        },
      }),
    ],
  },
};

interface PageEditorProps {
  /** Начальный документ. Компонент должен пересоздаваться при смене страницы
   *  (key={page.id} снаружи) — BlockNote не меняет initialContent на лету. */
  initialContent: PageContent;
  editable: boolean;
  theme: Theme;
  onChange: (content: PartialBlock[]) => void;
}

export function PageEditor({ initialContent, editable, theme, onChange }: PageEditorProps) {
  const { nodes } = useTree();

  // BlockNote требует непустой initialContent либо undefined (пустой массив —
  // ошибка). Пустой контент → даём редактору стартовать с чистого абзаца.
  const editor = useCreateBlockNote({
    schema,
    // Локализация + строки многоколоночности.
    dictionary: { ...ru, multi_column: multiColumnLocales.ru },
    // Drop-курсор с поддержкой перетаскивания в колонки.
    dropCursor: multiColumnDropCursor,
    initialContent: initialContent.length > 0 ? initialContent : undefined,
    uploadFile: async (file: File) => {
      const { url } = await uploadFile(file);
      return url;
    },
  });

  // Полное меню вставки: стандартные блоки + колонки + выноски + статусы +
  // mermaid. Одинаковое для «/» и «{{» (в стиле Confluence). Toggle-блоки —
  // среди стандартных пунктов BlockNote.
  const insertItems = async (query: string) =>
    filterSuggestionItems(
      [
        ...getDefaultReactSlashMenuItems(editor),
        ...getMultiColumnSlashMenuItems(editor),
        ...calloutMenuItems(editor, "Выноска: "),
        ...statusMenuItems(editor),
        ...mermaidMenuItems(editor),
      ],
      query,
    );

  return (
    <BlockNoteView
      editor={editor}
      editable={editable}
      theme={theme}
      slashMenu={false}
      onChange={() => onChange(editor.document as unknown as PartialBlock[])}
    >
      <SuggestionMenuController
        triggerCharacter="/"
        getItems={insertItems}
        floatingUIOptions={suggestionFloatingOptions}
      />
      <SuggestionMenuController
        triggerCharacter="{{"
        getItems={insertItems}
        floatingUIOptions={suggestionFloatingOptions}
      />
      {/* @-упоминания страниц из дерева. */}
      <SuggestionMenuController
        triggerCharacter="@"
        getItems={async (query) => mentionMenuItems(editor, nodes, query)}
        floatingUIOptions={suggestionFloatingOptions}
      />
    </BlockNoteView>
  );
}
