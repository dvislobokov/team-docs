-- Теги страниц (ROADMAP §7, этап 4): массив строк + GIN для фильтрации
-- (tags @> ARRAY[...]) и подсчёта популярных тегов.
ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX IF NOT EXISTS idx_pages_tags ON pages USING GIN (tags);
