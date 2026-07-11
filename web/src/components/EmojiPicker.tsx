import { useEffect, useMemo, useRef, useState } from "react";
import emojiData from "@emoji-mart/data";
import { Search } from "lucide-react";

// Свой компактный emoji-пикер поверх датасета @emoji-mart/data — рисуем сетку
// сами, чтобы попасть в «бумажную» палитру (готовый Picker emoji-mart тянет
// свою тему и не стыкуется с оформлением).

interface EmojiEntry {
  id: string;
  name: string;
  keywords: string[];
  skins: { native: string }[];
}
interface EmojiData {
  categories: { id: string; emojis: string[] }[];
  emojis: Record<string, EmojiEntry>;
}

const DATA = emojiData as unknown as EmojiData;

const CATEGORY_LABELS: Record<string, string> = {
  people: "Смайлы и люди",
  nature: "Природа",
  foods: "Еда и напитки",
  activity: "Активность",
  places: "Путешествия",
  objects: "Объекты",
  symbols: "Символы",
  flags: "Флаги",
};

interface Section {
  id: string;
  label: string;
  emojis: EmojiEntry[];
}

// Предрасчёт секций один раз на модуль.
const SECTIONS: Section[] = DATA.categories.map((cat) => ({
  id: cat.id,
  label: CATEGORY_LABELS[cat.id] ?? cat.id,
  emojis: cat.emojis.map((eid) => DATA.emojis[eid]).filter(Boolean),
}));

const ALL: EmojiEntry[] = SECTIONS.flatMap((s) => s.emojis);

interface EmojiPickerProps {
  onSelect: (native: string) => void;
  onRemove?: () => void;
}

export function EmojiPicker({ onSelect, onRemove }: EmojiPickerProps) {
  const [query, setQuery] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return null;
    return ALL.filter(
      (e) =>
        e.id.includes(q) ||
        e.name.toLowerCase().includes(q) ||
        e.keywords.some((k) => k.includes(q)),
    ).slice(0, 90);
  }, [query]);

  return (
    <div className="w-[336px] overflow-hidden rounded-xl border border-line bg-card shadow-2xl">
      <div className="flex items-center gap-2 border-b border-line px-3 py-2.5">
        <Search className="h-4 w-4 text-faint" />
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Найти emoji…"
          className="w-full bg-transparent text-[13px] text-ink outline-none placeholder:text-faint"
        />
        {onRemove && (
          <button
            type="button"
            onClick={onRemove}
            className="shrink-0 rounded px-1.5 py-0.5 text-[11px] text-muted transition hover:bg-line/60 hover:text-ink"
          >
            Убрать
          </button>
        )}
      </div>

      <div className="scroll max-h-[280px] overflow-y-auto px-2 py-2">
        {results ? (
          results.length === 0 ? (
            <div className="px-2 py-6 text-center text-[12px] text-faint">Ничего не найдено</div>
          ) : (
            <Grid emojis={results} onSelect={onSelect} />
          )
        ) : (
          SECTIONS.map((s) => (
            <div key={s.id} className="mb-1">
              <div className="px-1 pb-1 pt-2 font-mono text-[10px] uppercase tracking-[0.08em] text-faint">
                {s.label}
              </div>
              <Grid emojis={s.emojis} onSelect={onSelect} />
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function Grid({
  emojis,
  onSelect,
}: {
  emojis: EmojiEntry[];
  onSelect: (native: string) => void;
}) {
  return (
    <div className="grid grid-cols-8 gap-0.5">
      {emojis.map((e) => (
        <button
          key={e.id}
          type="button"
          title={e.name}
          onClick={() => onSelect(e.skins[0]?.native ?? "")}
          className="flex h-9 w-9 items-center justify-center rounded-md text-[20px] leading-none transition hover:bg-line/70"
        >
          {e.skins[0]?.native}
        </button>
      ))}
    </div>
  );
}
