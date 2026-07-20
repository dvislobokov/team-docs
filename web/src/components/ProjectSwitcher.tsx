import { useEffect, useRef, useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { useTree } from "../store/tree";

// Переключатель проекта в сайдбаре: кастомный дропдаун вместо нативного
// <select> — тот рисуется ОС и игнорирует тему (белые опции в тёмном UI).
export function ProjectSwitcher() {
  const { projects, project, setProject } = useTree();
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (projects.length === 0) {
    return (
      <span className="text-[11px] font-600 uppercase tracking-[0.08em] text-faint">
        Пространство
      </span>
    );
  }

  return (
    <div ref={wrapRef} className="relative min-w-0 flex-1">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        title="Сменить проект"
        className="flex w-full min-w-0 items-center gap-1 rounded-md border border-transparent py-0.5 pl-1 pr-1 text-left transition hover:border-line"
      >
        <span className="min-w-0 flex-1 truncate text-[11px] font-600 uppercase tracking-[0.08em] text-faint">
          {project?.icon ? `${project.icon} ` : ""}{project?.name ?? "Пространство"}
        </span>
        <ChevronsUpDown className="h-3 w-3 shrink-0 text-faint" />
      </button>

      {open && (
        <div className="animate-fade-in absolute left-0 top-full z-50 mt-1 w-56 overflow-hidden rounded-xl border border-line bg-card p-1.5 shadow-xl">
          {projects.map((p) => {
            const on = p.id === project?.id;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => {
                  setProject(p);
                  setOpen(false);
                }}
                className={
                  "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-[13px] transition " +
                  (on ? "bg-accentSoft text-accent" : "text-body hover:bg-line/50")
                }
              >
                <span className="w-4 shrink-0 text-center text-[14px] leading-none">
                  {p.icon || "📁"}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-500">{p.name}</span>
                  <span className="block truncate font-mono text-[10px] text-faint">{p.key}</span>
                </span>
                {on && <Check className="h-3.5 w-3.5 shrink-0" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
