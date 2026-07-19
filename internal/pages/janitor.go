package pages

// Фоновая уборка БД: корзина, старые ревизии, осиротевшие файлы.
// Запускается при старте и далее раз в сутки (см. RunJanitor).

import (
	"context"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/store"
)

const (
	// trashRetention — сколько страница живёт в корзине до автоочистки.
	trashRetention = 30 * 24 * time.Hour
	// revisionRetention — старше этого срока ревизии прореживаются
	// до одной (самой свежей) в день на страницу.
	revisionRetention = 30 * 24 * time.Hour
	// fileGraceperiod — файл моложе этого срока не считается сиротой:
	// загрузка могла ещё не попасть в сохранённый контент.
	fileGraceperiod = 24 * time.Hour
)

// pruneRevisionsSQL прореживает ревизии старше отсечки $1: на каждую пару
// (страница, день) остаётся только самая свежая.
const pruneRevisionsSQL = `
DELETE FROM page_revisions
WHERE created_at < $1
  AND id NOT IN (
      SELECT DISTINCT ON (page_id, date_trunc('day', created_at)) id
      FROM page_revisions
      WHERE created_at < $1
      ORDER BY page_id, date_trunc('day', created_at), created_at DESC
  )`

// PruneRevisions прореживает старые снапшоты. Возвращает число удалённых.
func PruneRevisions(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) (int64, error) {
	tag, err := pool.Exec(ctx, pruneRevisionsSQL, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// gcFilesSQL удаляет файлы старше отсечки $1, на которые нет ссылок ни в
// контенте страниц (включая корзину), ни в ревизиях. Ссылка — вхождение UUID
// в JSON контента (URL вида /api/files/<uuid>).
const gcFilesSQL = `
DELETE FROM files f
WHERE f.created_at < $1
  AND NOT EXISTS (
      SELECT 1 FROM pages p WHERE p.content::text LIKE '%' || f.id::text || '%')
  AND NOT EXISTS (
      SELECT 1 FROM page_revisions r WHERE r.content::text LIKE '%' || f.id::text || '%')`

// GCFiles удаляет осиротевшие файлы. Возвращает число удалённых.
func GCFiles(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) (int64, error) {
	tag, err := pool.Exec(ctx, gcFilesSQL, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// RunJanitor выполняет уборку при старте и далее раз в сутки. Блокирует
// горутину. Порядок важен: сначала корзина и ревизии (убирают ссылки на
// файлы), затем GC файлов.
func RunJanitor(ctx context.Context, pool *pgxpool.Pool, log *srog.Logger) {
	q := store.New(pool)
	sweep := func() {
		now := time.Now()
		if n, err := q.PurgeExpired(ctx, toTimestamptz(now.Add(-trashRetention))); err != nil {
			log.Error(err, "janitor: trash purge failed")
		} else if n > 0 {
			log.Information("janitor: purged {Count} expired page(s) from trash", n)
		}
		if n, err := PruneRevisions(ctx, pool, now.Add(-revisionRetention)); err != nil {
			log.Error(err, "janitor: revision prune failed")
		} else if n > 0 {
			log.Information("janitor: pruned {Count} old revision(s)", n)
		}
		if n, err := GCFiles(ctx, pool, now.Add(-fileGraceperiod)); err != nil {
			log.Error(err, "janitor: file gc failed")
		} else if n > 0 {
			log.Information("janitor: removed {Count} orphaned file(s)", n)
		}
	}
	sweep()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}
