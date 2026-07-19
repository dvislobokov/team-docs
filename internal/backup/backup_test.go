package backup_test

// Интеграционные тесты бэкапа. Внимание: Import ПОЛНОСТЬЮ перезаписывает БД,
// поэтому тесты создают себе отдельную базу (teamdocs_test_backup) — пакеты
// go test бегут параллельно, и общую teamdocs_test трогать нельзя.

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/backup"
	"team-docs/internal/db"
	"team-docs/internal/store"
)

func backupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEAMDOCS_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMDOCS_TEST_DSN не задан — пропускаю интеграционный тест")
	}
	ctx := context.Background()

	// Пересоздаём выделенную базу через основное подключение.
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS teamdocs_test_backup WITH (FORCE)`); err != nil {
		t.Fatalf("drop db: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE DATABASE teamdocs_test_backup`); err != nil {
		t.Fatalf("create db: %v", err)
	}

	pool, err := pgxpool.New(ctx, strings.Replace(dsn, "/teamdocs_test", "/teamdocs_test_backup", 1))
	if err != nil {
		t.Fatalf("подключение к teamdocs_test_backup: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("миграции: %v", err)
	}
	return pool
}

func TestBackupRoundTrip(t *testing.T) {
	pool := backupPool(t)
	ctx := context.Background()
	q := store.New(pool)
	svc := backup.New(pool)

	// Сеанс данных: пользователь, страница с авторством, ребёнок в корзине, ревизия.
	u, err := q.UpsertUser(ctx, store.UpsertUserParams{Subject: "t-user", Username: "tu", Name: "Тест", Email: "t@t"})
	if err != nil {
		t.Fatal(err)
	}
	mainP, err := q.GetProjectByKey(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}
	page, err := q.CreatePage(ctx, store.CreatePageParams{Title: "bk-root", AuthorID: &u.ID, ProjectID: mainP.ID})
	if err != nil {
		t.Fatal(err)
	}
	child, err := q.CreatePage(ctx, store.CreatePageParams{ParentID: &page.ID, Title: "bk-child", ProjectID: mainP.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.InsertRevision(ctx, store.InsertRevisionParams{
		PageID: page.ID, Version: 1, Title: "bk-root", Content: []byte(`[]`), AuthorID: &u.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE pages SET deleted_at = now() WHERE id = $1`, child.ID); err != nil {
		t.Fatal(err)
	}

	dump, err := svc.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if dump.Version != backup.DumpVersion {
		t.Fatalf("версия дампа %d, ожидалась %d", dump.Version, backup.DumpVersion)
	}

	if err := svc.Import(ctx, dump); err != nil {
		t.Fatalf("import: %v", err)
	}
	dump2, err := svc.Export(ctx)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}

	// Инварианты выживания round-trip.
	if len(dump2.Pages) != len(dump.Pages) || len(dump2.Users) != len(dump.Users) ||
		len(dump2.Projects) != len(dump.Projects) || len(dump2.Revisions) != len(dump.Revisions) {
		t.Fatalf("счётчики разошлись: pages %d/%d users %d/%d projects %d/%d revisions %d/%d",
			len(dump.Pages), len(dump2.Pages), len(dump.Users), len(dump2.Users),
			len(dump.Projects), len(dump2.Projects), len(dump.Revisions), len(dump2.Revisions))
	}
	var rootAfter, childAfter *backup.PageRow
	for i := range dump2.Pages {
		switch dump2.Pages[i].ID {
		case page.ID:
			rootAfter = &dump2.Pages[i]
		case child.ID:
			childAfter = &dump2.Pages[i]
		}
	}
	if rootAfter == nil || childAfter == nil {
		t.Fatal("страницы потерялись при round-trip")
	}
	if rootAfter.CreatedBy == nil || *rootAfter.CreatedBy != u.ID {
		t.Fatalf("авторство потерялось: %v", rootAfter.CreatedBy)
	}
	if childAfter.DeletedAt == nil {
		t.Fatal("deleted_at потерялся — удалённая страница воскресла")
	}
	if childAfter.ParentID == nil || *childAfter.ParentID != page.ID {
		t.Fatal("parent_id потерялся при round-trip")
	}
	if dump2.Revisions[0].AuthorID == nil || *dump2.Revisions[0].AuthorID != u.ID {
		t.Fatal("автор ревизии потерялся")
	}

	// Новая страница после импорта — sequence не должен конфликтовать.
	mainAfter, err := q.GetProjectByKey(ctx, "main")
	if err != nil {
		t.Fatalf("main после импорта: %v", err)
	}
	if _, err := q.CreatePage(ctx, store.CreatePageParams{Title: "bk-after-import", ProjectID: mainAfter.ID}); err != nil {
		t.Fatalf("создание после импорта (sequence): %v", err)
	}
}

func TestImportV1Dump(t *testing.T) {
	pool := backupPool(t)
	ctx := context.Background()
	svc := backup.New(pool)

	// v1-дамп: без users/projects/авторства/deleted_at (формат до этапа 0).
	raw := `{
		"version": 1,
		"exportedAt": "2026-01-01T00:00:00Z",
		"pages": [
			{"id": 10, "parentId": null, "title": "v1-root", "content": [],
			 "contentText": "", "position": 0, "version": 1, "icon": "",
			 "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z"},
			{"id": 11, "parentId": 10, "title": "v1-child", "content": [],
			 "contentText": "", "position": 0, "version": 1, "icon": "",
			 "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z"}
		],
		"revisions": [], "files": [], "diagrams": []
	}`
	var d backup.Dump
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	if err := svc.Import(ctx, &d); err != nil {
		t.Fatalf("import v1: %v", err)
	}

	// Страницы живы и уехали в дефолтный проект.
	q := store.New(pool)
	row, err := q.GetPage(ctx, 10)
	if err != nil {
		t.Fatalf("страница v1 после импорта: %v", err)
	}
	if row.Title != "v1-root" {
		t.Fatalf("title = %q", row.Title)
	}
	var projectKey string
	if err := pool.QueryRow(ctx,
		`SELECT pr.key FROM pages p JOIN projects pr ON pr.id = p.project_id WHERE p.id = 10`,
	).Scan(&projectKey); err != nil {
		t.Fatal(err)
	}
	if projectKey != "main" {
		t.Fatalf("v1-страница должна попасть в проект main, получено %q", projectKey)
	}
}

func TestImportRejectsBadDump(t *testing.T) {
	pool := backupPool(t)
	svc := backup.New(pool)
	ctx := context.Background()

	if err := svc.Import(ctx, nil); err == nil {
		t.Fatal("nil-дамп должен отклоняться")
	}
	if err := svc.Import(ctx, &backup.Dump{Version: 99, ExportedAt: time.Now()}); err == nil {
		t.Fatal("неизвестная версия должна отклоняться")
	}
}
