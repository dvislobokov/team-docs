-- Полнотекстовый поиск: генерируемая tsvector-колонка с русской морфологией.
-- Прежний индекс был построен по другому выражению, чем фильтр в SearchPages
-- (content_text без title), и потому не использовался; конфиг 'simple'
-- не давал стемминга («настройка» не находила «настройки»).
-- Конфиг 'russian' стеммит русские слова, латиница идёт через english_stem.
ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('russian', title || ' ' || content_text)) STORED;

DROP INDEX IF EXISTS idx_pages_search;
CREATE INDEX IF NOT EXISTS idx_pages_search ON pages USING GIN (search_vector);
