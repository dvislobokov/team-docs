import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { ClipboardPaste, LogOut, PanelLeftClose, Plus, Search, Settings, Trash2, Upload } from "lucide-react";
import { createPage } from "../api/pages";
import { useAuth } from "../store/auth";
import { useBranding } from "../store/branding";
import { usePalette } from "../store/palette";
import { useSidebar } from "../store/sidebar";
import { useTree } from "../store/tree";
import { ImportMarkdown } from "./ImportMarkdown";
import { PasteMarkdown } from "./PasteMarkdown";
import { TrashDialog } from "./TrashDialog";
import { TreeNav } from "./TreeNav";

// Левый сайдбар: воркспейс, поиск (⌘K), дерево страниц, футер пользователя.
// Разметка перенесена из design/mockup.html.
export function Sidebar() {
  const { setOpen: setPaletteOpen } = usePalette();
  const { open, setOpen } = useSidebar();
  const { reload, projects, project, setProject } = useTree();
  const user = useAuth();
  const branding = useBranding();
  const navigate = useNavigate();
  const location = useLocation();
  const [busy, setBusy] = useState(false);
  const [trashOpen, setTrashOpen] = useState(false);

  // На мобильном закрываем сайдбар при переходе на другую страницу.
  useEffect(() => {
    if (window.matchMedia("(max-width: 767px)").matches) setOpen(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.pathname]);

  const newRootPage = async () => {
    if (busy) return;
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
  };

  return (
    <aside
      className={
        "flex w-[312px] shrink-0 flex-col border-r border-line bg-paper transition-[transform,margin] duration-200 ease-out " +
        "fixed inset-y-0 left-0 z-40 md:static md:z-auto " +
        (open
          ? "translate-x-0 md:ml-0"
          : "-translate-x-full md:translate-x-0 md:-ml-[312px]")
      }
    >
      {/* воркспейс — клик ведёт на главную; справа — свернуть */}
      <div className="flex items-center gap-1 px-2 pt-3">
        <button
          type="button"
          onClick={() => navigate("/")}
          className="flex flex-1 items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition hover:bg-line/40"
        >
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-accent font-display text-sm font-600 lowercase text-white">
            {branding.monogram}
          </span>
          <div className="min-w-0 leading-tight">
            <div className="truncate text-[14px] font-600 text-ink">{branding.appName}</div>
            <div className="truncate text-[11px] text-faint">{branding.workspaceName}</div>
          </div>
        </button>
        <button
          type="button"
          onClick={() => setOpen(false)}
          className="shrink-0 rounded-md p-1.5 text-faint transition hover:bg-line/60 hover:text-ink"
          title="Свернуть панель"
        >
          <PanelLeftClose className="h-[18px] w-[18px]" />
        </button>
      </div>

      {/* поиск */}
      <div className="px-3 pb-3 pt-1">
        <button
          type="button"
          data-tour="search"
          onClick={() => setPaletteOpen(true)}
          className="flex w-full items-center gap-2 rounded-lg border border-line bg-card px-3 py-2 text-left text-[13px] text-muted transition hover:border-faint"
        >
          <Search className="h-4 w-4 text-faint" />
          Поиск…
          <span className="ml-auto font-mono text-[11px] text-faint">⌘K</span>
        </button>
      </div>

      {/* дерево */}
      <nav data-tour="tree" className="scroll flex-1 overflow-y-auto px-2 pb-4 text-[15px]">
        <div className="flex items-center justify-between gap-2 px-2 pb-1 pt-3">
          {projects.length > 1 ? (
            <select
              value={project?.key ?? ""}
              onChange={(e) => {
                const p = projects.find((x) => x.key === e.target.value);
                if (p) setProject(p);
              }}
              title="Проект"
              className="min-w-0 flex-1 cursor-pointer truncate rounded-md border-0 bg-transparent py-0.5 text-[11px] font-600 uppercase tracking-[0.08em] text-faint outline-none transition hover:text-ink"
            >
              {projects.map((p) => (
                <option key={p.key} value={p.key}>
                  {p.icon ? `${p.icon} ` : ""}{p.name}
                </option>
              ))}
            </select>
          ) : (
            <span className="text-[11px] font-600 uppercase tracking-[0.08em] text-faint">
              {project?.name ?? "Пространство"}
            </span>
          )}
          {user.canEdit && project?.myRole !== "reader" && (
            <span className="flex items-center gap-0.5">
              <ImportMarkdown
                title="Импорт из файла Markdown"
                className="cursor-pointer rounded p-0.5 text-faint transition hover:bg-line/70 hover:text-ink"
              >
                <Upload className="h-3.5 w-3.5" />
              </ImportMarkdown>
              <PasteMarkdown
                title="Вставить Markdown"
                className="rounded p-0.5 text-faint transition hover:bg-line/70 hover:text-ink"
              >
                <ClipboardPaste className="h-3.5 w-3.5" />
              </PasteMarkdown>
              <button
                type="button"
                data-tour="new-page"
                onClick={newRootPage}
                className="rounded p-0.5 text-faint transition hover:bg-line/70 hover:text-ink"
                title="Новая страница"
                disabled={busy}
              >
                <Plus className="h-3.5 w-3.5" />
              </button>
            </span>
          )}
        </div>

        <TreeNav />
      </nav>

      {/* корзина (редакторам) и админка (админам) */}
      {user.canEdit && (
        <div className="border-t border-line px-2 py-1.5">
          <button
            type="button"
            onClick={() => setTrashOpen(true)}
            className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-[13px] text-muted transition hover:bg-line/40 hover:text-ink"
          >
            <Trash2 className="h-4 w-4 text-faint" />
            Корзина
          </button>
          {user.isAdmin && (
            <button
              type="button"
              onClick={() => navigate("/admin")}
              className="flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left text-[13px] text-muted transition hover:bg-line/40 hover:text-ink"
            >
              <Settings className="h-4 w-4 text-faint" />
              Администрирование
            </button>
          )}
          <TrashDialog open={trashOpen} onOpenChange={setTrashOpen} />
        </div>
      )}

      {/* пользователь (из /api/me) */}
      <div className="flex items-center gap-2.5 border-t border-line px-4 py-3">
        <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-ink text-[12px] font-600 uppercase text-white">
          {(user.name || user.username || "?").trim().charAt(0)}
        </span>
        <span className="min-w-0 flex-1 truncate text-[13px] text-body" title={user.email || undefined}>
          {user.name || user.username || "Пользователь"}
        </span>
        {user.authEnabled && user.authenticated && (
          <button
            type="button"
            onClick={async () => {
              await fetch("/auth/logout", { method: "POST" });
              window.location.href = "/";
            }}
            className="ml-auto shrink-0 rounded-md p-1.5 text-faint transition hover:bg-line/60 hover:text-ink"
            title="Выйти"
          >
            <LogOut className="h-4 w-4" />
          </button>
        )}
      </div>
    </aside>
  );
}
