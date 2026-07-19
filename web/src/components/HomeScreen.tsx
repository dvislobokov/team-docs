import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Clock, FolderOpen, Plus, Search } from "lucide-react";
import { createPage, getRecentPages, type RecentPage } from "../api/pages";
import { relativeTime } from "../lib/format";
import { getRecents } from "../lib/recents";
import { useAuth } from "../store/auth";
import { useBranding } from "../store/branding";
import { usePalette } from "../store/palette";
import { useTree } from "../store/tree";
import { EmptyState } from "./EmptyState";
import { Topbar } from "./Topbar";

// Главная-хаб (ROADMAP §11): поиск по центру, карточки проектов,
// «продолжить чтение» (локальные recents) и «недавно обновлено» (по серверу).
export function HomeScreen() {
  const { tree, nodes, loading, projects, project, setProject } = useTree();
  const { setOpen: setPaletteOpen } = usePalette();
  const branding = useBranding();
  const user = useAuth();
  const navigate = useNavigate();
  const [recentUpdated, setRecentUpdated] = useState<RecentPage[] | null>(null);

  useEffect(() => {
    const ctrl = new AbortController();
    getRecentPages(ctrl.signal)
      .then(setRecentUpdated)
      .catch(() => setRecentUpdated([]));
    return () => ctrl.abort();
  }, []);

  if (loading) return <Topbar />;

  // Совсем пустая инсталляция — прежний «чистый лист».
  if (tree.length === 0 && (recentUpdated?.length ?? 0) === 0 && projects.length <= 1) {
    return (
      <>
        <Topbar />
        <EmptyState />
      </>
    );
  }

  // «Продолжить чтение»: локальные recents, найденные в текущем дереве.
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const continueReading = getRecents()
    .map((id) => byId.get(id))
    .filter((n): n is NonNullable<typeof n> => Boolean(n))
    .slice(0, 6);

  const newPage = async () => {
    const p = await createPage({ parentId: null, title: "Новая страница", projectId: project?.id });
    navigate(`/pages/${p.id}`, { state: { isNew: true } });
  };

  return (
    <>
      <Topbar />
      <section className="animate-page-in mx-auto w-full max-w-[880px] px-6 py-10 md:py-16">
        {/* герой: приветствие + поиск */}
        <h1 className="text-center font-display text-[32px] font-500 text-ink md:text-[40px]">
          {branding.appName}
        </h1>
        <button
          type="button"
          onClick={() => setPaletteOpen(true)}
          className="mx-auto mt-5 flex w-full max-w-[560px] items-center gap-3 rounded-xl border border-line bg-card px-4 py-3 text-left text-[15px] text-muted shadow-sm transition hover:border-faint hover:shadow"
        >
          <Search className="h-[18px] w-[18px] text-faint" />
          Поиск по страницам…
          <span className="ml-auto font-mono text-[12px] text-faint">⌘K</span>
        </button>

        {/* проекты */}
        {projects.length > 1 && (
          <HomeSection icon={<FolderOpen className="h-4 w-4" />} title="Проекты">
            <div className="grid grid-cols-2 gap-2.5 md:grid-cols-3">
              {projects.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => {
                    setProject(p);
                    window.scrollTo(0, 0);
                  }}
                  className={
                    "rounded-xl border px-4 py-3 text-left transition hover:border-faint hover:shadow-sm " +
                    (p.id === project?.id ? "border-accent/50 bg-accentSoft/40" : "border-line bg-card")
                  }
                >
                  <div className="text-[18px] leading-none">{p.icon || "📁"}</div>
                  <div className="mt-1.5 truncate text-[14px] font-500 text-ink">{p.name}</div>
                  <div className="truncate font-mono text-[11px] text-faint">{p.key}</div>
                </button>
              ))}
            </div>
          </HomeSection>
        )}

        {/* продолжить чтение */}
        {continueReading.length > 0 && (
          <HomeSection icon={<Clock className="h-4 w-4" />} title="Продолжить чтение">
            <div className="flex flex-wrap gap-2">
              {continueReading.map((n) => (
                <button
                  key={n.id}
                  type="button"
                  onClick={() => navigate(`/pages/${n.id}`)}
                  className="flex items-center gap-2 rounded-lg border border-line bg-card px-3 py-2 text-[13px] text-body transition hover:border-faint"
                >
                  <span>{n.icon || "📄"}</span>
                  <span className="max-w-[220px] truncate">{n.title || "Без названия"}</span>
                </button>
              ))}
            </div>
          </HomeSection>
        )}

        {/* недавно обновлено (по всем доступным проектам, с автором) */}
        {recentUpdated && recentUpdated.length > 0 && (
          <HomeSection icon={<Clock className="h-4 w-4" />} title="Недавно обновлено">
            <div className="overflow-hidden rounded-xl border border-line">
              {recentUpdated.map((r) => (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => navigate(`/pages/${r.id}`)}
                  className="flex w-full items-center gap-3 border-b border-line/60 bg-card px-4 py-2.5 text-left transition last:border-0 hover:bg-line/40"
                >
                  <span className="shrink-0 text-[16px] leading-none">{r.icon || "📄"}</span>
                  <span className="min-w-0 flex-1 truncate text-[14px] text-ink">
                    {r.title || "Без названия"}
                  </span>
                  <span className="shrink-0 font-mono text-[12px] text-muted">
                    {relativeTime(r.updatedAt)}
                    {r.updatedByName ? ` · ${r.updatedByName}` : ""}
                  </span>
                </button>
              ))}
            </div>
          </HomeSection>
        )}

        {user.canEdit && project?.myRole !== "reader" && (
          <div className="mt-10 text-center">
            <button
              type="button"
              onClick={newPage}
              className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-4 py-2 text-[14px] font-500 text-white transition hover:bg-accent/90"
            >
              <Plus className="h-4 w-4" /> Новая страница
            </button>
          </div>
        )}
      </section>
    </>
  );
}

function HomeSection({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="mt-10">
      <h2 className="mb-3 flex items-center gap-2 text-[13px] font-600 uppercase tracking-[0.06em] text-faint">
        {icon} {title}
      </h2>
      {children}
    </div>
  );
}
