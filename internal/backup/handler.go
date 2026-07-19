package backup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// Handler обслуживает /api/backup/*.
type Handler struct {
	svc *Service
	log *srog.Logger
	// onImport дёргается после успешного импорта (сброс кэшей: таблица users
	// пересоздана, закэшированные id невалидны). Может быть nil.
	onImport func()
}

func NewHandler(pool *pgxpool.Pool, log *srog.Logger, onImport func()) *Handler {
	return &Handler{svc: New(pool), log: log, onImport: onImport}
}

// Register регистрирует роуты на группе /api. exportGuard закрывает выгрузку
// всей БД (GET, но чувствительный) — его надо требовать даже при публичном
// чтении; импорт (POST) и так отсекается общим RequireEditor на /api.
func (h *Handler) Register(api *echo.Group, exportGuard echo.MiddlewareFunc) {
	api.GET("/backup/export", h.export, exportGuard)
	api.POST("/backup/import", h.importDump)
}

// export отдаёт весь дамп БД как скачиваемый .json.
func (h *Handler) export(c echo.Context) error {
	d, err := h.svc.Export(c.Request().Context())
	if err != nil {
		h.log.Error(err, "backup: export failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "не удалось выгрузить БД")
	}
	filename := fmt.Sprintf("teamdocs-backup-%s.json", time.Now().Format("2006-01-02-150405"))
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	return c.JSONPretty(http.StatusOK, d, "  ")
}

// importDump ПОЛНОСТЬЮ заменяет содержимое БД из загруженного дампа.
func (h *Handler) importDump(c echo.Context) error {
	var d Dump
	if err := json.NewDecoder(c.Request().Body).Decode(&d); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "некорректный файл резервной копии")
	}
	if err := h.svc.Import(c.Request().Context(), &d); err != nil {
		h.log.Error(err, "backup: import failed")
		return echo.NewHTTPError(http.StatusBadRequest, "импорт не выполнен: "+err.Error())
	}
	if h.onImport != nil {
		h.onImport()
	}
	return c.JSON(http.StatusOK, echo.Map{
		"status":    "ok",
		"pages":     len(d.Pages),
		"revisions": len(d.Revisions),
		"files":     len(d.Files),
		"diagrams":  len(d.Diagrams),
	})
}
