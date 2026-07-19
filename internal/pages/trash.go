package pages

import (
	"context"
	"time"

	"github.com/dvislobokov/srog"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/store"
)

func toTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// trashRetention — сколько страница живёт в корзине до автоочистки.
const trashRetention = 30 * 24 * time.Hour

// softDeleteSQL помечает удалённым всё живое поддерево корня $1.
// Рекурсивный CTE — сырой SQL (не проходит анализатор sqlc).
const softDeleteSQL = `
WITH RECURSIVE sub AS (
    SELECT id FROM pages WHERE id = $1 AND deleted_at IS NULL
    UNION ALL
    SELECT p.id FROM pages p JOIN sub ON p.parent_id = sub.id
    WHERE p.deleted_at IS NULL
)
UPDATE pages SET deleted_at = now() WHERE id IN (SELECT id FROM sub)`

// restoreSubtreeSQL снимает пометку удаления со всего поддерева корня $1.
const restoreSubtreeSQL = `
WITH RECURSIVE sub AS (
    SELECT id FROM pages WHERE id = $1 AND deleted_at IS NOT NULL
    UNION ALL
    SELECT p.id FROM pages p JOIN sub ON p.parent_id = sub.id
    WHERE p.deleted_at IS NOT NULL
)
UPDATE pages SET deleted_at = NULL WHERE id IN (SELECT id FROM sub)`

// SoftDelete перемещает страницу с поддеревом в корзину.
// Возвращает ErrPageNotFound, если живой страницы с таким id нет.
func SoftDelete(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	tag, err := pool.Exec(ctx, softDeleteSQL, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPageNotFound
	}
	return nil
}

// Restore возвращает поддерево из корзины. Если исходный родитель тоже
// в корзине (или уже вычищен) — корень восстанавливается в корень дерева.
// Позиция — в конец списка детей нового родителя.
func Restore(ctx context.Context, pool *pgxpool.Pool, id int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op после успешного Commit

	// Корень должен лежать в корзине; заодно читаем его родителя.
	var parentID *int64
	err = tx.QueryRow(ctx,
		`SELECT parent_id FROM pages WHERE id = $1 AND deleted_at IS NOT NULL`, id,
	).Scan(&parentID)
	if err != nil {
		return ErrPageNotFound
	}

	// Родитель мёртв/удалён → восстанавливаем в корень.
	if parentID != nil {
		var alive bool
		err = tx.QueryRow(ctx,
			`SELECT deleted_at IS NULL FROM pages WHERE id = $1`, *parentID,
		).Scan(&alive)
		if err != nil || !alive {
			parentID = nil
		}
	}

	if _, err := tx.Exec(ctx, restoreSubtreeSQL, id); err != nil {
		return err
	}

	// Корень — в конец списка живых детей целевого родителя.
	if _, err := tx.Exec(ctx, `
		UPDATE pages
		SET parent_id = $2,
		    position = COALESCE(
		        (SELECT MAX(position) + 1 FROM pages
		         WHERE parent_id IS NOT DISTINCT FROM $2
		           AND deleted_at IS NULL AND id <> $1),
		        0)
		WHERE id = $1`, id, parentID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RunTrashJanitor чистит корзину при старте и раз в сутки: всё, что удалено
// раньше trashRetention назад, стирается окончательно. Блокирует горутину.
func RunTrashJanitor(ctx context.Context, pool *pgxpool.Pool, log *srog.Logger) {
	q := store.New(pool)
	purge := func() {
		n, err := q.PurgeExpired(ctx, toTimestamptz(time.Now().Add(-trashRetention)))
		if err != nil {
			log.Error(err, "trash: janitor purge failed")
			return
		}
		if n > 0 {
			log.Information("trash: purged {Count} expired page(s)", n)
		}
	}
	purge()
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			purge()
		}
	}
}
