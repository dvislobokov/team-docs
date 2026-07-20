import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import type { PartialBlock } from "@blocknote/core";
import { Download, LayoutTemplate, Pencil, Star, Trash2 } from "lucide-react";
import { createPage, deletePage, getPage, getRevision, updatePage } from "../api/pages";
import { ConflictError } from "../api/client";
import type { Page, PageTreeNode } from "../api/types";
import { extractHeadings, isEmptyDoc, readingMinutes } from "../lib/blocks";
import { useContentWidth, WIDTH_CLASS } from "../lib/contentWidth";
import { readingLabel, relativeTime } from "../lib/format";
import { exportPageMarkdown } from "../lib/pageActions";
import { pushRecent } from "../lib/recents";
import { useTheme } from "../lib/theme";
import { pathToNode } from "../lib/tree";
import { useAuth } from "../store/auth";
import { useConfirm } from "../store/confirm";
import { useFavorites } from "../store/favorites";
import { useTemplates } from "../store/templates";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";
import { ChildCards } from "./ChildCards";
import { EmojiButton } from "./EmojiButton";
import { PageEditor } from "./PageEditor";
import { PageTags } from "./PageTags";
import { RevisionsDialog } from "./RevisionsDialog";
import { RightRail } from "./RightRail";
import { ShareButton } from "./ShareButton";
import { Topbar, type Crumb } from "./Topbar";

type SaveState = "idle" | "saving" | "saved" | "conflict" | "error";

const SAVE_DEBOUNCE_MS = 800;

export function PageScreen() {
  const { id } = useParams();
  const pageId = Number(id);
  const navigate = useNavigate();
  const location = useLocation();
  const theme = useTheme();
  const width = useContentWidth();
  const { nodes, reload, projects, project, setProject } = useTree();
  const user = useAuth();
  const { canEdit } = user;
  const { isFavorite, toggle: toggleFavorite, reload: reloadFavorites } = useFavorites();
  const { reload: reloadTemplates } = useTemplates();
  const confirm = useConfirm();
  const toast = useToast();

  const [page, setPage] = useState<Page | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState("");
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [historyOpen, setHistoryOpen] = useState(false);
  // Меняется при внешней подмене контента (откат/перезагрузка) — форсирует
  // пересоздание BlockNote (он читает initialContent только при монтировании).
  const [editorEpoch, setEditorEpoch] = useState(0);

  // Актуальные значения для отложенного сохранения (без пересоздания таймера).
  const pageRef = useRef<Page | null>(null);
  const titleRef = useRef("");
  const iconRef = useRef("");
  const tagsRef = useRef<string[]>([]);
  const contentRef = useRef<PartialBlock[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const titleInputRef = useRef<HTMLInputElement>(null);

  // Заголовки для оглавления — мемоизируем, чтобы observer в RightRail не
  // пересоздавался на каждый рендер.
  const pageHeadings = useMemo(() => (page ? extractHeadings(page.content) : []), [page]);

  // Загрузка страницы при смене :id. Прошлую страницу НЕ обнуляем — иначе топбар
  // с кнопками мигает; держим её до прихода новой, а контент плавно сменяется.
  useEffect(() => {
    if (!Number.isFinite(pageId)) return;
    const ctrl = new AbortController();
    setLoadError(null);
    setEditing(false);
    setSaveState("idle");
    getPage(pageId, ctrl.signal)
      .then((p) => {
        setPage(p);
        setTitle(p.title);
        pageRef.current = p;
        titleRef.current = p.title;
        iconRef.current = p.icon;
        tagsRef.current = p.tags ?? [];
        contentRef.current = p.content;
        pushRecent(p.id);
        // Новая страница (создана только что) — сразу в режим правки, фокус на
        // заголовке с выделением, чтобы можно было переименовать.
        if (location.state?.isNew) {
          setEditing(true);
          requestAnimationFrame(() => {
            titleInputRef.current?.focus();
            titleInputRef.current?.select();
          });
          navigate(location.pathname, { replace: true, state: {} });
        }
      })
      .catch((e) => {
        if (ctrl.signal.aborted) return;
        setLoadError(e instanceof Error ? e.message : "Не удалось загрузить страницу");
      });
    return () => ctrl.abort();
    // location.state читаем один раз на смену страницы — намеренно не в зависимостях.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pageId]);

  // Страница из другого проекта (избранное, недавние, прямая ссылка) —
  // переключаем сайдбар на её проект, чтобы дерево и крошки совпадали.
  // Ручное переключение проекта пользователем не откатываем: эффект зависит
  // только от страницы и списка проектов.
  useEffect(() => {
    if (!page?.projectId || page.isTemplate) return;
    if (project && project.id === page.projectId) return;
    const target = projects.find((p) => p.id === page.projectId);
    if (target) setProject(target);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page?.id, page?.projectId, projects]);

  const doSave = useCallback(async () => {
    const p = pageRef.current;
    if (!p) return;
    setSaveState("saving");
    try {
      const updated = await updatePage(p.id, {
        title: titleRef.current,
        icon: iconRef.current,
        content: contentRef.current,
        version: p.version,
        tags: tagsRef.current,
      });
      pageRef.current = updated;
      setPage(updated);
      setSaveState("saved");
      void reload(); // заголовок мог измениться — обновим дерево
      if (updated.isTemplate) void reloadTemplates();
      void reloadFavorites(); // заголовок в секции «Избранное»
    } catch (e) {
      setSaveState(e instanceof ConflictError ? "conflict" : "error");
    }
  }, [reload, reloadTemplates, reloadFavorites]);

  const scheduleSave = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => void doSave(), SAVE_DEBOUNCE_MS);
  }, [doSave]);

  // Сохранить оставшееся при уходе со страницы/размонтировании.
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  const onTitleChange = (value: string) => {
    setTitle(value);
    titleRef.current = value;
    scheduleSave();
  };

  const onContentChange = (content: PartialBlock[]) => {
    contentRef.current = content;
    scheduleSave();
  };

  // Теги сохраняются сразу (дискретное действие, не дебаунсим).
  const onTagsChange = (tags: string[]) => {
    tagsRef.current = tags;
    setPage((p) => (p ? { ...p, tags } : p));
    void doSave();
  };

  // Смена иконки сохраняется сразу (дискретное действие, не дебаунсим).
  const onIconChange = (emoji: string) => {
    iconRef.current = emoji;
    setPage((p) => (p ? { ...p, icon: emoji } : p));
    void doSave();
  };

  const remove = async () => {
    if (!page) return;
    const ok = await confirm({
      title: "Удалить страницу?",
      message: `«${page.title || "Без названия"}» и все вложенные страницы отправятся в корзину (хранится 30 дней).`,
      confirmLabel: "Удалить",
      danger: true,
    });
    if (!ok) return;
    await deletePage(page.id);
    await reload();
    if (page.isTemplate) void reloadTemplates();
    void reloadFavorites();
    toast("Страница перемещена в корзину", "success");
    navigate("/", { replace: true });
  };

  // «Создать из шаблона»: копия шаблона обычной корневой страницей проекта.
  const createFromTemplate = async () => {
    if (!page) return;
    const created = await createPage({ parentId: null, title: "", templateId: page.id });
    await reload();
    navigate(`/pages/${created.id}`, { state: { isNew: true } });
  };

  const applyLoaded = (p: Page) => {
    setPage(p);
    setTitle(p.title);
    pageRef.current = p;
    titleRef.current = p.title;
    iconRef.current = p.icon;
    tagsRef.current = p.tags ?? [];
    contentRef.current = p.content;
    setEditorEpoch((e) => e + 1); // форсируем перечитку контента редактором
  };

  const reloadFromServer = () => {
    setSaveState("idle");
    setEditing(false);
    getPage(pageId).then(applyLoaded).catch(() => undefined);
  };

  // Откат к версии: тянем её контент и сохраняем как новую версию (обычный PUT).
  const restoreRevision = async (revId: number) => {
    const p = pageRef.current;
    if (!p) return;
    const rev = await getRevision(p.id, revId);
    const updated = await updatePage(p.id, {
      title: rev.title,
      icon: p.icon,
      content: rev.content,
      version: p.version,
    });
    applyLoaded(updated);
    setHistoryOpen(false);
    void reload();
    toast("Версия восстановлена", "success");
  };

  const doExport = async () => {
    if (!page) return;
    await exportPageMarkdown(page.id);
    toast("Экспортировано в Markdown", "success");
  };

  if (loadError) {
    return (
      <>
        <Topbar />
        <div className="mx-auto max-w-[560px] px-8 py-32 text-center text-muted">
          {loadError}
        </div>
      </>
    );
  }

  if (!page) {
    return <Topbar crumbs={crumbsFor(nodes, pageId)} />;
  }

  const headings = pageHeadings;
  // Право правки: глобальное И проектное (роль в проекте страницы, §10).
  const pageEditable = canEdit && page.canEdit !== false;

  // Дочерние страницы для карточек-галереи (сценарий страницы-контейнера).
  const childNodes = nodes
    .filter((n) => n.parentId === page.id)
    .sort((a, b) => a.position - b.position || a.id - b.id);
  const emptyContainer = !editing && isEmptyDoc(page.content) && childNodes.length > 0;

  const favorited = isFavorite(page.id);
  const actions = (
    <>
      {page.isTemplate && pageEditable && (
        <button
          type="button"
          onClick={() => void createFromTemplate()}
          className="flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-[13px] text-muted transition hover:bg-line/60 hover:text-ink"
          title="Создать страницу из этого шаблона"
        >
          <LayoutTemplate className="h-3.5 w-3.5" />
          Создать из шаблона
        </button>
      )}
      {user.authenticated && !page.isTemplate && (
        <button
          type="button"
          onClick={() => void toggleFavorite(page.id)}
          className="rounded-md p-1.5 text-muted transition hover:bg-line/60 hover:text-ink"
          title={favorited ? "Убрать из избранного" : "В избранное"}
        >
          <Star
            className={
              "h-[17px] w-[17px] " + (favorited ? "fill-amber-400 text-amber-400" : "")
            }
          />
        </button>
      )}
      {pageEditable && (
        <button
          type="button"
          onClick={() => setHistoryOpen(true)}
          className="rounded-md px-2.5 py-1.5 text-[13px] text-muted transition hover:bg-line/60"
        >
          История
        </button>
      )}
      <button
        type="button"
        onClick={doExport}
        className="rounded-md p-1.5 text-muted transition hover:bg-line/60 hover:text-ink"
        title="Экспорт в Markdown"
      >
        <Download className="h-[17px] w-[17px]" />
      </button>
      {pageEditable && (
        <button
          type="button"
          onClick={remove}
          className="rounded-md p-1.5 text-muted transition hover:bg-line/60 hover:text-ink"
          title="Удалить страницу"
        >
          <Trash2 className="h-[17px] w-[17px]" />
        </button>
      )}
      <ShareButton />
      {pageEditable && (
        <button
          type="button"
          onClick={() => setEditing((v) => !v)}
          className={
            "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-[13px] font-500 transition " +
            (editing
              ? "border border-line text-body hover:bg-line/60"
              : "bg-accent text-white hover:bg-accent/90")
          }
        >
          {!editing && <Pencil className="h-3.5 w-3.5" />}
          {editing ? "Готово" : "Редактировать"}
        </button>
      )}
    </>
  );

  return (
    <>
      <Topbar crumbs={crumbsFor(nodes, pageId)} actions={actions} />

      {saveState === "conflict" && (
        <div className="border-b border-amber-400/30 bg-amber-400/10 px-8 py-2.5 text-[13px] text-ink">
          Страницу изменил кто-то ещё. Ваши правки не сохранены.{" "}
          <button className="font-600 underline" onClick={reloadFromServer}>
            Загрузить свежую версию
          </button>
        </div>
      )}

      <section key={page.id} className="animate-page-in py-8 md:py-12">
        {/* Контент центрируем в свободном пространстве, правый список — к краю. */}
        <div className="flex w-full gap-8 px-5 md:px-10 xl:px-16">
          <div className="flex min-w-0 flex-1 justify-center">
            <article className={"w-full " + WIDTH_CLASS[width]}>
          <div className="flex items-center gap-3">
            <EmojiButton
              value={page.icon}
              onChange={onIconChange}
              disabled={!pageEditable}
              fallback="📄"
              triggerClassName="flex h-[48px] w-[48px] shrink-0 items-center justify-center rounded-xl text-[36px] leading-none transition hover:bg-line/50 md:h-[60px] md:w-[60px] md:text-[46px]"
            />

            {editing ? (
              <input
                ref={titleInputRef}
                value={title}
                onChange={(e) => onTitleChange(e.target.value)}
                placeholder="Без названия"
                className="min-w-0 flex-1 bg-transparent font-display text-[32px] font-500 leading-[1.1] tracking-[-0.01em] text-ink outline-none placeholder:text-faint md:text-[46px] md:leading-[1.05]"
              />
            ) : (
              <h1 className="min-w-0 truncate font-display text-[32px] font-500 leading-[1.1] tracking-[-0.01em] text-ink md:text-[46px] md:leading-[1.05]">
                {page.title || "Без названия"}
              </h1>
            )}
          </div>

          <p className="mt-3 font-mono text-[12px] text-muted">
            {page.isTemplate && (
              <span className="mr-2 inline-flex items-center gap-1 rounded bg-accent-soft px-1.5 py-0.5 text-[11px] uppercase tracking-[0.06em] text-accent">
                <LayoutTemplate className="h-3 w-3" />
                Шаблон
              </span>
            )}
            Обновлено · {relativeTime(page.updatedAt)}
            {page.updatedByName ? ` · ${page.updatedByName}` : ""} ·{" "}
            {readingLabel(readingMinutes(page.content))}
            {editing && <SaveIndicator state={saveState} />}
          </p>

          <PageTags tags={page.tags ?? []} editable={pageEditable} onChange={onTagsChange} />

          {/* Пустая страница-контейнер: вместо пустого редактора — карточки
              дочерних страниц. Иначе — редактор, а карточки (если дети есть)
              показываем секцией ниже. */}
          {emptyContainer ? (
            <div className="mt-8">
              <ChildCards nodes={childNodes} allNodes={nodes} parentId={page.id} />
            </div>
          ) : (
            <>
              <div className="mt-8" data-tour="editor">
                <PageEditor
                  key={`${page.id}-${editorEpoch}`}
                  initialContent={page.content}
                  editable={editing && pageEditable}
                  theme={theme}
                  onChange={onContentChange}
                />
              </div>
              {childNodes.length > 0 && (
                <div className="mt-8 border-t border-line pt-6">
                  <ChildCards nodes={childNodes} allNodes={nodes} parentId={page.id} />
                </div>
              )}
            </>
          )}
            </article>
          </div>
          <RightRail headings={headings} />
        </div>
      </section>

      <RevisionsDialog
        pageId={page.id}
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        onRestore={restoreRevision}
      />
    </>
  );
}

function crumbsFor(nodes: PageTreeNode[], pageId: number): Crumb[] {
  return pathToNode(nodes, pageId).map((n) => ({ id: n.id, title: n.title, icon: n.icon }));
}

function SaveIndicator({ state }: { state: SaveState }) {
  const label =
    state === "saving"
      ? " · сохранение…"
      : state === "saved"
        ? " · сохранено"
        : state === "error"
          ? " · ошибка сохранения"
          : "";
  if (!label) return null;
  return <span className="text-faint">{label}</span>;
}
