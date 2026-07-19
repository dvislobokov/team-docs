# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Что это

team-docs — внутренний аналог Confluence: React-фронт (BlockNote-редактор) + Go-бэкенд (Echo) + PostgreSQL, в prod собирается в **один бинарь** со встроенным фронтом. Вся документация, UI и комментарии — **на русском языке**.

## Команды

```bash
# Разработка (нужен поднятый Postgres, база teamdocs)
task dev:backend        # или: go run ./cmd/server           — API на :8080
task dev:frontend       # или: cd web && npm run dev         — Vite на :5173, proxy /api → :8080

# После изменения SQL в internal/store/query/*.sql
task sqlc               # или: sqlc generate                 — CI падает, если сгенерированный код устарел

# Prod-сборка: web build → sqlc → go build -tags prod (фронт встраивается через //go:embed)
task build

# Качество (то же, что проверяет CI)
go vet ./...
gofmt -l .              # CI требует отформатированный код
cd web && npm run lint  # oxlint
cd web && npm run build # tsc -b + vite build (typecheck фронта)
```

```bash
go test ./...                          # юнит; интеграционные скипаются без TEAMDOCS_TEST_DSN
go test ./internal/pages/ -run TestMove  # один тест
# интеграционные локально: поднять compose-постгрес и задать
#   TEAMDOCS_TEST_DSN=postgres://teamdocs:teamdocs@localhost:54329/teamdocs_test?sslmode=disable
```

CI (`.github/workflows/ci.yml`): oxlint + tsc/vite build; gofmt (без vendor) + go vet + dev-сборка + go test (интеграционные на postgres-сервисе) + проверка актуальности sqlc; затем prod-бинарь.

### Тестовая инфраструктура (docker-compose.test.yml)

```bash
docker compose -f docker-compose.test.yml up -d --wait            # ядро: postgres
docker compose -f docker-compose.test.yml --profile cache up -d   # + redis standalone
docker compose -f docker-compose.test.yml down -v                 # остановить и почистить
```

Профили: `mail` (Mailpit), `cache` (redis standalone), `cache-sentinel`, `cache-cluster`, `s3` (MinIO); postgres поднимается всегда. Тестовый DSN: `postgres://teamdocs:teamdocs@localhost:54329/teamdocs_test` (порты меняются через env, см. комментарии в файле). Интеграционные тесты должны скипаться, если БД недоступна, — юнит-часть бежит без Docker. Sentinel/cluster анонсируют внутренние docker-адреса — тесты этих режимов гоняются внутри той же docker-сети (или в CI), не с хоста.

## Архитектура

**Один бинарь, два режима.** `cmd/server` через build-теги выбирает раздачу статики: без тега `prod` (`static_dev.go`) статику отдаёт Vite; с тегом `prod` (`static_prod.go`) — встроенный `web/dist` (`web/embed.go`). Миграции (`internal/db/migrations/*.sql`) применяются автоматически при старте.

**Доступ к БД — только через sqlc.** Исходные запросы лежат в `internal/store/query/*.sql`, из них `sqlc generate` (конфиг `sqlc.yaml`) порождает код в `internal/store` (pgx/v5). Сгенерированные `*.sql.go`/`models.go` руками не править — менять `.sql` и перегенерировать, коммитя результат.

**Контент — блоки BlockNote в JSONB.** Страница хранит JSON-массив блоков; `internal/pages/blocktext.go` извлекает из блоков плоский текст для полнотекстового поиска, `internal/blocknote` конвертирует Markdown ↔ блоки (используется MCP-инструментами, импортом/экспортом и `GET /api/pages/:id/markdown`). Файлы/картинки хранятся в БД (`files.content BYTEA`), не на диске.

**HTTP-слой.** `internal/server/server.go` собирает Echo: подключает handlers из `pages`, `uploads`, `auth`, `backup`, MCP-эндпоинт `/mcp` (`internal/mcp`, Streamable HTTP — LLM-агенты создают/правят страницы из Markdown), `/api/branding` и SPA-fallback. Сохранение страницы использует optimistic lock по `version` (409 при конфликте); ревизии пишутся снапшотами не чаще раза в 2 минуты.

**Авторизация опциональна** (`internal/auth`): JWT от IAM-прокси (RS256 по JWKS или HS256). По умолчанию выключена — middleware подставляет dev-пользователя. При включении чтение/запись разделены: `GET` открыт (если `publicRead`), мутации — только через `RequireEditor`/`RequireEditorStrict`.

**Конфиг** — `appsettings.yaml` + env с префиксом `TEAMDOCS_` (вложенность через `__`), библиотека sconf. Пресеты цветовых схем — в `internal/config/themes.go`, фронт получает их через `GET /api/branding` (брендинг меняется без пересборки фронта).

**Фронт** (`web/src`): `api/` — типизированный клиент к REST, `components/` — экраны и блоки (в т.ч. кастомные BlockNote-блоки: Mermaid, OpenAPI/Redoc), `lib/editorSchema.tsx` — схема редактора с кастомными блоками, `store/` — состояние. React 19 + Tailwind + Radix.

## Нюансы

- UI на русском — все используемые шрифты обязаны поддерживать кириллицу (поэтому PT Serif, а не Fraunces).
- Таблица `diagrams` и её sqlc-запросы — задел под draw.io; HTTP-модуль и UI-блок не подключены, для схем используется Mermaid.
- Полный REST-API и список MCP-инструментов описаны в README.md.
- План развития и известные проблемы — в ROADMAP.md (живой документ: отмечать выполненное, дописывать решения в журнал).
