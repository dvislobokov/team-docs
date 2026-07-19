package pages

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"team-docs/internal/store"
)

// Ошибки Move — для маппинга в HTTP-статусы и ответы MCP-инструментов.
var (
	ErrPageNotFound    = errors.New("page not found")
	ErrMoveIntoSubtree = errors.New("cannot move page into its own subtree")
)

// isInSubtreeSQL: находится ли candidate ($2) в поддереве root ($1), включая
// candidate = root. Сырой SQL: рекурсивный CTE не проходит анализатор sqlc.
const isInSubtreeSQL = `
WITH RECURSIVE anc AS (
    SELECT id, parent_id FROM pages WHERE id = $2
    UNION ALL
    SELECT p.id, p.parent_id FROM pages p JOIN anc ON p.id = anc.parent_id
)
SELECT EXISTS(SELECT 1 FROM anc WHERE anc.id = $1)`

// Move переносит страницу атомарно: проверка цикла, кламп позиции и
// переиндексация соседей в старом и новом родителе — в одной транзакции.
// position — индекс среди детей нового родителя без учёта самой страницы
// (семантика planMove на фронте). Используется HTTP-хендлером и MCP.
func Move(ctx context.Context, pool *pgxpool.Pool, id int64, parentID *int64, position int32) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op после успешного Commit

	q := store.New(pool).WithTx(tx)

	meta, err := q.GetPageMeta(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrPageNotFound
	}
	if err != nil {
		return err
	}

	if parentID != nil {
		var inSubtree bool
		if err := tx.QueryRow(ctx, isInSubtreeSQL, id, *parentID).Scan(&inSubtree); err != nil {
			return err
		}
		if inSubtree {
			return ErrMoveIntoSubtree
		}
	}

	siblings, err := q.CountSiblings(ctx, store.CountSiblingsParams{ParentID: parentID, PageID: id})
	if err != nil {
		return err
	}
	if position < 0 {
		position = 0
	}
	if int64(position) > siblings {
		position = int32(siblings)
	}

	// «Изъять из старого места, вставить в новое»: сдвигаем соседей по бокам.
	if err := q.ShiftAfterRemove(ctx, store.ShiftAfterRemoveParams{
		ParentID: meta.ParentID, Position: meta.Position, PageID: id,
	}); err != nil {
		return err
	}
	if err := q.ShiftForInsert(ctx, store.ShiftForInsertParams{
		ParentID: parentID, Position: position, PageID: id,
	}); err != nil {
		return err
	}
	if err := q.MovePage(ctx, store.MovePageParams{ID: id, ParentID: parentID, Position: position}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
