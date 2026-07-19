-- Корзина: мягкое удаление вместо безвозвратного каскада.
-- DELETE помечает всё поддерево deleted_at; восстановление снимает пометку;
-- окончательное удаление (purge) и автоочистка старше 30 дней — жёсткий DELETE
-- (FK-каскад добивает поддерево).
ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Частичный индекс: живые страницы (все основные запросы фильтруют по нему).
DROP INDEX IF EXISTS idx_pages_parent;
CREATE INDEX IF NOT EXISTS idx_pages_parent
    ON pages (parent_id, position) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pages_deleted
    ON pages (deleted_at) WHERE deleted_at IS NOT NULL;
