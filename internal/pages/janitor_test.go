package pages_test

// Интеграционные тесты фоновой уборки: прореживание ревизий и GC файлов.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"team-docs/internal/pages"
)

func TestPruneRevisions(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	page := createPage(t, pool, nil, "it-prune")

	// Бэкдейтим ревизии сырым INSERT: два снапшота в один старый день,
	// один в другой старый день, один свежий.
	old := time.Now().Add(-60 * 24 * time.Hour).Truncate(24 * time.Hour)
	insert := func(at time.Time, v int32) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO page_revisions (page_id, version, title, content, created_at)
			VALUES ($1, $2, 'it-prune', '[]', $3)`, page.ID, v, at); err != nil {
			t.Fatal(err)
		}
	}
	insert(old.Add(10*time.Hour), 1) // день 1, утро — должен уйти
	insert(old.Add(12*time.Hour), 2) // день 1, позже — остаётся (свежайший за день)
	insert(old.Add(34*time.Hour), 3) // день 2 — остаётся
	insert(time.Now(), 4)            // свежий — не трогаем (моложе отсечки)

	n, err := pages.PruneRevisions(ctx, pool, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n != 1 {
		t.Fatalf("prune удалил %d ревизий, ожидалась 1", n)
	}

	var versions []int32
	rows, err := pool.Query(ctx,
		`SELECT version FROM page_revisions WHERE page_id = $1 ORDER BY version`, page.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int32
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	want := []int32{2, 3, 4}
	if fmt.Sprint(versions) != fmt.Sprint(want) {
		t.Fatalf("остались версии %v, ожидались %v", versions, want)
	}
}

func TestGCFiles(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	referenced := uuid.New()
	orphanOld := uuid.New()
	orphanFresh := uuid.New()

	old := time.Now().Add(-48 * time.Hour)
	insertFile := func(id uuid.UUID, at time.Time) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO files (id, filename, mime, size, content, created_at)
			VALUES ($1, 'f.png', 'image/png', 1, '\x00'::bytea, $2)`, id, at); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM files WHERE id = $1`, id) })
	}
	insertFile(referenced, old)
	insertFile(orphanOld, old)
	insertFile(orphanFresh, time.Now())

	// Страница ссылается на referenced в контенте (как это делает редактор).
	page := createPage(t, pool, nil, "it-gc-files")
	content := fmt.Sprintf(`[{"type":"image","props":{"url":"/api/files/%s"}}]`, referenced)
	if _, err := pool.Exec(ctx,
		`UPDATE pages SET content = $2 WHERE id = $1`, page.ID, content); err != nil {
		t.Fatal(err)
	}

	n, err := pages.GCFiles(ctx, pool, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if n != 1 {
		t.Fatalf("gc удалил %d файлов, ожидался 1 (старый сирота)", n)
	}

	exists := func(id uuid.UUID) bool {
		var ok bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM files WHERE id = $1)`, id).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !exists(referenced) {
		t.Fatal("файл, на который ссылается страница, не должен удаляться")
	}
	if exists(orphanOld) {
		t.Fatal("старый сирота должен быть удалён")
	}
	if !exists(orphanFresh) {
		t.Fatal("свежий файл в grace-периоде не должен удаляться")
	}
}
