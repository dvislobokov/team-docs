import { useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { createPageFromMarkdown } from "../lib/pageActions";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";

interface ImportMarkdownProps {
  parentId?: number | null;
  className?: string;
  title?: string;
  children: ReactNode;
}

// Кнопка-лейбл со скрытым file-input: читает .md, парсит в блоки, создаёт
// страницу и открывает её. Заголовок — из первого H1 или имени файла.
export function ImportMarkdown({
  parentId = null,
  className,
  title,
  children,
}: ImportMarkdownProps) {
  const { reload } = useTree();
  const navigate = useNavigate();
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  const onFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ""; // позволяем повторно выбрать тот же файл
    if (!file || busy) return;
    setBusy(true);
    try {
      const md = await file.text();
      const saved = await createPageFromMarkdown(md, parentId, file.name);
      await reload();
      toast(`Импортировано: ${saved.title}`, "success");
      navigate(`/pages/${saved.id}`);
    } catch (err) {
      toast("Не удалось импортировать файл: " + (err instanceof Error ? err.message : err), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <label className={className} title={title} aria-disabled={busy}>
      <input
        type="file"
        accept=".md,.markdown,.txt,text/markdown,text/plain"
        className="hidden"
        onChange={onFile}
        disabled={busy}
      />
      {children}
    </label>
  );
}
