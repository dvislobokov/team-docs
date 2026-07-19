import { useState } from "react";
import { Plus, X } from "lucide-react";

interface PageTagsProps {
  tags: string[];
  editable: boolean;
  onChange: (tags: string[]) => void;
}

// Теги страницы: чипы + инлайн-добавление (Enter/запятая), × для удаления.
// Сохранение — сразу через onChange (родитель делает PUT).
export function PageTags({ tags, editable, onChange }: PageTagsProps) {
  const [adding, setAdding] = useState(false);
  const [draft, setDraft] = useState("");

  if (!editable && tags.length === 0) return null;

  const commit = () => {
    const t = draft.trim().replace(/,+$/, "");
    setDraft("");
    setAdding(false);
    if (t && !tags.includes(t)) onChange([...tags, t]);
  };

  return (
    <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
      {tags.map((t) => (
        <span
          key={t}
          className="group/tag inline-flex items-center gap-1 rounded-full bg-accentSoft px-2.5 py-0.5 text-[12px] text-accent"
        >
          {t}
          {editable && (
            <button
              type="button"
              onClick={() => onChange(tags.filter((x) => x !== t))}
              className="rounded-full opacity-40 transition hover:opacity-100 group-hover/tag:opacity-70"
              title="Убрать тег"
            >
              <X className="h-3 w-3" />
            </button>
          )}
        </span>
      ))}
      {editable &&
        (adding ? (
          <input
            autoFocus
            value={draft}
            onChange={(e) => {
              if (e.target.value.endsWith(",")) {
                setDraft(e.target.value);
                commit();
              } else {
                setDraft(e.target.value);
              }
            }}
            onBlur={commit}
            onKeyDown={(e) => {
              if (e.key === "Enter") commit();
              if (e.key === "Escape") {
                setDraft("");
                setAdding(false);
              }
            }}
            placeholder="тег"
            className="w-24 rounded-full border border-line bg-card px-2.5 py-0.5 text-[12px] text-ink outline-none focus:border-accent"
          />
        ) : (
          <button
            type="button"
            onClick={() => setAdding(true)}
            className="inline-flex items-center gap-1 rounded-full border border-dashed border-line px-2.5 py-0.5 text-[12px] text-faint transition hover:border-faint hover:text-body"
          >
            <Plus className="h-3 w-3" /> тег
          </button>
        ))}
    </div>
  );
}
