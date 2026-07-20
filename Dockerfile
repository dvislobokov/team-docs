# Multi-stage сборка team-docs: фронт (Vite) → Go-бинарь с встроенной
# статикой (-tags prod) → минимальный рантайм-образ.
# Сборка:  docker build -t team-docs .
# Запуск:  см. docker-compose.yml (нужен Postgres и TEAMDOCS_DB__DSN).

# --- 1. Фронтенд ---
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- 2. Бэкенд (vendor уже в репо — сеть не нужна) ---
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -tags prod -trimpath -ldflags "-s -w" -o /team-docs ./cmd/server

# --- 3. Рантайм ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /team-docs /team-docs
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/team-docs"]
