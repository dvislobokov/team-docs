import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { createPage } from "../api/pages";
import { HOTKEYS_HELP, useHotkey } from "../lib/hotkeys";
import { useAuth } from "../store/auth";
import { useSidebar } from "../store/sidebar";
import { useTree } from "../store/tree";

// Глобальные горячие клавиши (навигация и команды уровня приложения) плюс
// справка по Ctrl+/. Хоткеи уровня страницы живут в PageScreen, палитра —
// в PaletteProvider.
export function GlobalHotkeys() {
  const navigate = useNavigate();
  const user = useAuth();
  const { toggle } = useSidebar();
  const { project, reload } = useTree();
  const [helpOpen, setHelpOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const canCreate = user.canEdit && project?.myRole !== "reader";

  useHotkey({ code: "Slash", allowInEditable: true }, () => setHelpOpen((v) => !v));
  useHotkey({ code: "KeyT", alt: true }, toggle);
  useHotkey({ code: "KeyA", alt: true }, () => {
    if (user.isAdmin) navigate("/admin");
  });
  useHotkey({ code: "KeyN", alt: true }, async () => {
    if (!canCreate || busy) return;
    setBusy(true);
    try {
      const page = await createPage({
        parentId: null,
        title: "Новая страница",
        projectId: project?.id,
      });
      await reload();
      navigate(`/pages/${page.id}`, { state: { isNew: true } });
    } finally {
      setBusy(false);
    }
  });

  useEffect(() => {
    if (!helpOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setHelpOpen(false);
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [helpOpen]);

  if (!helpOpen) return null;
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4 backdrop-blur-[1px]"
      onClick={() => setHelpOpen(false)}
    >
      <div
        className="animate-fade-in w-full max-w-[420px] rounded-xl border border-line bg-card p-5 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-[15px] font-600 text-ink">Горячие клавиши</h2>
        <div className="mt-3 flex flex-col gap-0.5">
          {HOTKEYS_HELP.map((h) => (
            <div key={h.keys} className="flex items-center gap-3 py-1">
              <span className="flex-1 text-[13px] text-body">{h.label}</span>
              <kbd className="rounded-md border border-line bg-paper px-1.5 py-0.5 font-mono text-[11px] text-muted">
                {h.keys}
              </kbd>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
