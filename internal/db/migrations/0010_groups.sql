-- Локальные группы пользователей (ROADMAP §8): права в проектах можно
-- выдавать группе, а не только человеку. Приоритет при вычислении роли:
-- личное членство в проекте > групповое членство > видимость проекта.
CREATE TABLE IF NOT EXISTS groups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    user_id  BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS project_group_members (
    project_id BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    group_id   BIGINT NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    role       TEXT   NOT NULL DEFAULT 'editor', -- reader | editor | admin
    PRIMARY KEY (project_id, group_id)
);
