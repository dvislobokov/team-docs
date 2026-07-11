-- Иконка страницы (emoji). Пустая строка = дефолтная иконка на фронте.
ALTER TABLE pages ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '';
