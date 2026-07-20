# team-docs

Внутренний аналог Confluence для команды: страницы и подстраницы в редакторе
[BlockNote](https://www.blocknotejs.org/), дерево навигации, полнотекстовый
поиск, история версий, диаграммы и графики, светлая/тёмная темы и цветовые схемы.
Весь фронт встраивается в **один Go-бинарь**.

![Гайд для новичков — страница в редакторе](docs/screenshots/hero.png)

## Возможности

**Редактор (BlockNote):**
- Страницы и подстраницы, дерево с **drag-n-drop** (перенос и вложенность),
  инлайн-переименование, дублирование, удаление.
- Меню вставки по `/` и `{{` (в стиле Confluence): заголовки, списки,
  **чек-листы**, таблицы, цитаты, **сворачиваемые (toggle)** блоки, разделители.
- **Выноски** (info / success / warning / danger / note), **статус-бейджи**,
  **@-упоминания** страниц (связка с деревом), **многоколоночность**.
- **Блок кода** — «тёплый терминал», подстраивается под тему (светлый фон/тёмный
  текст и наоборот).
- **Mermaid**: блок-схемы, sequence, gantt, class, ER, state, mindmap — и
  **графики** (столбцы/линия через `xychart`, круговая). Рендер под тему, lazy-load.
- **OpenAPI**: блок с полноценным рендером спецификации через **Swagger UI**
  (операции, схемы, примеры, Try it out, разрешение `$ref`). Источник —
  URL или вставленный YAML/JSON. Тема-совместимый (light/dark), lazy-load.
- **Медиа**: загрузка картинок/файлов — содержимое хранится в БД (BYTEA).

**Навигация и поиск:**
- Полнотекстовый поиск по `⌘K` с подсветкой совпадений и недавними.
- Хлебные крошки, оглавление «На этой странице» со **scroll-spy**.
- Страницы-контейнеры показывают карточки вложенных страниц.

**Оформление (отдаётся бэкендом, `GET /api/branding`):**
- Светлая/тёмная тема + **7 цветовых схем**: Бумага, Dracula, Nord, Tokyo Night,
  Gruvbox, Solarized, Catppuccin. Ширина контента (узкая/средняя/широкая).
  Выбор сохраняется в `localStorage`.
- Брендинг (название/монограмма/палитра) конфигурируется без пересборки фронта.

**Прочее:** история версий и откат · импорт/экспорт Markdown (файл и вставка) ·
полный **бэкап/восстановление БД** одним JSON-файлом (меню оформления → «Данные») ·
приветственный тур · автосейв с optimistic-lock (409 при конфликте) ·
тосты и стилизованные подтверждения · адаптивный сворачиваемый сайдбар ·
локализация редактора на русский.

**Авторизация (опциональна):** проверка JWT от IAM-прокси (RS256 по JWKS либо
HS256 по секрету). По умолчанию выключена — открытый режим для разработки. При
включении — разделение **чтение / запись**: смотреть могут все (в т.ч. анонимно,
если `publicRead`), а **редактировать — только авторизованные** (любой вошедший
или только члены `editorGroups`). UI-элементы правки скрываются для читателей,
бэкенд отдаёт `403`/`401` на запись.

| Поиск (`⌘K`) | Темы и оформление |
|---|---|
| ![Поиск](docs/screenshots/search.png) | ![Темы](docs/screenshots/themes.png) |

## Стек

- **Фронт:** React 19 + TypeScript + Tailwind CSS + Radix + BlockNote
  (`+ xl-multi-column`, `mermaid`, `swagger-ui-dist` для OpenAPI, `floating-ui`) на Vite.
  Локальные шрифты PT Serif / Inter / JetBrains Mono.
- **Бэк:** Go + [Echo](https://echo.labstack.com/), JWT ([golang-jwt](https://github.com/golang-jwt/jwt)).
- **БД:** PostgreSQL, доступ через [sqlc](https://sqlc.dev/) + `pgx/v5`.
- **Конфиг:** [sconf](https://github.com/dvislobokov/sconf) (YAML + env).
- **Логи:** [srog](https://github.com/dvislobokov/srog) (Serilog-стиль).

## Требования

- Go 1.25+ (подтягивается автоматически при `GOTOOLCHAIN=auto`)
- Node.js 20+ / npm
- PostgreSQL 14+
- (опц.) [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html),
  [task](https://taskfile.dev/installation/) или `make`

## Конфигурация

Настройки читаются из `appsettings.yaml` (опционально) и переменных окружения
с префиксом `TEAMDOCS_` (вложенность — через `__`, напр. `TEAMDOCS_HTTP__PORT=9000`).

```yaml
http:
  host: 0.0.0.0
  port: 8080
db:
  dsn: "postgres://user:pass@localhost:5432/teamdocs?sslmode=disable"
maxUploadBytes: 20971520 # лимит размера файла; сами файлы хранятся в БД (BYTEA)

# Брендинг/палитра (всё опционально — иначе дефолты).
branding:
  appName: "team-docs"
  monogram: "td"
  theme: default   # default | dracula | nord | tokyo | gruvbox | solarized | catppuccin

# Авторизация (по умолчанию выключена). Приложение за IAM-прокси, который
# прокидывает JWT; здесь мы его валидируем.
auth:
  enabled: false
  # --- режим 1: за IAM-прокси (валидация JWT из заголовка) ---
  # jwksUrl: "http://keycloak:8080/realms/teamdocs/protocol/openid-connect/certs"
  # issuer:  "http://keycloak:8080/realms/teamdocs"
  # hmacSecret: ""      # альтернатива JWKS для HS256
  publicRead: true       # анонимное чтение (GET); запись всегда требует вход
  editorGroups: []       # пусто → писать может любой вошедший; иначе — только
                         # члены этих групп/ролей (claim groups или realm_access.roles)
  # --- режим 2: встроенный OAuth-логин (без IAM; работают одновременно) ---
  # publicUrl: "https://docs.example.com"   # для redirect_uri
  # sessionSecret: "случайная-строка"        # подпись cookie-сессий (задать в проде!)
  # defaultRole: editor                      # роль новых пользователей: reader | editor
  # adminEmails: ["boss@example.com"]        # бутстрап админов
  # providers:
  #   google: { clientId: "...", clientSecret: "..." }
  #   yandex: { clientId: "...", clientSecret: "..." }
  #   vk:     { clientId: "...", clientSecret: "..." }
  #   apple:  { clientId: "...", teamId: "...", keyId: "...", privateKey: "..." } # .p8
  #   oidc:                                    # любой OIDC IdP: Keycloak, Authentik, Dex…
  #     label: "Keycloak"
  #     issuer: "https://keycloak.corp.local/realms/teamdocs"
  #     clientId: "teamdocs"
  #     clientSecret: "..."
  #     # groupsClaim: groups  # + всегда читается realm_access.roles (Keycloak);
  #     #                        группы работают с editorGroups
  # --- режим 3: LDAP (FreeIPA / OpenLDAP / Active Directory) ---
  # ldap:
  #   url: ldaps://dc.corp.local:636   # или ldap:// (+ startTls: true)
  #   preset: ad                       # ad | freeipa | openldap
  #   baseDn: dc=corp,dc=local
  #   bindLogin: svc-teamdocs          # DN / UPN / короткий логин (см. bindLoginTemplate)
  #   bindPassword: "..."
  #   adminGroups: ["docs-admins"]     # DN или CN групп с ролью admin
  #   nestedGroups: true               # вложенные группы (FreeIPA/OpenLDAP; AD — всегда)
  #   syncGroups: true                 # зеркалировать группы в локальные:
  #                                    # роль в проекте можно выдать LDAP-группе
  # localAdmin:                        # break-glass: работает даже при упавшем LDAP
  #   username: root
  #   passwordHash: "$2y$..."          # bcrypt: htpasswd -nbB x 'пароль'
```

Схема БД применяется автоматически при старте (миграции встроены в бинарь).

## Разработка

```bash
# 1. Поднять Postgres и создать базу teamdocs
# 2. Бэкенд (API на :8080)
task dev:backend      # или: make dev-backend / go run ./cmd/server

# 3. Фронт (Vite на :5173, проксирует /api → :8080)
task dev:frontend     # или: make dev-frontend / cd web && npm install && npm run dev
```

Открыть http://localhost:5173.

После изменения SQL-запросов в `internal/store/query/*.sql`:

```bash
task sqlc             # или: make sqlc / sqlc generate
```

## Сборка (один бинарь)

```bash
task build            # или: make build   (web build → sqlc → go build -tags prod)
./team-docs.exe
```

Собирает фронт в `web/dist`, встраивает его в бинарь (`//go:embed`, тег `prod`)
и линкует. Итог — один исполняемый файл, который раздаёт и UI, и API, и применяет
миграции. Открыть http://localhost:8080.

> Без тега `prod` (`go build ./...`) фронт не встраивается — статику отдаёт Vite.

CI (`.github/workflows/ci.yml`): lint + build фронта, `go vet` + `gofmt` +
проверка актуальности sqlc, сборка встроенного бинаря.

## Структура

```
cmd/server/          entrypoint, wiring, dev/prod раздача статики (build-теги)
internal/
  config/            Settings + пресеты цветовых схем (sconf)
  db/                pgxpool + раннер миграций (migrations/*.sql)
  store/             сгенерированный sqlc-код (query/*.sql — исходники)
  pages/             /api/pages/*, /api/search + извлечение текста из BlockNote
  uploads/           /api/upload, /api/files/:id (содержимое в БД)
  auth/              валидация JWT (JWKS/HMAC), middleware, /api/me
  backup/            полный экспорт/импорт БД одним JSON-дампом
  blocknote/         конвертер Markdown → блоки BlockNote
  mcp/               MCP-сервер (инструменты) на /mcp
  server/            сборка Echo, middleware, /api/branding, /mcp, SPA-fallback
web/                 фронтенд (Vite); embed.go встраивает dist в prod-сборке
```

## API

| Метод | Путь | Назначение |
|---|---|---|
| GET | `/api/health` | проверка живости + БД (публичный) |
| GET | `/api/branding` | брендинг + цветовые схемы (публичный) |
| GET | `/api/me` | текущий пользователь |
| GET | `/api/pages/tree` | дерево страниц |
| GET | `/api/pages/:id` | страница с контентом |
| POST | `/api/pages` | создать |
| PUT | `/api/pages/:id` | сохранить (optimistic lock по `version`, 409 при конфликте) |
| PATCH | `/api/pages/:id/move` | переместить (drag-n-drop) |
| DELETE | `/api/pages/:id` | удалить (каскад поддерева) |
| GET | `/api/pages/:id/revisions` | список версий |
| GET | `/api/pages/:id/revisions/:revId` | контент версии (для отката) |
| GET | `/api/pages/:id/markdown` | экспорт страницы в Markdown (`.md`) |
| GET | `/api/search?q=` | полнотекстовый поиск |
| GET | `/api/favorites` | избранное текущего пользователя |
| PUT | `/api/pages/:id/favorite` | добавить страницу в избранное |
| DELETE | `/api/pages/:id/favorite` | убрать из избранного |
| GET | `/api/templates?project=` | шаблоны проекта (редакторам) |
| POST | `/api/upload` | загрузка файла (в БД) |
| GET | `/api/files/:id` | отдать файл |
| GET | `/api/backup/export` | полный дамп БД (`.json`: страницы, версии, файлы) |
| POST | `/api/backup/import` | восстановление БД из дампа (**полная перезапись**) |

Все `/api/*`, кроме `/api/health` и `/api/branding`, проходят через middleware
авторизации (в открытом режиме оно подставляет dev-пользователя). При включённой
авторизации доступ разграничен: `GET` — чтение (для всех, если `publicRead`),
а `POST/PUT/PATCH/DELETE` и выгрузка БД — только для пользователя с правом правки
(`RequireEditor`/`RequireEditorStrict`, иначе `401`/`403`). `/mcp` при включённой
авторизации закрывается теми же гардами. Исключение — избранное
(`/api/favorites`, `PUT/DELETE /api/pages/:id/favorite`): это личная навигация,
доступная и читателям; требуется только аутентификация (аноним получает `401`).

Шаблоны — обычные страницы с флагом `isTemplate` (создаются `POST /api/pages`
с `"template": true`); они всегда корневые, скрыты из дерева/поиска/недавних.
Создание страницы из шаблона: `POST /api/pages` с `"templateId": N` — копия
заголовка, иконки, контента и тегов в проекте шаблона.

## MCP — генерация доков LLM-агентом → прямо в team-docs

Сервер поднимает **MCP-эндпоинт** (Streamable HTTP) на `/mcp`. Подключите
MCP-клиент (Claude Desktop, Cursor, свой агент) по URL
`http://localhost:8080/mcp` — и LLM сможет читать структуру, искать и
**создавать/обновлять страницы прямо из Markdown**.

| Инструмент | Назначение |
|---|---|
| `list_pages` | дерево страниц (чтобы выбрать родителя) |
| `search_pages(query)` | полнотекстовый поиск |
| `get_page(id)` | прочитать (заголовок + текст) |
| `create_page(title, markdown, parent_id?)` | создать страницу из Markdown |
| `update_page(id, markdown, title?)` | перезаписать содержимое |
| `append_to_page(id, markdown)` | дописать в конец, не трогая существующее |
| `export_page(id)` | экспортировать страницу в Markdown |
| `move_page(id, parent_id?, position?)` | переместить |
| `delete_page(id)` | удалить страницу и поддерево |
| `list_revisions(id)` | история версий |

Экспорт доступен и по REST: `GET /api/pages/:id/markdown` (скачивание `.md`).

Markdown (заголовки, списки, код, цитаты, `**жирный**`/`*курсив*`/`` `код` ``,
ссылки) конвертируется в блоки BlockNote (`internal/blocknote`) и рендерится
нативно. Пример конфигурации remote-MCP:

```json
{ "mcpServers": { "team-docs": { "url": "http://localhost:8080/mcp" } } }
```

> `/mcp` не закрыт auth-middleware — рассчитан на локальную/доверенную интеграцию;
> при включённой авторизации закрывается тем же middleware.

## Заметки

- **Файлы** хранятся в БД (`files.content BYTEA`), диск не используется. Читаются
  в память с лимитом `maxUploadBytes`; для очень больших файлов имеет смысл
  потоковая отдача/внешнее хранилище.
- **История версий** пишется снапшотами не чаще раза в 2 минуты на страницу;
  откат = сохранение старого контента как новой версии.
- **Диаграммы (draw.io)** отложены: таблица `diagrams` и sqlc-запросы оставлены
  как задел, но HTTP-модуль и UI-блок не подключены. Для схем используется Mermaid.
