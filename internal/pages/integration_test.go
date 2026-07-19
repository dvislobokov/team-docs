package pages_test

// Интеграционные тесты поверх реального Postgres (docker-compose.test.yml).
// Запуск:
//   docker compose -f docker-compose.test.yml up -d --wait
//   $env:TEAMDOCS_TEST_DSN = "postgres://teamdocs:teamdocs@localhost:54329/teamdocs_test?sslmode=disable"
//   go test ./internal/pages/
// Без TEAMDOCS_TEST_DSN тесты скипаются (юнит-часть бежит всегда).

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/db"
	"team-docs/internal/pages"
	"team-docs/internal/store"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEAMDOCS_TEST_DSN")
	if dsn == "" {
		t.Skip("TEAMDOCS_TEST_DSN не задан — пропускаю интеграционный тест")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("подключение к тестовой БД: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("миграции: %v", err)
	}
	return pool
}

func createPage(t *testing.T, pool *pgxpool.Pool, parent *int64, title string) store.CreatePageRow {
	t.Helper()
	q := store.New(pool)
	row, err := q.CreatePage(context.Background(), store.CreatePageParams{ParentID: parent, Title: title})
	if err != nil {
		t.Fatalf("создание страницы %q: %v", title, err)
	}
	// Жёсткая очистка после теста (FK-каскад добьёт поддерево).
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pages WHERE id = $1`, row.ID)
	})
	return row
}

func TestMoveReindexAndCycle(t *testing.T) {
	pool := testPool(t)

	q := store.New(pool)
	ctx := context.Background()

	root := createPage(t, pool, nil, "it-move-root")
	rootID := root.ID
	a := createPage(t, pool, &rootID, "it-move-a")
	b := createPage(t, pool, &rootID, "it-move-b")
	c := createPage(t, pool, &rootID, "it-move-c")

	// Исходный порядок: a(0), b(1), c(2). Переносим a на позицию 2 → b, c, a.
	if err := pages.Move(ctx, pool, a.ID, &rootID, 2); err != nil {
		t.Fatalf("move a: %v", err)
	}
	order := childOrder(t, q, rootID)
	want := []int64{b.ID, c.ID, a.ID}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("порядок после переноса: %v, ожидалось %v", order, want)
		}
	}

	// Позиции должны быть плотными: 0,1,2.
	for i, id := range order {
		meta, err := q.GetPageMeta(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if int(meta.Position) != i {
			t.Fatalf("позиция %d у страницы %d, ожидалось %d", meta.Position, id, i)
		}
	}

	// Цикл: перенос root под его внука запрещён.
	if err := pages.Move(ctx, pool, rootID, &c.ID, 0); !errors.Is(err, pages.ErrMoveIntoSubtree) {
		t.Fatalf("перенос в собственное поддерево должен давать ErrMoveIntoSubtree, получено: %v", err)
	}
	// Перенос в самого себя — тоже.
	if err := pages.Move(ctx, pool, rootID, &rootID, 0); !errors.Is(err, pages.ErrMoveIntoSubtree) {
		t.Fatalf("перенос в самого себя должен давать ErrMoveIntoSubtree, получено: %v", err)
	}
}

func childOrder(t *testing.T, q *store.Queries, parentID int64) []int64 {
	t.Helper()
	rows, err := q.GetPageTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, r := range rows {
		if r.ParentID != nil && *r.ParentID == parentID {
			out = append(out, r.ID)
		}
	}
	return out
}

func TestSearchRussianMorphology(t *testing.T) {
	pool := testPool(t)

	q := store.New(pool)
	ctx := context.Background()

	page := createPage(t, pool, nil, "it-search-Настройки поиска")
	if _, err := q.UpdatePage(ctx, store.UpdatePageParams{
		ID:          page.ID,
		Title:       page.Title,
		Content:     []byte(`[]`),
		ContentText: "Здесь описаны настройки полнотекстового поиска и индексов",
		Version:     page.Version,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Словоформа «настройка» должна находить «настройки» (russian-стемминг).
	hits, err := q.SearchPages(ctx, "настройка")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	found := false
	for _, h := range hits {
		if h.ID == page.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("поиск «настройка» не нашёл страницу с «настройки» (id=%d, hits=%d)", page.ID, len(hits))
	}
}

func TestTrashSoftDeleteRestore(t *testing.T) {
	pool := testPool(t)

	q := store.New(pool)
	ctx := context.Background()

	parent := createPage(t, pool, nil, "it-trash-parent")
	parentID := parent.ID
	child := createPage(t, pool, &parentID, "it-trash-child")

	if err := pages.SoftDelete(ctx, pool, parentID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Обе страницы скрыты из чтения.
	if _, err := q.GetPage(ctx, parentID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("удалённый родитель должен быть скрыт, получено: %v", err)
	}
	if _, err := q.GetPage(ctx, child.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("удалённый ребёнок должен быть скрыт, получено: %v", err)
	}

	// В корзине виден только корень поддерева.
	trash, err := q.ListTrash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var parentInTrash, childInTrash bool
	for _, item := range trash {
		if item.ID == parentID {
			parentInTrash = true
		}
		if item.ID == child.ID {
			childInTrash = true
		}
	}
	if !parentInTrash || childInTrash {
		t.Fatalf("в корзине должен быть только корень: parent=%v child=%v", parentInTrash, childInTrash)
	}

	// Восстановление ребёнка при удалённом родителе — в корень дерева.
	if err := pages.Restore(ctx, pool, child.ID); err != nil {
		t.Fatalf("restore child: %v", err)
	}
	meta, err := q.GetPageMeta(ctx, child.ID)
	if err != nil {
		t.Fatalf("ребёнок должен ожить: %v", err)
	}
	if meta.ParentID != nil {
		t.Fatalf("ребёнок должен восстановиться в корень, parent=%v", *meta.ParentID)
	}

	// Восстановление родителя возвращает его живым.
	if err := pages.Restore(ctx, pool, parentID); err != nil {
		t.Fatalf("restore parent: %v", err)
	}
	if _, err := q.GetPage(ctx, parentID); err != nil {
		t.Fatalf("родитель должен ожить: %v", err)
	}

	// Повторный SoftDelete + purge: из корзины страница уходит окончательно.
	if err := pages.SoftDelete(ctx, pool, parentID); err != nil {
		t.Fatal(err)
	}
	n, err := q.PurgePage(ctx, parentID)
	if err != nil || n == 0 {
		t.Fatalf("purge: n=%d err=%v", n, err)
	}
	if _, err := q.GetPageMeta(ctx, parentID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("после purge страницы быть не должно, получено: %v", err)
	}
}
