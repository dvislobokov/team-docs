import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ChevronRight,
  Copy,
  Download,
  FilePlus,
  FileText,
  MoreHorizontal,
  Pencil,
  Plus,
  Trash2,
} from "lucide-react";
import { createPage, deletePage } from "../api/pages";
import { duplicatePage, exportPageMarkdown, renamePage } from "../lib/pageActions";
import { planMove, type DropMode } from "../lib/tree";
import type { TreeItem } from "../lib/tree";
import { useAuth } from "../store/auth";
import { useConfirm } from "../store/confirm";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";

// --- DnD-контекст: общее состояние перетаскивания на всё дерево ---
interface DndCtx {
  draggingId: number | null;
  setDraggingId: (id: number | null) => void;
  performMove: (dragId: number, targetId: number, mode: DropMode) => void;
}
const Dnd = createContext<DndCtx | null>(null);
const useDnd = () => useContext(Dnd)!;

function Node({ item, depth }: { item: TreeItem; depth: number }) {
  const { id } = useParams();
  const navigate = useNavigate();
  const { reload } = useTree();
  const dnd = useDnd();
  const { canEdit } = useAuth();
  const confirm = useConfirm();
  const toast = useToast();

  const active = String(item.id) === id;
  const hasChildren = item.children.length > 0;
  const [open, setOpen] = useState(true);
  const [busy, setBusy] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [renameValue, setRenameValue] = useState(item.title);
  const [dropMode, setDropMode] = useState<DropMode | null>(null);
  const menuWrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (menuWrapRef.current && !menuWrapRef.current.contains(e.target as Node)) setMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setMenuOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  const addChild = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    if (busy) return;
    setBusy(true);
    try {
      const page = await createPage({ parentId: item.id, title: "Новая страница" });
      await reload();
      setOpen(true);
      navigate(`/pages/${page.id}`, { state: { isNew: true } });
    } finally {
      setBusy(false);
    }
  };

  const commitRename = async () => {
    setRenaming(false);
    const next = renameValue.trim();
    if (!next || next === item.title) return;
    await renamePage(item.id, next);
    await reload();
  };

  const doDuplicate = async () => {
    setMenuOpen(false);
    const copy = await duplicatePage(item.id);
    await reload();
    toast("Создана копия страницы", "success");
    navigate(`/pages/${copy.id}`);
  };

  const doExport = async () => {
    setMenuOpen(false);
    await exportPageMarkdown(item.id);
    toast("Экспортировано в Markdown", "success");
  };

  const doDelete = async () => {
    setMenuOpen(false);
    const ok = await confirm({
      title: "Удалить страницу?",
      message: `«${item.title || "Без названия"}» и все вложенные страницы будут удалены безвозвратно.`,
      confirmLabel: "Удалить",
      danger: true,
    });
    if (!ok) return;
    await deletePage(item.id);
    await reload();
    toast("Страница удалена", "success");
    if (active) navigate("/", { replace: true });
  };

  // --- drop-таргет ---
  const onDragOver = (e: React.DragEvent) => {
    if (dnd.draggingId == null || dnd.draggingId === item.id) return;
    e.preventDefault();
    e.stopPropagation();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const y = e.clientY - r.top;
    setDropMode(y < r.height * 0.25 ? "before" : y > r.height * 0.75 ? "after" : "inside");
  };
  const onDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const dragId = dnd.draggingId;
    const mode = dropMode;
    setDropMode(null);
    if (dragId != null && mode) {
      if (mode === "inside") setOpen(true);
      dnd.performMove(dragId, item.id, mode);
    }
  };

  return (
    <div className={depth > 0 ? "ml-3 border-l border-line pl-2" : undefined}>
      <div className="relative">
        {dropMode === "before" && (
          <div className="pointer-events-none absolute inset-x-1 -top-px z-10 h-0.5 rounded bg-accent" />
        )}
        {dropMode === "after" && (
          <div className="pointer-events-none absolute inset-x-1 -bottom-px z-10 h-0.5 rounded bg-accent" />
        )}

        <div
          draggable={!renaming && canEdit}
          onDragStart={(e) => {
            e.dataTransfer.effectAllowed = "move";
            e.dataTransfer.setData("text/plain", String(item.id));
            dnd.setDraggingId(item.id);
          }}
          onDragEnd={() => {
            dnd.setDraggingId(null);
            setDropMode(null);
          }}
          onDragOver={onDragOver}
          onDragLeave={() => setDropMode(null)}
          onDrop={onDrop}
          onContextMenu={(e) => {
            e.preventDefault();
            setMenuOpen(true);
          }}
          className={
            "group relative flex items-center gap-1.5 rounded-md px-2 py-1.5 transition-colors " +
            (dropMode === "inside" ? "ring-1 ring-inset ring-accent " : "") +
            (active
              ? "-ml-[9px] border-l-2 border-accent bg-accentSoft pl-3 font-500 text-ink"
              : "text-body hover:bg-line/60")
          }
        >
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              setOpen((v) => !v);
            }}
            className={
              "flex h-4 w-4 shrink-0 items-center justify-center text-faint " +
              (hasChildren ? "" : "invisible")
            }
            aria-label={open ? "Свернуть" : "Развернуть"}
          >
            <ChevronRight className={"h-3 w-3 transition-transform " + (open ? "rotate-90" : "")} />
          </button>

          {renaming ? (
            <>
              {item.icon ? (
                <span className="w-[18px] shrink-0 text-center text-[15px] leading-none">
                  {item.icon}
                </span>
              ) : (
                <FileText className="h-[18px] w-[18px] shrink-0 text-faint" />
              )}
              <input
                autoFocus
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onBlur={commitRename}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commitRename();
                  if (e.key === "Escape") setRenaming(false);
                }}
                className="min-w-0 flex-1 rounded border border-accent/50 bg-card px-1 py-0.5 text-[14.5px] text-ink outline-none"
              />
            </>
          ) : (
            <button
              type="button"
              onClick={() => navigate(`/pages/${item.id}`)}
              className="flex min-w-0 flex-1 items-center gap-2 text-left"
            >
              {item.icon ? (
                <span className="w-[18px] shrink-0 text-center text-[15px] leading-none">
                  {item.icon}
                </span>
              ) : (
                <FileText className="h-[18px] w-[18px] shrink-0 text-faint" />
              )}
              <span className="truncate text-[14.5px] font-500">
                {item.title || "Без названия"}
              </span>
            </button>
          )}

          {!renaming && (
            <div ref={menuWrapRef} className="relative flex shrink-0 items-center">
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  setMenuOpen((v) => !v);
                }}
                className="rounded p-0.5 text-faint opacity-0 transition hover:bg-line/70 hover:text-ink group-hover:opacity-100"
                title="Действия"
              >
                <MoreHorizontal className="h-3.5 w-3.5" />
              </button>
              {canEdit && (
                <button
                  type="button"
                  onClick={addChild}
                  className="rounded p-0.5 text-faint opacity-0 transition hover:bg-line/70 hover:text-ink group-hover:opacity-100"
                  title="Добавить под-страницу"
                  disabled={busy}
                >
                  <Plus className="h-3.5 w-3.5" />
                </button>
              )}

              {menuOpen && (
                <div className="animate-fade-in absolute right-0 top-full z-50 mt-1 w-52 overflow-hidden rounded-lg border border-line bg-card py-1 shadow-xl">
                  {canEdit && (
                    <>
                      <MenuItem icon={<Pencil className="h-4 w-4" />} onClick={() => { setMenuOpen(false); setRenameValue(item.title); setRenaming(true); }}>
                        Переименовать
                      </MenuItem>
                      <MenuItem icon={<FilePlus className="h-4 w-4" />} onClick={() => { setMenuOpen(false); addChild(); }}>
                        Добавить под-страницу
                      </MenuItem>
                      <MenuItem icon={<Copy className="h-4 w-4" />} onClick={doDuplicate}>
                        Дублировать
                      </MenuItem>
                    </>
                  )}
                  <MenuItem icon={<Download className="h-4 w-4" />} onClick={doExport}>
                    Экспорт в Markdown
                  </MenuItem>
                  {canEdit && (
                    <>
                      <div className="my-1 border-t border-line" />
                      <MenuItem icon={<Trash2 className="h-4 w-4" />} onClick={doDelete} danger>
                        Удалить
                      </MenuItem>
                    </>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {hasChildren && (
        <div
          className={
            "grid transition-[grid-template-rows] duration-200 ease-out " +
            (open ? "grid-rows-[1fr]" : "grid-rows-[0fr]")
          }
        >
          <div className="overflow-hidden">
            {item.children.map((child) => (
              <Node key={child.id} item={child} depth={depth + 1} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function MenuItem({
  icon,
  children,
  onClick,
  danger,
}: {
  icon: ReactNode;
  children: ReactNode;
  onClick: () => void;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex w-full items-center gap-2.5 px-3 py-1.5 text-left text-[13px] transition hover:bg-line/60 " +
        (danger ? "text-red-500" : "text-body")
      }
    >
      <span className="shrink-0 text-faint">{icon}</span>
      {children}
    </button>
  );
}

export function TreeNav() {
  const { tree, loading, nodes, moveTo } = useTree();
  const [draggingId, setDraggingId] = useState<number | null>(null);

  const performMove = (dragId: number, targetId: number, mode: DropMode) => {
    const plan = planMove(nodes, dragId, targetId, mode);
    if (!plan) return;
    void moveTo(dragId, plan.parentId, plan.position);
  };

  if (loading) {
    return <div className="px-2 py-2 text-[13px] text-faint">Загрузка…</div>;
  }
  if (tree.length === 0) {
    return <div className="px-2 py-2 text-[13px] text-faint">Пока нет страниц</div>;
  }
  return (
    <Dnd.Provider value={{ draggingId, setDraggingId, performMove }}>
      <div className="mt-0.5">
        {tree.map((item) => (
          <Node key={item.id} item={item} depth={0} />
        ))}
      </div>
    </Dnd.Provider>
  );
}
