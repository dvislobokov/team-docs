-- Пользователи и авторство. Identity приходит из JWT (или dev-пользователь в
-- открытом режиме); при первом запросе пользователь upsert-ится по subject.
-- Авторство nullable: данные, созданные до этой миграции, остаются без автора;
-- MCP-запись без identity тоже даёт NULL.
CREATE TABLE IF NOT EXISTS users (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subject      TEXT        NOT NULL UNIQUE, -- sub из JWT; "dev" в открытом режиме
    username     TEXT        NOT NULL DEFAULT '',
    name         TEXT        NOT NULL DEFAULT '',
    email        TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS created_by BIGINT REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_by BIGINT REFERENCES users (id) ON DELETE SET NULL;

ALTER TABLE page_revisions
    ADD COLUMN IF NOT EXISTS author_id BIGINT REFERENCES users (id) ON DELETE SET NULL;
