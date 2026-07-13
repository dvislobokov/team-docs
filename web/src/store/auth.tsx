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
import type { Me } from "../api/types";

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
    return (
      <FullScreen>
        <div className="text-[46px]">🔒</div>
        <h1 className="mt-4 font-display text-[26px] font-500 text-ink">Требуется авторизация</h1>
        <p className="mt-2 max-w-sm text-[14px] leading-relaxed text-muted">
          Доступ к team-docs выдаёт корпоративный вход. Откройте приложение через
          него — или обратитесь к администратору.
        </p>
      </FullScreen>
    );
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

function FullScreen({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-screen flex-col items-center justify-center bg-paper px-6 text-center font-sans text-body">
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
