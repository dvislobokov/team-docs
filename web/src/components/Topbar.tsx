import { Fragment, type ReactNode } from "react";
import { Menu } from "lucide-react";
import { useBranding } from "../store/branding";
import { useSidebar } from "../store/sidebar";
import { DisplaySettings } from "./DisplaySettings";

export interface Crumb {
  id: number;
  title: string;
  icon?: string;
}

interface TopbarProps {
  crumbs?: Crumb[];
  /** Правая зона (кнопки действий конкретного экрана). */
  actions?: ReactNode;
}

// Верхняя липкая панель: хлебные крошки слева, переключатель темы и действия
// справа. По макету — полупрозрачный фон с backdrop-blur.
export function Topbar({ crumbs = [], actions }: TopbarProps) {
  const { open, toggle } = useSidebar();
  const { appName } = useBranding();
  return (
    <div className="sticky top-0 z-20 flex items-center gap-2 border-b border-line/70 bg-paper/80 px-4 py-3 backdrop-blur md:px-8">
      <button
        type="button"
        onClick={toggle}
        className={
          "-ml-1 shrink-0 rounded-md p-1.5 text-muted transition hover:bg-line/60 hover:text-ink " +
          (open ? "md:hidden" : "")
        }
        title="Показать панель"
      >
        <Menu className="h-[18px] w-[18px]" />
      </button>
      <div className="flex min-w-0 items-center gap-2 font-mono text-[11px] uppercase tracking-[0.06em] text-muted">
        {crumbs.length === 0 ? (
          <span className="text-ink">{appName}</span>
        ) : (
          crumbs.map((c, i) => (
            <Fragment key={c.id}>
              {i > 0 && <span className="text-faint">/</span>}
              <span className={i === crumbs.length - 1 ? "truncate text-ink" : "truncate"}>
                {c.icon ? <span className="mr-1">{c.icon}</span> : null}
                {c.title || "Без названия"}
              </span>
            </Fragment>
          ))
        )}
      </div>

      <div className="ml-auto flex items-center gap-1.5">
        {actions}
        <DisplaySettings />
      </div>
    </div>
  );
}
