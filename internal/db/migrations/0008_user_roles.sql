-- Роли пользователей для встроенной авторизации (ROADMAP §8):
-- reader (только чтение) | editor (правка) | admin (управление).
-- Дефолт editor сохраняет прежнее поведение «любой вошедший может писать»;
-- роль назначается админом (или бутстрапом через auth.adminEmails).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'editor';
