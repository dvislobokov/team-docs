// Стор избранного: один источник правды для секции в сайдбаре и звезды на
// странице. Аноним (publicRead) получает 401 — тогда избранное просто пусто.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { addFavorite, getFavorites, removeFavorite } from "../api/pages";
import type { FavoriteItem } from "../api/types";

interface FavoritesContextValue {
  favorites: FavoriteItem[];
  isFavorite: (pageId: number) => boolean;
  toggle: (pageId: number) => Promise<void>;
  reload: () => Promise<void>;
}

const FavoritesContext = createContext<FavoritesContextValue | null>(null);

export function FavoritesProvider({ children }: { children: ReactNode }) {
  const [favorites, setFavorites] = useState<FavoriteItem[]>([]);

  const reload = useCallback(async () => {
    try {
      setFavorites(await getFavorites());
    } catch {
      setFavorites([]); // 401 (аноним) или сеть — секцию не показываем
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const isFavorite = useCallback(
    (pageId: number) => favorites.some((f) => f.id === pageId),
    [favorites],
  );

  const toggle = useCallback(
    async (pageId: number) => {
      if (favorites.some((f) => f.id === pageId)) {
        await removeFavorite(pageId);
      } else {
        await addFavorite(pageId);
      }
      await reload();
    },
    [favorites, reload],
  );

  const value = useMemo<FavoritesContextValue>(
    () => ({ favorites, isFavorite, toggle, reload }),
    [favorites, isFavorite, toggle, reload],
  );

  return <FavoritesContext.Provider value={value}>{children}</FavoritesContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useFavorites(): FavoritesContextValue {
  const ctx = useContext(FavoritesContext);
  if (!ctx) throw new Error("useFavorites must be used within <FavoritesProvider>");
  return ctx;
}
