-- Скелет проектов (пространств) и таблица настроек. Только схема:
-- ролевая модель, переключатель проектов и админка — отдельные этапы
-- (ROADMAP §9–10). Все существующие страницы уезжают в дефолтный проект.
CREATE TABLE IF NOT EXISTS projects (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        TEXT        NOT NULL UNIQUE, -- машинный ключ (напр. "main")
    name       TEXT        NOT NULL,
    icon       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO projects (key, name)
VALUES ('main', 'Основное пространство')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE pages
    ADD COLUMN IF NOT EXISTS project_id BIGINT REFERENCES projects (id);

UPDATE pages
SET project_id = (SELECT id FROM projects WHERE key = 'main')
WHERE project_id IS NULL;

ALTER TABLE pages
    ALTER COLUMN project_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_pages_project ON pages (project_id);

-- Настройки приложения (под админку, ROADMAP §9). Приоритет источников:
-- конфиг (env/yaml) > БД > дефолты; значения из конфига в UI будут read-only.
CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users (id) ON DELETE SET NULL
);
