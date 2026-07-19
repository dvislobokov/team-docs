package server

import (
	"io/fs"
	"net/http"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"team-docs/internal/config"
)

// Server держит зависимости и Echo-инстанс.
type Server struct {
	echo *echo.Echo
	log  *srog.Logger
	pool *pgxpool.Pool
	cfg  *config.Settings
	// brandingFn — динамический источник брендинга (настройки в БД, §9);
	// nil → статичный конфиг.
	brandingFn func() config.BrandingSettings
}

// SetBrandingSource подключает динамический источник брендинга.
func (s *Server) SetBrandingSource(fn func() config.BrandingSettings) { s.brandingFn = fn }

// New собирает сервер: middleware, базовые роуты. API-модули регистрируются отдельно.
func New(cfg *config.Settings, log *srog.Logger, pool *pgxpool.Pool) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(requestLogger(log))

	s := &Server{echo: e, log: log, pool: pool, cfg: cfg}

	// Публичные роуты (без авторизации): проверка живости и брендинг.
	// Брендинг нужен ещё до логина (в т.ч. на экране «требуется авторизация»).
	api := e.Group("/api")
	api.GET("/health", s.health)
	api.GET("/branding", s.branding)

	return s
}

// Echo возвращает Echo-инстанс для регистрации дополнительных роутов.
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// RegisterMCP монтирует MCP-эндпоинт (Streamable HTTP) на /mcp. Вне группы /api
// и без auth-middleware — рассчитано на локальную/доверенную интеграцию; при
// включённой авторизации можно закрыть тем же middleware.
func (s *Server) RegisterMCP(h http.Handler, mw ...echo.MiddlewareFunc) {
	s.echo.Any("/mcp", echo.WrapHandler(h), mw...)
}

// API возвращает группу /api для регистрации модулей.
func (s *Server) API() *echo.Group {
	return s.echo.Group("/api")
}

// Start запускает HTTP-сервер (блокирующий вызов).
func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) health(c echo.Context) error {
	if err := s.pool.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, echo.Map{"status": "db_unavailable"})
	}
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

// branding отдаёт брендинг, тему по умолчанию и все пресеты палитры.
// Пользователь выбирает схему в UI (сохранение в localStorage); сервер лишь
// задаёт дефолт (branding.theme).
func (s *Server) branding(c echo.Context) error {
	b := s.cfg.Branding
	if s.brandingFn != nil {
		b = s.brandingFn()
	}

	themes := make([]echo.Map, 0, len(config.Themes()))
	for _, t := range config.Themes() {
		themes = append(themes, echo.Map{
			"id":    t.ID,
			"label": t.Label,
			"palette": echo.Map{
				"light": paletteMap(t.Palette.Light),
				"dark":  paletteMap(t.Palette.Dark),
			},
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"appName":       b.AppName,
		"workspaceName": b.WorkspaceName,
		"monogram":      b.Monogram,
		"defaultTheme":  b.DefaultThemeID(),
		"themes":        themes,
	})
}

func paletteMap(p config.PaletteColors) echo.Map {
	return echo.Map{
		"paper":      p.Paper,
		"card":       p.Card,
		"ink":        p.Ink,
		"body":       p.Body,
		"muted":      p.Muted,
		"faint":      p.Faint,
		"line":       p.Line,
		"accent":     p.Accent,
		"accentSoft": p.AccentSoft,
		"marker":     p.Marker,
	}
}

// RegisterStatic подключает встроенный фронт: раздаёт статику и на любой
// не-API путь отдаёт index.html (SPA-fallback). Вызывается только в prod-сборке.
func (s *Server) RegisterStatic(assets fs.FS) {
	fileServer := http.FileServer(http.FS(assets))
	s.echo.GET("/*", func(c echo.Context) error {
		req := c.Request()
		// Если файл существует — отдаём его, иначе index.html (клиентский роутинг).
		if _, err := fs.Stat(assets, trimLeadingSlash(req.URL.Path)); err != nil {
			req.URL.Path = "/"
		}
		fileServer.ServeHTTP(c.Response(), req)
		return nil
	})
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	if p == "" {
		return "."
	}
	return p
}

// requestLogger логирует HTTP-запросы через srog.
func requestLogger(log *srog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error != nil {
				log.Error(v.Error, "{Method} {URI} -> {Status}", v.Method, v.URI, v.Status)
			} else {
				log.Information("{Method} {URI} -> {Status}", v.Method, v.URI, v.Status)
			}
			return nil
		},
	})
}
