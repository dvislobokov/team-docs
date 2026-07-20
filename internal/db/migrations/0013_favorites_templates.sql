-- Избранное и шаблоны страниц (ROADMAP §7, этап 4).

-- Избранные страницы пользователя (звезда в топбаре, секция в сайдбаре).
CREATE TABLE IF NOT EXISTS favorites (
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    page_id    BIGINT      NOT NULL REFERENCES pages (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, page_id)
);

-- Шаблоны — обычные страницы в служебном разделе: is_template = TRUE,
-- всегда корневые (parent_id IS NULL), скрыты из дерева/поиска/недавних.
ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS is_template BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_pages_templates
    ON pages (project_id) WHERE is_template;
