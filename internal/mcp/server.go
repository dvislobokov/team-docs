// Package mcp — MCP-сервер поверх слоя данных team-docs. Позволяет LLM-агенту
// генерировать доки (Markdown) и пушить их прямо в систему, а также читать
// структуру, искать и обновлять страницы.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"team-docs/internal/blocknote"
	"team-docs/internal/pages"
	"team-docs/internal/store"
)

const version = "1.0.0"

// NewHTTPHandler возвращает HTTP-обработчик MCP (Streamable HTTP) — его монтируют
// на /mcp. Обёртка нужна, чтобы не тащить mcp-go/server в main (там уже есть
// пакет server из internal).
func NewHTTPHandler(pool *pgxpool.Pool, log *srog.Logger) http.Handler {
	return server.NewStreamableHTTPServer(New(pool, log))
}

// New строит MCP-сервер team-docs со всеми инструментами.
func New(pool *pgxpool.Pool, log *srog.Logger) *server.MCPServer {
	s := server.NewMCPServer(
		"team-docs",
		version,
		server.WithToolCapabilities(true),
		server.WithInstructions(
			"Инструменты для работы с вики team-docs. Контент страниц принимается и "+
				"хранится в Markdown. Перед созданием загляни в list_pages, чтобы выбрать "+
				"родителя и не плодить дубли.",
		),
	)
	h := &handlers{q: store.New(pool), pool: pool, log: log}
	h.register(s)
	return s
}

type handlers struct {
	q    *store.Queries
	pool *pgxpool.Pool // для операций с транзакцией (move)
	log  *srog.Logger
}

func (h *handlers) register(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_pages",
		mcp.WithDescription("Дерево всех страниц (id, parentId, title, icon, position). Используй, чтобы понять структуру и выбрать родителя.")),
		h.listPages)

	s.AddTool(mcp.NewTool("search_pages",
		mcp.WithDescription("Полнотекстовый поиск по страницам: id, title, сниппет."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Поисковый запрос"))),
		h.searchPages)

	s.AddTool(mcp.NewTool("get_page",
		mcp.WithDescription("Прочитать страницу: заголовок, иконку, текст, версию."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы"))),
		h.getPage)

	s.AddTool(mcp.NewTool("create_page",
		mcp.WithDescription("Создать страницу из Markdown. Возвращает id и ссылку."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Заголовок")),
		mcp.WithString("markdown", mcp.Required(), mcp.Description("Содержимое в Markdown")),
		mcp.WithNumber("parent_id", mcp.Description("ID родительской страницы (опц.; иначе корень)"))),
		h.createPage)

	s.AddTool(mcp.NewTool("update_page",
		mcp.WithDescription("Перезаписать содержимое страницы из Markdown."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы")),
		mcp.WithString("markdown", mcp.Required(), mcp.Description("Новое содержимое в Markdown")),
		mcp.WithString("title", mcp.Description("Новый заголовок (опц.)"))),
		h.updatePage)

	s.AddTool(mcp.NewTool("move_page",
		mcp.WithDescription("Переместить страницу: сменить родителя и/или позицию."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы")),
		mcp.WithNumber("parent_id", mcp.Description("Новый родитель (опц.; иначе корень)")),
		mcp.WithNumber("position", mcp.Description("Позиция среди сиблингов (по умолчанию 0)"))),
		h.movePage)

	s.AddTool(mcp.NewTool("delete_page",
		mcp.WithDescription("Удалить страницу и всё её поддерево (в корзину; восстановимо через UI)."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы"))),
		h.deletePage)

	s.AddTool(mcp.NewTool("list_revisions",
		mcp.WithDescription("История версий страницы."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы"))),
		h.listRevisions)

	s.AddTool(mcp.NewTool("append_to_page",
		mcp.WithDescription("Дописать Markdown в конец страницы, не затрагивая существующий контент."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы")),
		mcp.WithString("markdown", mcp.Required(), mcp.Description("Что дописать (Markdown)"))),
		h.appendToPage)

	s.AddTool(mcp.NewTool("export_page",
		mcp.WithDescription("Экспортировать страницу в Markdown (с заголовком H1)."),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("ID страницы"))),
		h.exportPage)
}

// --- helpers ---

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("marshal: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func (h *handlers) fail(op string, err error) (*mcp.CallToolResult, error) {
	h.log.Error(err, "mcp: {Op} failed", op)
	return mcp.NewToolResultError(op + ": " + err.Error()), nil
}

func optParentID(r mcp.CallToolRequest) *int64 {
	if v, ok := r.GetArguments()["parent_id"]; ok && v != nil {
		if f, ok := v.(float64); ok && f > 0 {
			id := int64(f)
			return &id
		}
	}
	return nil
}

// --- tools ---

// mainProjectID — MCP работает в дефолтном проекте 'main' (доверенная
// интеграция; проектный параметр — задел ROADMAP §10).
func (h *handlers) mainProjectID(ctx context.Context) (int64, error) {
	p, err := h.q.GetProjectByKey(ctx, "main")
	return p.ID, err
}

func (h *handlers) listPages(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mainID, err := h.mainProjectID(ctx)
	if err != nil {
		return h.fail("list_pages", err)
	}
	rows, err := h.q.GetPageTree(ctx, mainID)
	if err != nil {
		return h.fail("list_pages", err)
	}
	type node struct {
		ID       int64  `json:"id"`
		ParentID *int64 `json:"parentId"`
		Title    string `json:"title"`
		Icon     string `json:"icon"`
		Position int32  `json:"position"`
	}
	out := make([]node, 0, len(rows))
	for _, x := range rows {
		out = append(out, node{ID: x.ID, ParentID: x.ParentID, Title: x.Title, Icon: x.Icon, Position: x.Position})
	}
	return jsonResult(out)
}

func (h *handlers) searchPages(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	q, err := r.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	mainID, err := h.mainProjectID(ctx)
	if err != nil {
		return h.fail("search_pages", err)
	}
	rows, err := h.q.SearchPages(ctx, store.SearchPagesParams{PlaintoTsquery: q, ProjectIds: []int64{mainID}})
	if err != nil {
		return h.fail("search_pages", err)
	}
	type hit struct {
		ID      int64  `json:"id"`
		Title   string `json:"title"`
		Snippet string `json:"snippet"`
	}
	out := make([]hit, 0, len(rows))
	for _, x := range rows {
		out = append(out, hit{ID: x.ID, Title: x.Title, Snippet: string(x.Snippet)})
	}
	return jsonResult(out)
}

func (h *handlers) getPage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	row, err := h.q.GetPage(ctx, int64(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("page not found"), nil
	}
	if err != nil {
		return h.fail("get_page", err)
	}
	return jsonResult(map[string]any{
		"id":        row.ID,
		"parentId":  row.ParentID,
		"title":     row.Title,
		"icon":      row.Icon,
		"text":      pages.ExtractText(row.Content),
		"version":   row.Version,
		"updatedAt": row.UpdatedAt.Time,
	})
}

func (h *handlers) createPage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, err := r.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	md, err := r.RequireString("markdown")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	content, text, err := blocknote.FromMarkdown(md)
	if err != nil {
		return h.fail("create_page: markdown", err)
	}

	mainID, err := h.mainProjectID(ctx)
	if err != nil {
		return h.fail("create_page", err)
	}
	created, err := h.q.CreatePage(ctx, store.CreatePageParams{
		ParentID: optParentID(r), Title: title, ProjectID: mainID,
	})
	if err != nil {
		return h.fail("create_page", err)
	}
	upd, err := h.q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          created.ID,
		Title:       title,
		Content:     content,
		ContentText: text,
		Version:     created.Version,
		Icon:        "",
	})
	if err != nil {
		return h.fail("create_page: save content", err)
	}
	return jsonResult(map[string]any{
		"id":    upd.ID,
		"title": upd.Title,
		"url":   fmt.Sprintf("/api/pages/%d", upd.ID),
	})
}

func (h *handlers) updatePage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	md, err := r.RequireString("markdown")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cur, err := h.q.GetPage(ctx, int64(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("page not found"), nil
	}
	if err != nil {
		return h.fail("update_page: load", err)
	}
	content, text, err := blocknote.FromMarkdown(md)
	if err != nil {
		return h.fail("update_page: markdown", err)
	}
	upd, err := h.q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          int64(id),
		Title:       r.GetString("title", cur.Title),
		Content:     content,
		ContentText: text,
		Version:     cur.Version,
		Icon:        cur.Icon,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("version conflict: page was modified"), nil
	}
	if err != nil {
		return h.fail("update_page", err)
	}
	return jsonResult(map[string]any{
		"id":      upd.ID,
		"title":   upd.Title,
		"version": upd.Version,
		"url":     fmt.Sprintf("/api/pages/%d", upd.ID),
	})
}

func (h *handlers) movePage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	switch err := pages.Move(ctx, h.pool, int64(id), optParentID(r), int32(r.GetInt("position", 0))); {
	case errors.Is(err, pages.ErrPageNotFound), errors.Is(err, pages.ErrMoveIntoSubtree):
		return mcp.NewToolResultError(err.Error()), nil
	case err != nil:
		return h.fail("move_page", err)
	}
	return jsonResult(map[string]any{"status": "moved", "id": id})
}

func (h *handlers) deletePage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := pages.SoftDelete(ctx, h.pool, int64(id)); errors.Is(err, pages.ErrPageNotFound) {
		return mcp.NewToolResultError(err.Error()), nil
	} else if err != nil {
		return h.fail("delete_page", err)
	}
	return jsonResult(map[string]any{"status": "deleted", "id": id})
}

func (h *handlers) appendToPage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	md, err := r.RequireString("markdown")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	cur, err := h.q.GetPage(ctx, int64(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("page not found"), nil
	}
	if err != nil {
		return h.fail("append_to_page: load", err)
	}

	newRaw, _, err := blocknote.FromMarkdown(md)
	if err != nil {
		return h.fail("append_to_page: markdown", err)
	}
	// Склеиваем существующие блоки с новыми, сохраняя JSON как есть.
	var existing, added []json.RawMessage
	if len(cur.Content) > 0 {
		_ = json.Unmarshal(cur.Content, &existing)
	}
	if err := json.Unmarshal(newRaw, &added); err != nil {
		return h.fail("append_to_page: parse", err)
	}
	combined, err := json.Marshal(append(existing, added...))
	if err != nil {
		return h.fail("append_to_page: encode", err)
	}

	upd, err := h.q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          int64(id),
		Title:       cur.Title,
		Content:     combined,
		ContentText: pages.ExtractText(combined),
		Version:     cur.Version,
		Icon:        cur.Icon,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("version conflict: page was modified"), nil
	}
	if err != nil {
		return h.fail("append_to_page", err)
	}
	return jsonResult(map[string]any{
		"id":      upd.ID,
		"title":   upd.Title,
		"version": upd.Version,
		"url":     fmt.Sprintf("/api/pages/%d", upd.ID),
	})
}

func (h *handlers) exportPage(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	row, err := h.q.GetPage(ctx, int64(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return mcp.NewToolResultError("page not found"), nil
	}
	if err != nil {
		return h.fail("export_page", err)
	}
	body, err := blocknote.ToMarkdown(row.Content)
	if err != nil {
		return h.fail("export_page: convert", err)
	}
	return mcp.NewToolResultText("# " + row.Title + "\n\n" + body), nil
}

func (h *handlers) listRevisions(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := r.RequireInt("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	rows, err := h.q.ListRevisions(ctx, int64(id))
	if err != nil {
		return h.fail("list_revisions", err)
	}
	type rev struct {
		ID        int64  `json:"id"`
		Version   int32  `json:"version"`
		Title     string `json:"title"`
		CreatedAt string `json:"createdAt"`
	}
	out := make([]rev, 0, len(rows))
	for _, x := range rows {
		out = append(out, rev{ID: x.ID, Version: x.Version, Title: x.Title, CreatedAt: x.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00")})
	}
	return jsonResult(out)
}
