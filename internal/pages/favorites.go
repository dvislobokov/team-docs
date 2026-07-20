package pages

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"team-docs/internal/auth"
	"team-docs/internal/projects"
	"team-docs/internal/store"
)

// RegisterFavorites регистрирует роуты избранного. Ставится на отдельную
// группу /api БЕЗ RequireEditor: избранное — личная навигация, доступная и
// читателям; identity обязательна (аноним получает 401).
func (h *Handler) RegisterFavorites(api *echo.Group) {
	api.GET("/favorites", h.favorites)
	api.PUT("/pages/:id/favorite", h.addFavorite)
	api.DELETE("/pages/:id/favorite", h.removeFavorite)
}

// favoriteUser — id пользователя для операций избранного; 401 без identity.
func favoriteUser(c echo.Context) (int64, error) {
	uid := auth.UserID(c)
	if uid == nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	return *uid, nil
}

// favorites отдаёт избранные страницы пользователя (по всем доступным проектам).
func (h *Handler) favorites(c echo.Context) error {
	uid, err := favoriteUser(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rows, err := h.q.ListFavorites(ctx, uid)
	if err != nil {
		return h.fail(c, err, "list favorites")
	}
	// Страницы из ставших недоступными проектов прячем (аналогично корзине).
	u, _ := auth.FromContext(c)
	readable := map[int64]bool{}
	type item struct {
		ID        int64     `json:"id"`
		Title     string    `json:"title"`
		Icon      string    `json:"icon"`
		ProjectID int64     `json:"projectId"`
		CreatedAt time.Time `json:"createdAt"`
	}
	out := []item{}
	for _, r := range rows {
		ok, seen := readable[r.ProjectID]
		if !seen {
			role, err := projects.RoleForID(ctx, h.q, h.a, u, r.ProjectID)
			if err != nil {
				return h.fail(c, err, "resolve project role")
			}
			ok = projects.CanRead(role)
			readable[r.ProjectID] = ok
		}
		if ok {
			out = append(out, item{
				ID: r.PageID, Title: r.Title, Icon: r.Icon,
				ProjectID: r.ProjectID, CreatedAt: r.CreatedAt.Time,
			})
		}
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) addFavorite(c echo.Context) error {
	uid, err := favoriteUser(c)
	if err != nil {
		return err
	}
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.guardRead(c, id); err != nil {
		return err
	}
	if err := h.q.AddFavorite(c.Request().Context(), store.AddFavoriteParams{
		UserID: uid,
		PageID: id,
	}); err != nil {
		return h.fail(c, err, "add favorite")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) removeFavorite(c echo.Context) error {
	uid, err := favoriteUser(c)
	if err != nil {
		return err
	}
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.q.RemoveFavorite(c.Request().Context(), store.RemoveFavoriteParams{
		UserID: uid,
		PageID: id,
	}); err != nil {
		return h.fail(c, err, "remove favorite")
	}
	return c.NoContent(http.StatusNoContent)
}
