# Makefile для team-docs — аналог Taskfile.yml для тех, у кого установлен make.
# Использование: make <цель>. Полный список: make help (или просто make).

# Имя бинаря зависит от ОС (на Windows — .exe).
ifeq ($(OS),Windows_NT)
BIN := team-docs.exe
else
BIN := team-docs
endif

WEB := web

.DEFAULT_GOAL := help

.PHONY: help
help: ## Показать список целей
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- разработка ---
.PHONY: dev-backend
dev-backend: ## Запустить Go-бэкенд (dev, API на :8080)
	go run ./cmd/server

.PHONY: dev-frontend
dev-frontend: ## Запустить Vite dev-сервер (:5173, proxy /api -> :8080)
	cd $(WEB) && npm run dev

# --- генерация ---
.PHONY: sqlc
sqlc: ## Сгенерировать типобезопасный код доступа к БД (sqlc)
	sqlc generate

# --- сборка ---
.PHONY: build-frontend
build-frontend: ## Собрать фронтенд в web/dist
	cd $(WEB) && npm install && npm run build

.PHONY: build
build: build-frontend sqlc ## Собрать один бинарь со встроенным фронтом (prod)
	go build -tags prod -o $(BIN) ./cmd/server

.PHONY: run
run: build ## Собрать и запустить prod-бинарь
	./$(BIN)

# --- качество ---
.PHONY: lint
lint: ## Линт фронтенда (oxlint)
	cd $(WEB) && npm run lint

.PHONY: vet
vet: ## go vet ./...
	go vet ./...

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: clean
clean: ## Удалить бинарь и web/dist
	rm -rf $(BIN) $(WEB)/dist
