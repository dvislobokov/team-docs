package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dvislobokov/sconf"
	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/backup"
	"team-docs/internal/config"
	"team-docs/internal/db"
	"team-docs/internal/mcp"
	"team-docs/internal/pages"
	"team-docs/internal/server"
	"team-docs/internal/uploads"
)

func main() {
	log := srog.NewConsole()
	defer func() { _ = log.Close() }()

	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, sconf.ErrHelp) {
			os.Exit(0)
		}
		log.Fatal(err, "failed to load config")
	}

	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DB.DSN)
	if err != nil {
		log.Fatal(err, "failed to connect to database")
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err, "failed to run migrations")
	}
	log.Information("migrations applied")

	authenticator, err := auth.New(cfg.Auth)
	if err != nil {
		log.Fatal(err, "failed to init auth")
	}
	if cfg.Auth.Enabled {
		log.Information("auth: JWT validation enabled")
	} else {
		log.Information("auth: disabled — running in open mode (dev)")
	}

	srv := server.New(cfg, log, pool)

	// Все /api-роуты (кроме /api/health) за middleware авторизации.
	// Middleware — identity; RequireEditor — гард на запись (см. internal/auth).
	api := srv.API()
	api.Use(auth.Middleware(authenticator, log))
	api.Use(auth.RequireEditor(authenticator, log))
	auth.NewHandler(authenticator).Register(api)
	pages.NewHandler(pool, log).Register(api)
	uploads.NewHandler(pool, log, cfg.MaxUploadBytes).Register(api)
	backup.NewHandler(pool, log).Register(api, auth.RequireEditorStrict(authenticator, log))

	// MCP-эндпоинт (/mcp): генерация доков LLM-агентом → прямо в team-docs.
	// MCP умеет писать, поэтому при включённой авторизации закрываем его теми же
	// гардами (клиент должен слать токен в заголовке). Без авторизации — открыт.
	var mcpMW []echo.MiddlewareFunc
	if cfg.Auth.Enabled {
		mcpMW = []echo.MiddlewareFunc{
			auth.Middleware(authenticator, log),
			auth.RequireEditorStrict(authenticator, log),
		}
	}
	srv.RegisterMCP(mcp.NewHTTPHandler(pool, log), mcpMW...)
	log.Information("mcp: endpoint mounted at /mcp")

	registerStatic(srv, log) // no-op в dev, embed в prod-сборке

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	log.Information("starting server on {Addr}", addr)
	if err := srv.Start(addr); err != nil {
		log.Fatal(err, "server stopped")
	}
}
