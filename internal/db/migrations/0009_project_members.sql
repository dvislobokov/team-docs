-- Ролевая модель проектов (ROADMAP §10).
-- Видимость: public (читают все, при publicRead — и анонимы) |
-- internal (все вошедшие) | private (только участники).
-- Роль участника перекрывает глобальную роль пользователя внутри проекта;
-- глобальный админ имеет полный доступ везде.
ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'internal';

CREATE TABLE IF NOT EXISTS project_members (
    project_id BIGINT      NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       TEXT        NOT NULL DEFAULT 'editor', -- reader | editor | admin
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
