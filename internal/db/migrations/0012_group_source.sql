-- Источник группы (LDAP фаза 2): local — создана в админке; ldap — зеркало
-- группы каталога (создаётся при входе, членство синхронизируется с LDAP;
-- ручные правки состава таких групп перетираются синхронизацией).
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'local';
