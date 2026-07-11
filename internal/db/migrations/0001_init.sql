-- pages: дерево страниц через parent_id (adjacency list)
CREATE TABLE IF NOT EXISTS pages (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id    BIGINT REFERENCES pages (id) ON DELETE CASCADE,
    title        TEXT        NOT NULL DEFAULT 'Untitled',
    content      JSONB       NOT NULL DEFAULT '[]', -- документ BlockNote
    content_text TEXT        NOT NULL DEFAULT '',   -- плоский текст для поиска
    position     INT         NOT NULL DEFAULT 0,    -- порядок среди сиблингов
    version      INT         NOT NULL DEFAULT 1,    -- optimistic lock
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pages_parent ON pages (parent_id, position);
CREATE INDEX IF NOT EXISTS idx_pages_search ON pages USING GIN (to_tsvector('simple', content_text));

-- история версий (снапшоты, пишутся по троттлингу)
CREATE TABLE IF NOT EXISTS page_revisions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    page_id    BIGINT      NOT NULL REFERENCES pages (id) ON DELETE CASCADE,
    version    INT         NOT NULL,
    title      TEXT        NOT NULL,
    content    JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_revisions_page ON page_revisions (page_id, version DESC);

-- загруженные файлы/картинки из редактора
CREATE TABLE IF NOT EXISTS files (
    id         UUID PRIMARY KEY,
    page_id    BIGINT REFERENCES pages (id) ON DELETE SET NULL,
    filename   TEXT        NOT NULL,
    mime       TEXT        NOT NULL,
    size       BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- draw.io диаграммы: XML — источник правды, preview — PNG для отображения
CREATE TABLE IF NOT EXISTS diagrams (
    id         UUID PRIMARY KEY,
    page_id    BIGINT REFERENCES pages (id) ON DELETE CASCADE,
    xml        TEXT        NOT NULL DEFAULT '',
    preview    BYTEA,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
