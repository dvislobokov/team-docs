package uploads

import (
	"errors"
	"io"
	"net/http"

	"github.com/dvislobokov/srog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/projects"
	"team-docs/internal/store"
)

const defaultMaxBytes = 20 << 20 // 20 MiB

// Handler обслуживает загрузку и отдачу файлов из редактора. Содержимое
// хранится в БД (BYTEA), диск не используется.
type Handler struct {
	q   *store.Queries
	a   *auth.Authenticator
	log *srog.Logger
	// maxBytes — динамический лимит (настройки в БД, §9).
	maxBytes func() int64
}

func NewHandler(pool *pgxpool.Pool, a *auth.Authenticator, log *srog.Logger, maxBytes func() int64) *Handler {
	return &Handler{q: store.New(pool), a: a, log: log, maxBytes: maxBytes}
}

func (h *Handler) Register(api *echo.Group) {
	api.POST("/upload", h.upload)
	api.GET("/files/:id", h.serve)
}

func (h *Handler) upload(c echo.Context) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file field required")
	}

	limit := h.maxBytes()
	if limit <= 0 {
		limit = defaultMaxBytes
	}
	if fh.Size > limit {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
	}

	src, err := fh.Open()
	if err != nil {
		return h.fail(c, err, "open upload")
	}
	defer func() { _ = src.Close() }()

	// Читаем в память с жёстким лимитом (+1 байт — чтобы поймать превышение,
	// если заявленный размер вдруг занижен).
	data, err := io.ReadAll(io.LimitReader(src, limit+1))
	if err != nil {
		return h.fail(c, err, "read upload")
	}
	if int64(len(data)) > limit {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file too large")
	}

	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	id := uuid.New()
	if err := h.q.InsertFile(c.Request().Context(), store.InsertFileParams{
		ID:       id,
		PageID:   nil,
		Filename: fh.Filename,
		Mime:     mime,
		Size:     int64(len(data)),
		Content:  data,
	}); err != nil {
		return h.fail(c, err, "insert file")
	}

	// BlockNote ожидает URL строкой.
	return c.JSON(http.StatusOK, echo.Map{
		"id":  id.String(),
		"url": "/api/files/" + id.String(),
	})
}

func (h *Handler) serve(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	ctx := c.Request().Context()

	// Проектный гард (§10): файл принадлежит проекту страницы, где он
	// используется. Не привязанный ни к одной странице (свежезагруженный) —
	// виден только пользователям с правом правки.
	u, _ := auth.FromContext(c)
	projectID, err := h.q.FindFileProject(ctx, id.String())
	switch {
	case err == nil:
		role, rerr := projects.RoleForID(ctx, h.q, h.a, u, projectID)
		if rerr != nil {
			return h.fail(c, rerr, "resolve file project role")
		}
		if !projects.CanRead(role) {
			return echo.NewHTTPError(http.StatusNotFound, "file not found")
		}
	case errors.Is(err, pgx.ErrNoRows):
		if h.a.Enabled() && (u == nil || !h.a.CanEdit(u)) {
			return echo.NewHTTPError(http.StatusNotFound, "file not found")
		}
	default:
		return h.fail(c, err, "find file project")
	}

	rec, err := h.q.GetFile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusNotFound, "file not found")
	}
	if err != nil {
		return h.fail(c, err, "get file")
	}

	// id неизменяем и уникален (uuid) — можно кешировать надолго.
	c.Response().Header().Set("Content-Disposition", "inline; filename=\""+rec.Filename+"\"")
	c.Response().Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	return c.Blob(http.StatusOK, rec.Mime, rec.Content)
}

func (h *Handler) fail(c echo.Context, err error, op string) error {
	h.log.Error(err, "uploads: {Op} failed", op)
	return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
}
