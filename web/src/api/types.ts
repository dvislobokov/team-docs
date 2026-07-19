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
}

export interface UpdatePageRequest {
  title: string;
  icon: string;
  content: PageContent;
  version: number;
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
