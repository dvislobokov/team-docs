import { useEffect, useRef, useState } from "react";
import { Lock } from "lucide-react";
import {
  addGroupMember,
  authCheck,
  createGroup,
  deleteGroup,
  listGroupMembers,
  listGroups,
  listSettings,
  listUsers,
  removeGroupMember,
  runCleanup,
  saveSetting,
  setUserRole,
  type AdminUser,
  type Group,
  type Setting,
} from "../api/admin";
import { downloadBackup, importBackup } from "../api/backup";
import { ApiError } from "../api/client";
import { useConfirm } from "../store/confirm";
import {
  createProject,
  listMembers,
  listProjectGroups,
  listProjects,
  removeMember,
  removeProjectGroup,
  setMember,
  setProjectGroup,
  updateProject,
  type Project,
  type ProjectGroup,
  type ProjectMember,
} from "../api/projects";
import { relativeTime } from "../lib/format";
import { useAuth } from "../store/auth";
import { useToast } from "../store/toast";
import { useTree } from "../store/tree";
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

        <ProjectsSection />

        <GroupsSection />

        <DataSection />

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

// «Данные»: экспорт/импорт всей БД (переехали из меню оформления) и ручной
// запуск уборки; плюс диагностика конфигурации авторизации.
function DataSection() {
  const toast = useToast();
  const confirm = useConfirm();
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [check, setCheck] = useState<string | null>(null);

  const onImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    const ok = await confirm({
      title: "Импортировать резервную копию?",
      message:
        "Текущее содержимое БД (страницы, версии, файлы, пользователи, проекты) будет ПОЛНОСТЬЮ заменено данными из файла. Действие необратимо.",
      confirmLabel: "Заменить и импортировать",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      const r = await importBackup(file);
      toast(`Импортировано: ${r.pages} стр., ${r.files} файлов`, "success");
      setTimeout(() => (window.location.href = "/"), 600);
    } catch (err) {
      toast(err instanceof ApiError ? err.message : "не удалось импортировать", "error");
    } finally {
      setBusy(false);
    }
  };

  const cleanup = async () => {
    setBusy(true);
    try {
      const r = await runCleanup();
      toast(
        `Уборка: корзина ${r.trashPurged}, ревизии ${r.revisionsPruned}, файлы ${r.filesRemoved}`,
        "success",
      );
    } catch {
      toast("Уборка завершилась с ошибками", "error");
    } finally {
      setBusy(false);
    }
  };

  const doCheck = async () => {
    try {
      setCheck(JSON.stringify(await authCheck(), null, 2));
    } catch {
      setCheck("Проверка не удалась");
    }
  };

  return (
    <>
      <h2 className="mt-10 text-[15px] font-600 text-ink">Данные и обслуживание</h2>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <button type="button" onClick={() => downloadBackup()}
          className="rounded-md border border-line px-3 py-1.5 text-[13px] text-body hover:border-faint">
          Экспорт БД (.json)
        </button>
        <button type="button" disabled={busy} onClick={() => fileRef.current?.click()}
          className="rounded-md border border-line px-3 py-1.5 text-[13px] text-body hover:border-faint disabled:opacity-50">
          {busy ? "…" : "Импорт БД (перезапись)"}
        </button>
        <input ref={fileRef} type="file" accept="application/json,.json" className="hidden" onChange={onImportFile} />
        <button type="button" disabled={busy} onClick={cleanup}
          className="rounded-md border border-line px-3 py-1.5 text-[13px] text-body hover:border-faint disabled:opacity-50">
          Запустить уборку
        </button>
        <button type="button" onClick={doCheck}
          className="rounded-md border border-line px-3 py-1.5 text-[13px] text-body hover:border-faint">
          Проверить авторизацию
        </button>
      </div>
      <p className="mt-1.5 text-[12px] text-muted">
        Уборка удаляет просроченную корзину, прореживает старые ревизии и
        чистит осиротевшие файлы (то же происходит автоматически раз в сутки).
      </p>
      {check && (
        <pre className="mt-2 overflow-x-auto rounded-lg border border-line bg-card p-3 font-mono text-[12px] text-body">{check}</pre>
      )}
    </>
  );
}

// Локальные группы (§8): состав группы; права группе в проекте выдаются
// через API проекта (PUT /api/projects/:id/groups/:groupId).
function GroupsSection() {
  const toast = useToast();
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [newName, setNewName] = useState("");
  const [openId, setOpenId] = useState<number | null>(null);
  const [members, setMembers] = useState<AdminUser[]>([]);
  const [allUsers, setAllUsers] = useState<AdminUser[]>([]);
  const [addId, setAddId] = useState("");

  const load = () => {
    listGroups().then(setGroups).catch(() => setGroups([]));
  };
  useEffect(load, []);
  useEffect(() => {
    listUsers().then(setAllUsers).catch(() => undefined);
  }, []);

  const openMembers = async (g: Group) => {
    if (openId === g.id) return setOpenId(null);
    setMembers(await listGroupMembers(g.id).catch(() => []));
    setOpenId(g.id);
  };

  return (
    <>
      <h2 className="mt-10 text-[15px] font-600 text-ink">Группы</h2>
      <p className="mt-1 text-[13px] text-muted">
        Группе можно выдать роль в проекте (секция «Проекты» → участники);
        личное членство в проекте приоритетнее группового.
      </p>
      <div className="mt-4 flex flex-col gap-2">
        {groups?.map((g) => (
          <div key={g.id} className="rounded-xl border border-line px-4 py-2.5">
            <div className="flex items-center gap-3">
              <span className="min-w-0 flex-1 truncate text-[14px] text-ink">
                {g.name} <span className="text-[12px] text-faint">({g.members})</span>
              </span>
              <button type="button" onClick={() => openMembers(g)}
                className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body hover:border-faint">
                Состав
              </button>
              <button type="button"
                onClick={async () => { await deleteGroup(g.id); toast("Группа удалена", "success"); load(); }}
                className="rounded px-1.5 text-[12px] text-red-500 hover:bg-red-500/10">
                Удалить
              </button>
            </div>
            {openId === g.id && (
              <div className="mt-2 border-t border-line/60 pt-2">
                {members.map((m) => (
                  <div key={m.id} className="flex items-center gap-2 py-0.5">
                    <span className="min-w-0 flex-1 truncate text-[13px] text-body">
                      {m.name || m.username} <span className="text-faint">{m.email}</span>
                    </span>
                    <button type="button"
                      onClick={async () => {
                        await removeGroupMember(g.id, m.id);
                        setMembers((l) => l.filter((x) => x.id !== m.id));
                        load();
                      }}
                      className="rounded px-1.5 text-[12px] text-red-500 hover:bg-red-500/10">
                      Убрать
                    </button>
                  </div>
                ))}
                <div className="mt-1.5 flex items-center gap-2">
                  <select value={addId} onChange={(e) => setAddId(e.target.value)}
                    className="min-w-0 flex-1 rounded-md border border-line bg-card px-2 py-1 text-[12px] text-body outline-none">
                    <option value="">Добавить в группу…</option>
                    {allUsers.filter((u) => !members.some((m) => m.id === u.id)).map((u) => (
                      <option key={u.id} value={u.id}>{u.name || u.username} ({u.email || u.subject})</option>
                    ))}
                  </select>
                  <button type="button" disabled={!addId}
                    onClick={async () => {
                      await addGroupMember(g.id, Number(addId));
                      setMembers(await listGroupMembers(g.id).catch(() => []));
                      setAddId("");
                      load();
                    }}
                    className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body hover:border-faint disabled:opacity-50">
                    Добавить
                  </button>
                </div>
              </div>
            )}
          </div>
        ))}
        <div className="flex items-center gap-2">
          <input value={newName} onChange={(e) => setNewName(e.target.value)}
            placeholder="Название группы"
            className="min-w-0 flex-1 rounded-md border border-line bg-card px-2.5 py-1.5 text-[13px] text-ink outline-none focus:border-accent" />
          <button type="button" disabled={!newName}
            onClick={async () => {
              try {
                await createGroup(newName);
                setNewName("");
                toast("Группа создана", "success");
                load();
              } catch { toast("Не удалось создать группу", "error"); }
            }}
            className="rounded-md bg-accent px-3 py-1.5 text-[13px] font-500 text-white hover:bg-accent/90 disabled:opacity-50">
            Создать
          </button>
        </div>
      </div>
    </>
  );
}

const VISIBILITIES = [
  { value: "public", label: "Публичный" },
  { value: "internal", label: "Внутренний" },
  { value: "private", label: "Приватный" },
];

// Проекты: создание, видимость, участники с ролями (§10).
function ProjectsSection() {
  const toast = useToast();
  const { reloadProjects } = useTree();
  const [items, setItems] = useState<Project[] | null>(null);
  const [newKey, setNewKey] = useState("");
  const [newName, setNewName] = useState("");
  const [openId, setOpenId] = useState<number | null>(null);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [projGroups, setProjGroups] = useState<ProjectGroup[]>([]);
  const [allUsers, setAllUsers] = useState<AdminUser[]>([]);
  const [allGroups, setAllGroups] = useState<Group[]>([]);
  const [addUserId, setAddUserId] = useState("");
  const [addGroupId, setAddGroupId] = useState("");

  const load = () => {
    listProjects()
      .then(setItems)
      .catch(() => setItems([]));
  };
  useEffect(load, []);
  useEffect(() => {
    listUsers().then(setAllUsers).catch(() => undefined);
  }, []);

  const create = async () => {
    if (!newKey || !newName) return;
    try {
      await createProject({ key: newKey, name: newName });
      setNewKey("");
      setNewName("");
      toast("Проект создан", "success");
      load();
      void reloadProjects(); // селектор в сайдбаре видит новый проект сразу
    } catch (e) {
      toast(e instanceof Error ? e.message : "Не удалось создать проект", "error");
    }
  };

  const changeVisibility = async (p: Project, visibility: string) => {
    try {
      await updateProject(p.id, { visibility });
      setItems((list) => list?.map((x) => (x.id === p.id ? { ...x, visibility: visibility as Project["visibility"] } : x)) ?? null);
      toast(`Видимость обновлена: ${p.name}`, "success");
    } catch {
      toast("Не удалось обновить проект", "error");
    }
  };

  const toggleMembers = async (p: Project) => {
    if (openId === p.id) {
      setOpenId(null);
      return;
    }
    setMembers(await listMembers(p.id).catch(() => []));
    setProjGroups(await listProjectGroups(p.id).catch(() => []));
    listGroups().then(setAllGroups).catch(() => undefined);
    setOpenId(p.id);
  };

  const addMember = async (p: Project) => {
    const userId = Number(addUserId);
    if (!userId) return;
    await setMember(p.id, userId, "editor");
    setMembers(await listMembers(p.id).catch(() => []));
    setAddUserId("");
  };

  const changeMemberRole = async (p: Project, m: ProjectMember, role: string) => {
    await setMember(p.id, m.userId, role);
    setMembers((list) => list.map((x) => (x.userId === m.userId ? { ...x, role } : x)));
  };

  const drop = async (p: Project, m: ProjectMember) => {
    await removeMember(p.id, m.userId);
    setMembers((list) => list.filter((x) => x.userId !== m.userId));
  };

  return (
    <>
      <h2 className="mt-10 text-[15px] font-600 text-ink">Проекты</h2>
      <p className="mt-1 text-[13px] text-muted">
        Публичный — читают все; внутренний — все вошедшие; приватный — только
        участники. Роль участника действует вместо глобальной внутри проекта.
      </p>

      <div className="mt-4 flex flex-col gap-2">
        {items?.map((p) => (
          <div key={p.id} className="rounded-xl border border-line px-4 py-3">
            <div className="flex items-center gap-3">
              <span className="min-w-0 flex-1 truncate text-[14px] text-ink">
                {p.icon ? `${p.icon} ` : ""}{p.name}{" "}
                <span className="font-mono text-[11px] text-faint">{p.key}</span>
              </span>
              <select
                value={p.visibility}
                onChange={(e) => changeVisibility(p, e.target.value)}
                className="rounded-md border border-line bg-card px-2 py-1 text-[12px] text-body outline-none hover:border-faint"
              >
                {VISIBILITIES.map((v) => (
                  <option key={v.value} value={v.value}>{v.label}</option>
                ))}
              </select>
              <button
                type="button"
                onClick={() => toggleMembers(p)}
                className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body transition hover:border-faint"
              >
                Участники
              </button>
            </div>

            {openId === p.id && (
              <div className="mt-3 border-t border-line/60 pt-3">
                {members.length === 0 && (
                  <p className="text-[12px] text-faint">
                    Участников нет — действуют глобальные роли (для приватного проекта это «нет доступа»).
                  </p>
                )}
                {members.map((m) => (
                  <div key={m.userId} className="flex items-center gap-2 py-1">
                    <span className="min-w-0 flex-1 truncate text-[13px] text-body">
                      {m.name || m.username} <span className="text-faint">{m.email}</span>
                    </span>
                    <select
                      value={m.role}
                      onChange={(e) => changeMemberRole(p, m, e.target.value)}
                      className="rounded-md border border-line bg-card px-2 py-0.5 text-[12px] text-body outline-none"
                    >
                      {ROLES.map((r) => (
                        <option key={r.value} value={r.value}>{r.label}</option>
                      ))}
                    </select>
                    <button
                      type="button"
                      onClick={() => drop(p, m)}
                      className="rounded px-1.5 text-[12px] text-red-500 hover:bg-red-500/10"
                    >
                      Убрать
                    </button>
                  </div>
                ))}
                <div className="mt-2 flex items-center gap-2">
                  <select
                    value={addUserId}
                    onChange={(e) => setAddUserId(e.target.value)}
                    className="min-w-0 flex-1 rounded-md border border-line bg-card px-2 py-1 text-[12px] text-body outline-none"
                  >
                    <option value="">Добавить участника…</option>
                    {allUsers
                      .filter((u) => !members.some((m) => m.userId === u.id))
                      .map((u) => (
                        <option key={u.id} value={u.id}>
                          {u.name || u.username} ({u.email || u.subject})
                        </option>
                      ))}
                  </select>
                  <button
                    type="button"
                    onClick={() => addMember(p)}
                    disabled={!addUserId}
                    className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body transition hover:border-faint disabled:opacity-50"
                  >
                    Добавить
                  </button>
                </div>

                {/* Группы проекта: роль всей группе разом. */}
                <div className="mt-3 border-t border-line/60 pt-2">
                  <div className="mb-1 text-[11px] font-600 uppercase tracking-wide text-faint">Группы</div>
                  {projGroups.map((g) => (
                    <div key={g.groupId} className="flex items-center gap-2 py-1">
                      <span className="min-w-0 flex-1 truncate text-[13px] text-body">{g.name}</span>
                      <select
                        value={g.role}
                        onChange={async (e) => {
                          await setProjectGroup(p.id, g.groupId, e.target.value);
                          setProjGroups((l) => l.map((x) => (x.groupId === g.groupId ? { ...x, role: e.target.value } : x)));
                        }}
                        className="rounded-md border border-line bg-card px-2 py-0.5 text-[12px] text-body outline-none"
                      >
                        {ROLES.map((r) => (
                          <option key={r.value} value={r.value}>{r.label}</option>
                        ))}
                      </select>
                      <button
                        type="button"
                        onClick={async () => {
                          await removeProjectGroup(p.id, g.groupId);
                          setProjGroups((l) => l.filter((x) => x.groupId !== g.groupId));
                        }}
                        className="rounded px-1.5 text-[12px] text-red-500 hover:bg-red-500/10"
                      >
                        Убрать
                      </button>
                    </div>
                  ))}
                  <div className="mt-1.5 flex items-center gap-2">
                    <select
                      value={addGroupId}
                      onChange={(e) => setAddGroupId(e.target.value)}
                      className="min-w-0 flex-1 rounded-md border border-line bg-card px-2 py-1 text-[12px] text-body outline-none"
                    >
                      <option value="">Добавить группу…</option>
                      {allGroups
                        .filter((g) => !projGroups.some((x) => x.groupId === g.id))
                        .map((g) => (
                          <option key={g.id} value={g.id}>{g.name}</option>
                        ))}
                    </select>
                    <button
                      type="button"
                      disabled={!addGroupId}
                      onClick={async () => {
                        await setProjectGroup(p.id, Number(addGroupId), "editor");
                        setProjGroups(await listProjectGroups(p.id).catch(() => []));
                        setAddGroupId("");
                      }}
                      className="rounded-md border border-line px-2.5 py-1 text-[12px] text-body transition hover:border-faint disabled:opacity-50"
                    >
                      Добавить
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        ))}

        <div className="flex items-center gap-2">
          <input
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder="ключ (латиница)"
            className="w-[160px] rounded-md border border-line bg-card px-2.5 py-1.5 font-mono text-[12px] text-ink outline-none focus:border-accent"
          />
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Название проекта"
            className="min-w-0 flex-1 rounded-md border border-line bg-card px-2.5 py-1.5 text-[13px] text-ink outline-none focus:border-accent"
          />
          <button
            type="button"
            onClick={create}
            disabled={!newKey || !newName}
            className="rounded-md bg-accent px-3 py-1.5 text-[13px] font-500 text-white transition hover:bg-accent/90 disabled:opacity-50"
          >
            Создать
          </button>
        </div>
      </div>
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
