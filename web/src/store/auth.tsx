// Авторизация опциональна: приложение всегда спрашивает /api/me. В открытом
// режиме бэкенд отдаёт devUser. Если авторизация включена и токена нет/невалиден
// (обычно приложение за IAM-прокси, и такой запрос не доходит) — показываем
// экран «требуется авторизация».
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { ApiError } from "../api/client";
import { getMe } from "../api/auth";
import type { AuthProvider as OAuthProvider, Me } from "../api/types";

type Status = "loading" | "ready" | "unauthorized" | "error";

const AuthContext = createContext<Me | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<Me | null>(null);
  const [status, setStatus] = useState<Status>("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const ctrl = new AbortController();
    getMe(ctrl.signal)
      .then((me) => {
        setUser(me);
        setStatus("ready");
      })
      .catch((e) => {
        if (ctrl.signal.aborted) return;
        if (e instanceof ApiError && e.status === 401) {
          setStatus("unauthorized");
        } else {
          setMessage(e instanceof Error ? e.message : "Ошибка");
          setStatus("error");
        }
      });
    return () => ctrl.abort();
  }, []);

  const value = useMemo(() => user, [user]);

  if (status === "loading") {
    return <FullScreen>Загрузка…</FullScreen>;
  }
  if (status === "unauthorized") {
    return <LoginScreen />;
  }
  if (status === "error") {
    return (
      <FullScreen>
        <div className="text-[46px]">⚠️</div>
        <h1 className="mt-4 font-display text-[26px] font-500 text-ink">Что-то пошло не так</h1>
        <p className="mt-2 max-w-sm text-[14px] text-muted">{message}</p>
      </FullScreen>
    );
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// Экран входа: форма логин/пароль (LDAP/локальный админ) и кнопки
// OAuth-провайдеров (GET /auth/providers). Если ничего не настроено —
// приложение стоит за IAM-прокси, показываем подсказку.
function LoginScreen() {
  const [methods, setMethods] = useState<{ providers: OAuthProvider[]; password: boolean } | null>(null);
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    fetch("/auth/providers")
      .then((r) => (r.ok ? r.json() : { providers: [], password: false }))
      .then(setMethods)
      .catch(() => setMethods({ providers: [], password: false }));
  }, []);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await fetch("/auth/password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ login, password }),
      });
      if (res.ok) {
        window.location.href = "/";
        return;
      }
      setError(res.status === 401 ? "Неверный логин или пароль" : "Каталог недоступен — попробуйте позже");
    } catch {
      setError("Не удалось выполнить вход");
    } finally {
      setBusy(false);
    }
  };

  const empty = methods && methods.providers.length === 0 && !methods.password;

  return (
    <FullScreen>
      <div className="text-[46px]">🔒</div>
      <h1 className="mt-4 font-display text-[26px] font-500 text-ink">Вход в team-docs</h1>

      {methods === null && <p className="mt-2 text-[14px] text-muted">Загрузка…</p>}

      {methods?.password && (
        <form onSubmit={submit} className="mt-6 flex w-full max-w-[280px] flex-col gap-2">
          <input
            value={login}
            onChange={(e) => setLogin(e.target.value)}
            placeholder="Логин"
            autoComplete="username"
            className="rounded-lg border border-line bg-card px-4 py-2.5 text-[14px] text-ink outline-none focus:border-accent"
          />
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Пароль"
            autoComplete="current-password"
            className="rounded-lg border border-line bg-card px-4 py-2.5 text-[14px] text-ink outline-none focus:border-accent"
          />
          {error && <p className="text-[13px] text-red-500">{error}</p>}
          <button
            type="submit"
            disabled={busy || !login || !password}
            className="rounded-lg bg-accent px-4 py-2.5 text-[14px] font-500 text-white transition hover:bg-accent/90 disabled:opacity-50"
          >
            {busy ? "Вход…" : "Войти"}
          </button>
        </form>
      )}

      {methods && methods.providers.length > 0 && (
        <div className="mt-4 flex w-full max-w-[280px] flex-col gap-2">
          {methods.password && (
            <div className="my-1 flex items-center gap-3 text-[12px] text-faint">
              <span className="h-px flex-1 bg-line" /> или <span className="h-px flex-1 bg-line" />
            </div>
          )}
          {methods.providers.map((p) => (
            <a
              key={p.key}
              href={`/auth/login/${p.key}`}
              className="rounded-lg border border-line bg-card px-4 py-2.5 text-[14px] font-500 text-ink transition hover:border-faint hover:bg-line/40"
            >
              Войти через {p.label}
            </a>
          ))}
        </div>
      )}

      {empty && (
        <p className="mt-2 max-w-sm text-[14px] leading-relaxed text-muted">
          Доступ к team-docs выдаёт корпоративный вход. Откройте приложение
          через него — или обратитесь к администратору.
        </p>
      )}
    </FullScreen>
  );
}

function FullScreen({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-[calc(100vh/var(--ui-zoom,1))] flex-col items-center justify-center bg-paper px-6 text-center font-sans text-body">
      {children}
    </div>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): Me {
  const ctx = useContext(AuthContext);
  // Внутри AuthProvider (status=ready) контекст всегда есть.
  return ctx ?? { authenticated: false, canEdit: false, username: "", name: "", email: "", groups: null };
}
