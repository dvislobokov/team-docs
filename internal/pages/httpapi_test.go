package pages_test

// Интеграционный тест HTTP-уровня: CRUD страниц и optimistic lock (409)
// через настоящий echo-роутер и Postgres. Без auth-middleware — открытый режим.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dvislobokov/srog"
	"github.com/labstack/echo/v4"

	"team-docs/internal/pages"
)

type pageDTO struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Version int32  `json:"version"`
}

func newAPI(t *testing.T) *echo.Echo {
	t.Helper()
	pool := testPool(t)
	log := srog.NewConsole()
	t.Cleanup(func() { _ = log.Close() })
	e := echo.New()
	pages.NewHandler(pool, log).Register(e.Group("/api"))
	return e
}

func call(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestHTTPCrudAndConflict(t *testing.T) {
	e := newAPI(t)

	// Create
	rec := call(e, http.MethodPost, "/api/pages", `{"parentId":null,"title":"http-crud"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var p pageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	defer call(e, http.MethodDelete, fmt.Sprintf("/api/pages/%d/purge", p.ID), "")
	defer call(e, http.MethodDelete, fmt.Sprintf("/api/pages/%d", p.ID), "")

	// Read
	if rec := call(e, http.MethodGet, fmt.Sprintf("/api/pages/%d", p.ID), ""); rec.Code != http.StatusOK {
		t.Fatalf("get: code=%d", rec.Code)
	}

	// Update с актуальной версией
	upd := fmt.Sprintf(`{"title":"http-crud-2","icon":"","content":[],"version":%d}`, p.Version)
	rec = call(e, http.MethodPut, fmt.Sprintf("/api/pages/%d", p.ID), upd)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Update со старой версией → 409
	rec = call(e, http.MethodPut, fmt.Sprintf("/api/pages/%d", p.ID), upd)
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale update: code=%d, ожидался 409", rec.Code)
	}

	// Update несуществующей → 404
	rec = call(e, http.MethodPut, "/api/pages/999999999", upd)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: code=%d, ожидался 404", rec.Code)
	}

	// Некорректный id → 400
	if rec := call(e, http.MethodGet, "/api/pages/not-a-number", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad id: code=%d, ожидался 400", rec.Code)
	}

	// Создание под удалённым родителем → 400
	if rec := call(e, http.MethodDelete, fmt.Sprintf("/api/pages/%d", p.ID), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: code=%d", rec.Code)
	}
	rec = call(e, http.MethodPost, "/api/pages", fmt.Sprintf(`{"parentId":%d,"title":"orphan"}`, p.ID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create под удалённым родителем: code=%d, ожидался 400", rec.Code)
	}

	// Restore → страница снова читается
	if rec := call(e, http.MethodPost, fmt.Sprintf("/api/pages/%d/restore", p.ID), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("restore: code=%d", rec.Code)
	}
	if rec := call(e, http.MethodGet, fmt.Sprintf("/api/pages/%d", p.ID), ""); rec.Code != http.StatusOK {
		t.Fatalf("get после restore: code=%d", rec.Code)
	}

	// Move в собственное поддерево → 400 (HTTP-маппинг ErrMoveIntoSubtree)
	rec = call(e, http.MethodPatch, fmt.Sprintf("/api/pages/%d/move", p.ID),
		fmt.Sprintf(`{"parentId":%d,"position":0}`, p.ID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("move в себя: code=%d, ожидался 400", rec.Code)
	}
}
