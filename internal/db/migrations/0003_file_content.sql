-- Содержимое файлов храним прямо в БД (BYTEA), без диска. Существующие строки
-- получают пустой контент (их файлы на диске больше не читаются).
ALTER TABLE files ADD COLUMN IF NOT EXISTS content BYTEA NOT NULL DEFAULT ''::bytea;
