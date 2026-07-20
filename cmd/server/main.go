package main

import (
	"context"
	"fmt"
	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/backup"
	"team-docs/internal/config"
	"team-docs/internal/db"
	"team-docs/internal/mcp"
	"team-docs/internal/pages"
	"team-docs/internal/projects"
	"team-docs/internal/server"
	"team-docs/internal/settings"
	"team-docs/internal/uploads"
)

func main() {
	log := srog.NewConsole()
	defer func() { _ = log.Close() }()

	cfg, err := config.Load()
	if err != nil {
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

	// Реестр пользователей: upsert identity в users (авторство, роли,
	// бутстрап админов из auth.adminEmails).
	registry := auth.NewRegistry(pool, cfg.Auth)

	// Встроенный OAuth-логин (/auth/*): вне /api и auth-middleware.
	providers := auth.BuildProviders(cfg.Auth)
	oauthHandler := auth.NewOAuthHandler(authenticator, registry, log, cfg.Auth.PublicURL, providers)
	oauthHandler.Register(srv.Echo())
	if len(providers) > 0 {
		log.Information("auth: builtin OAuth providers enabled: {Count}", len(providers))
	}

	// Вход по логину/паролю: LDAP (§8) + break-glass локальный админ.
	ldapAuth, err := auth.NewLDAP(cfg.Auth.LDAP)
	if err != nil {
		log.Fatal(err, "failed to init ldap")
	}
	pwHandler := auth.NewPasswordHandler(authenticator, registry, ldapAuth, cfg.Auth.LocalAdmin, log)
	pwHandler.Register(srv.Echo())
	oauthHandler.SetPasswordEnabled(pwHandler.Enabled)
	if ldapAuth != nil {
		log.Information("auth: LDAP enabled ({Preset})", cfg.Auth.LDAP.Preset)
	}

	// Все /api-роуты (кроме /api/health) за middleware авторизации.
	// Middleware — identity; RequireEditor — гард на запись (см. internal/auth).
	api := srv.API()
	api.Use(auth.Middleware(authenticator, registry, log))
	api.Use(auth.RequireEditor(authenticator, log))
	// Настройки в БД (env > yaml > БД > дефолт; конфиг блокирует правку).
	settingsSvc, err := settings.New(pool, cfg, "appsettings.yaml")
	if err != nil {
		log.Fatal(err, "failed to init settings")
	}
	srv.SetBrandingSource(settingsSvc.Branding)

	auth.NewHandler(authenticator).Register(api)
	// Админка: пользователи/роли и настройки (только для role=admin).
	adminGroup := api.Group("/admin", auth.RequireAdmin(authenticator, log))
	auth.NewAdminHandler(pool, registry, authenticator).Register(adminGroup)
	settings.NewHandler(settingsSvc).Register(adminGroup)
	pages.RegisterMaintenance(adminGroup, pool, log)
	pagesHandler := pages.NewHandler(pool, authenticator, log)
	pagesHandler.Register(api)
	// Избранное — личная навигация, доступная и читателям: отдельная группа
	// /api только с identity-middleware, без RequireEditor.
	favAPI := srv.API()
	favAPI.Use(auth.Middleware(authenticator, registry, log))
	pagesHandler.RegisterFavorites(favAPI)
	projects.NewHandler(pool, authenticator, log).Register(api)
	uploads.NewHandler(pool, authenticator, log, settingsSvc.MaxUploadBytes).Register(api)
	backup.NewHandler(pool, log, registry.Reset).Register(api, auth.RequireEditorStrict(authenticator, log))

	// MCP-эндпоинт (/mcp): генерация доков LLM-агентом → прямо в team-docs.
	// MCP умеет писать, поэтому при включённой авторизации закрываем его теми же
	// гардами (клиент должен слать токен в заголовке). Без авторизации — открыт.
	var mcpMW []echo.MiddlewareFunc
	if cfg.Auth.Enabled {
		mcpMW = []echo.MiddlewareFunc{
			auth.Middleware(authenticator, registry, log),
			auth.RequireEditorStrict(authenticator, log),
		}
	}
	srv.RegisterMCP(mcp.NewHTTPHandler(pool, log), mcpMW...)
	log.Information("mcp: endpoint mounted at /mcp")

	// Фоновая уборка (корзина, старые ревизии, осиротевшие файлы):
	// при старте и раз в сутки.
	go pages.RunJanitor(ctx, pool, log)

	registerStatic(srv, log) // no-op в dev, embed в prod-сборке

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	log.Information("starting server on {Addr}", addr)
	if err := srv.Start(addr); err != nil {
		log.Fatal(err, "server stopped")
	}
}
