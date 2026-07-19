import { useEffect, useState } from "react";
import { Lock } from "lucide-react";
import {
  listSettings,
  listUsers,
  saveSetting,
  setUserRole,
  type AdminUser,
  type Setting,
} from "../api/admin";
import { relativeTime } from "../lib/format";
import { useAuth } from "../store/auth";
import { useToast } from "../store/toast";
import { Topbar } from "./Topbar";

const ROLES = [
  { value: "reader", label: "Читатель" },
  { value: "editor", label: "Редактор" },
  { value: "admin", label: "Администратор" },
];

// Страница администрирования (ROADMAP §9). Пока — пользователи и роли;
// секции настроек добавятся с переносом конфига в БД.
export function AdminScreen() {
  const me = useAuth();
  const toast = useToast();
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [busyId, setBusyId] = useState<number | null>(null);

  useEffect(() => {
    if (!me.isAdmin) return;
    const ctrl = new AbortController();
    listUsers(ctrl.signal)
      .then(setUsers)
      .catch(() => setUsers([]));
    return () => ctrl.abort();
  }, [me.isAdmin]);

  const changeRole = async (u: AdminUser, role: string) => {
    setBusyId(u.id);
    try {
      await setUserRole(u.id, role);
      setUsers((list) => list?.map((x) => (x.id === u.id ? { ...x, role } : x)) ?? null);
      toast(`Роль обновлена: ${u.name || u.username}`, "success");
    } catch {
      toast("Не удалось обновить роль", "error");
    } finally {
      setBusyId(null);
    }
  };

  if (!me.isAdmin) {
    return (
      <>
        <Topbar />
        <div className="mx-auto max-w-[560px] px-8 py-32 text-center text-muted">
          Доступ к администрированию есть только у администраторов.
        </div>
      </>
    );
  }

  return (
    <>
      <Topbar crumbs={[{ id: -1, title: "Администрирование", icon: "🛠️" }]} />
      <section className="animate-page-in mx-auto w-full max-w-[860px] px-6 py-10 md:py-14">
        <h1 className="font-display text-[30px] font-500 text-ink">Администрирование</h1>

        <SettingsSection />

        <h2 className="mt-10 text-[15px] font-600 text-ink">Пользователи и роли</h2>
        <p className="mt-1 text-[13px] text-muted">
          Читатель — только просмотр; редактор — правка страниц; администратор —
          плюс управление ролями. Роли действуют при включённой авторизации.
        </p>

        <div className="mt-4 overflow-x-auto rounded-xl border border-line">
          <table className="w-full text-[13px]">
            <thead>
              <tr className="border-b border-line text-left text-[12px] uppercase tracking-wide text-faint">
                <th className="px-4 py-2.5 font-500">Имя</th>
                <th className="px-4 py-2.5 font-500">Email</th>
                <th className="px-4 py-2.5 font-500">Был(а)</th>
                <th className="px-4 py-2.5 font-500">Роль</th>
              </tr>
            </thead>
            <tbody>
              {users === null && (
                <tr><td colSpan={4} className="px-4 py-6 text-faint">Загрузка…</td></tr>
              )}
              {users?.map((u) => (
                <tr key={u.id} className="border-b border-line/60 last:border-0">
                  <td className="px-4 py-2.5 text-ink">{u.name || u.username || u.subject}</td>
                  <td className="px-4 py-2.5 text-muted">{u.email}</td>
                  <td className="px-4 py-2.5 text-muted">{relativeTime(u.lastSeenAt)}</td>
                  <td className="px-4 py-2.5">
                    <select
                      value={u.role}
                      disabled={busyId === u.id}
                      onChange={(e) => changeRole(u, e.target.value)}
                      className="rounded-md border border-line bg-card px-2 py-1 text-[13px] text-body outline-none transition hover:border-faint"
                    >
                      {ROLES.map((r) => (
                        <option key={r.value} value={r.value}>{r.label}</option>
                      ))}
                    </select>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}

// Настройки приложения: значения из env/yaml показываются с замочком
// («управляется конфигурацией»), остальные редактируются и живут в БД.
function SettingsSection() {
  const toast = useToast();
  const [items, setItems] = useState<Setting[] | null>(null);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [savingKey, setSavingKey] = useState<string | null>(null);

  useEffect(() => {
    listSettings()
      .then(setItems)
      .catch(() => setItems([]));
  }, []);

  const save = async (s: Setting) => {
    const raw = draft[s.key];
    if (raw === undefined || raw === String(s.value)) return;
    const value = s.kind === "int" ? Number(raw) : raw;
    setSavingKey(s.key);
    try {
      await saveSetting(s.key, value);
      setItems((list) => list?.map((x) => (x.key === s.key ? { ...x, value, source: "db" } : x)) ?? null);
      toast(`Сохранено: ${s.label}`, "success");
    } catch (e) {
      toast(e instanceof Error ? e.message : "Не удалось сохранить", "error");
    } finally {
      setSavingKey(null);
    }
  };

  return (
    <>
      <h2 className="mt-8 text-[15px] font-600 text-ink">Настройки</h2>
      <p className="mt-1 text-[13px] text-muted">
        Значения с замочком заданы в конфигурации (env/yaml) и редактируются
        только там; остальные сохраняются в базе и применяются сразу.
      </p>
      <div className="mt-4 flex flex-col gap-2.5">
        {items === null && <div className="text-[13px] text-faint">Загрузка…</div>}
        {items?.map((s) => (
          <div key={s.key} className="flex items-center gap-3">
            <label className="w-[220px] shrink-0 text-[13px] text-body" title={s.key}>
              {s.label}
            </label>
            <input
              value={draft[s.key] ?? String(s.value)}
              disabled={!s.editable || savingKey === s.key}
              onChange={(e) => setDraft((d) => ({ ...d, [s.key]: e.target.value }))}
              onBlur={() => save(s)}
              onKeyDown={(e) => e.key === "Enter" && (e.target as HTMLInputElement).blur()}
              className="min-w-0 flex-1 rounded-md border border-line bg-card px-2.5 py-1.5 text-[13px] text-ink outline-none transition hover:border-faint focus:border-accent disabled:bg-line/30 disabled:text-muted"
            />
            {!s.editable && (
              <span
                className="flex shrink-0 items-center gap-1 text-[11px] text-faint"
                title={`Задано в конфигурации (${s.source})`}
              >
                <Lock className="h-3.5 w-3.5" /> {s.source}
              </span>
            )}
          </div>
        ))}
      </div>
    </>
  );
}
