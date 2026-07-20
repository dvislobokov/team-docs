// Типы DTO бэкенда (см. internal/pages/handler.go). Все поля — camelCase.
// `id`/`parentId` на бэке int64; держим как number — для малых инсталляций
// это безопасно (значения далеко от Number.MAX_SAFE_INTEGER).

import type { PartialBlock } from "@blocknote/core";

/** Контент страницы — документ BlockNote (массив блоков). */
export type PageContent = PartialBlock[];

export interface Page {
  id: number;
  parentId: number | null;
  title: string;
  icon: string; // emoji, "" = дефолтная иконка
  content: PageContent;
  position: number;
  version: number;
  updatedAt: string; // RFC3339
  /** Имя последнего редактора; null для страниц без авторства (старые, MCP). */
  updatedByName: string | null;
  /** Право правки текущего пользователя в проекте страницы (§10). */
  canEdit: boolean;
  tags: string[];
  /** Страница-шаблон (служебный раздел, скрыта из дерева/поиска). */
  isTemplate: boolean;
  /** Проект страницы; сайдбар переключается на него при открытии. */
  projectId: number;
}

/** Узел плоского списка дерева (GET /api/pages/tree). */
export interface PageTreeNode {
  id: number;
  parentId: number | null;
  title: string;
  icon: string;
  position: number;
}

export interface CreatePageRequest {
  parentId: number | null;
  title: string;
  /** Проект для корневых страниц (по умолчанию 'main'). */
  projectId?: number;
  /** Создать страницу-шаблон (всегда корневая, вне дерева). */
  template?: boolean;
  /** Создать страницу из шаблона (копия контента/иконки/тегов). */
  templateId?: number;
}

/** Избранная страница (GET /api/favorites). */
export interface FavoriteItem {
  id: number;
  title: string;
  icon: string;
  projectId: number;
  createdAt: string;
}

/** Шаблон проекта (GET /api/templates). */
export interface TemplateItem {
  id: number;
  title: string;
  icon: string;
  updatedAt: string;
}

export interface UpdatePageRequest {
  title: string;
  icon: string;
  content: PageContent;
  version: number;
  /** Полный список тегов (замена); null/отсутствие — не менять. */
  tags?: string[];
}

export interface MovePageRequest {
  parentId: number | null;
  position: number;
}

export interface Revision {
  id: number;
  version: number;
  title: string;
  createdAt: string;
  authorName: string | null;
}

/** Ревизия с контентом (GET /pages/:id/revisions/:revId) — для отката. */
export interface RevisionDetail extends Revision {
  content: PageContent;
}

export interface SearchHit {
  id: number;
  title: string;
  icon: string;
  snippet: string; // HTML-фрагмент ts_headline
}

/** Элемент корзины (корень удалённого поддерева). */
export interface TrashItem {
  id: number;
  title: string;
  icon: string;
  deletedAt: string;
}

export interface UploadResponse {
  id: string; // UUID
  url: string; // /api/files/<uuid>
}

/** Текущий пользователь (GET /api/me). В открытом режиме — devUser. */
export interface Me {
  authenticated: boolean;
  canEdit: boolean;
  username: string;
  name: string;
  email: string;
  groups: string[] | null;
  /** Роль встроенной авторизации: reader | editor | admin. */
  role?: string;
  isAdmin?: boolean;
  /** Включена ли авторизация на бэке (для показа кнопки выхода). */
  authEnabled?: boolean;
}

/** OAuth-провайдер (GET /auth/providers). */
export interface AuthProvider {
  key: string;
  label: string;
}

/** Токены цвета «R G B» (как значения CSS-переменных --c-*). */
export interface PaletteColors {
  paper: string;
  card: string;
  ink: string;
  body: string;
  muted: string;
  faint: string;
  line: string;
  accent: string;
  accentSoft: string;
  marker: string;
}

/** Цветовая схема: id, подпись и палитра для светлой/тёмной темы. */
export interface ThemePreset {
  id: string;
  label: string;
  palette: { light: PaletteColors; dark: PaletteColors };
}

/** Брендинг и набор цветовых схем, отдаваемые бэком (GET /api/branding). */
export interface Branding {
  appName: string;
  workspaceName: string;
  monogram: string;
  /** id схемы по умолчанию (задаётся на бэке; пользователь может сменить в UI). */
  defaultTheme: string;
  themes: ThemePreset[];
}
