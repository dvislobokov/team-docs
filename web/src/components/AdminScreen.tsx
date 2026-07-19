import { useEffect, useState } from "react";
import { listUsers, setUserRole, type AdminUser } from "../api/admin";
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

        <h2 className="mt-8 text-[15px] font-600 text-ink">Пользователи и роли</h2>
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
