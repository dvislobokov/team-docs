// Брендинг и цветовые схемы приходят с бэка (GET /api/branding). Выбор схемы
// пользователем сохраняется в localStorage и применяется поверх дефолта сервера.
// Палитра инжектится в <style> (:root/.dark) поверх дефолтов из index.css.
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getBranding } from "../api/branding";
import type { Branding, PaletteColors, ThemePreset } from "../api/types";

const SCHEME_KEY = "td-color-scheme";

const DEFAULT_LIGHT: PaletteColors = {
  paper: "251 250 248",
  card: "255 255 255",
  ink: "38 37 31",
  body: "64 60 51",
  muted: "139 133 122",
  faint: "168 162 150",
  line: "235 232 224",
  accent: "53 104 89",
  accentSoft: "231 239 234",
  marker: "247 233 160",
};
const DEFAULT_DARK: PaletteColors = {
  paper: "26 25 23",
  card: "33 31 28",
  ink: "240 237 230",
  body: "199 193 180",
  muted: "146 139 125",
  faint: "110 103 90",
  line: "50 46 41",
  accent: "78 158 120",
  accentSoft: "33 46 40",
  marker: "122 106 45",
};
const DEFAULT_PRESET: ThemePreset = {
  id: "default",
  label: "Бумага",
  palette: { light: DEFAULT_LIGHT, dark: DEFAULT_DARK },
};
const DEFAULT_BRANDING: Branding = {
  appName: "team-docs",
  workspaceName: "рабочее пространство",
  monogram: "td",
  defaultTheme: "default",
  themes: [DEFAULT_PRESET],
};

const VAR: Record<keyof PaletteColors, string> = {
  paper: "--c-paper",
  card: "--c-card",
  ink: "--c-ink",
  body: "--c-body",
  muted: "--c-muted",
  faint: "--c-faint",
  line: "--c-line",
  accent: "--c-accent",
  accentSoft: "--c-accent-soft",
  marker: "--c-marker",
};

function toRules(colors: PaletteColors): string {
  return (Object.keys(VAR) as (keyof PaletteColors)[])
    .map((k) => `${VAR[k]}: ${colors[k]};`)
    .join("");
}

function applyPalette(palette: ThemePreset["palette"]) {
  const css = `:root{${toRules(palette.light)}}\n.dark{${toRules(palette.dark)}}`;
  let el = document.getElementById("td-palette") as HTMLStyleElement | null;
  if (!el) {
    el = document.createElement("style");
    el.id = "td-palette";
    document.head.appendChild(el);
  }
  el.textContent = css;
}

interface BrandingContextValue {
  appName: string;
  workspaceName: string;
  monogram: string;
  themes: ThemePreset[];
  schemeId: string;
  setScheme: (id: string) => void;
}

const BrandingContext = createContext<BrandingContextValue | null>(null);

export function BrandingProvider({ children }: { children: ReactNode }) {
  const [branding, setBranding] = useState<Branding>(DEFAULT_BRANDING);
  const [schemeId, setSchemeId] = useState<string>("default");

  useEffect(() => {
    const ctrl = new AbortController();
    getBranding(ctrl.signal)
      .then((b) => {
        setBranding(b);
        document.title = b.appName;
        // Выбор пользователя из localStorage, иначе дефолт сервера.
        const saved = localStorage.getItem(SCHEME_KEY);
        const chosen = b.themes.find((t) => t.id === saved) ?? b.themes.find((t) => t.id === b.defaultTheme) ?? b.themes[0];
        if (chosen) {
          setSchemeId(chosen.id);
          applyPalette(chosen.palette);
        }
      })
      .catch(() => undefined); // офлайн/ошибка — остаёмся на дефолтах index.css
    return () => ctrl.abort();
  }, []);

  const setScheme = (id: string) => {
    const preset = branding.themes.find((t) => t.id === id);
    if (!preset) return;
    localStorage.setItem(SCHEME_KEY, id);
    setSchemeId(id);
    applyPalette(preset.palette);
  };

  const value = useMemo<BrandingContextValue>(
    () => ({
      appName: branding.appName,
      workspaceName: branding.workspaceName,
      monogram: branding.monogram,
      themes: branding.themes,
      schemeId,
      setScheme,
    }),
    // setScheme стабилен по branding.themes
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [branding, schemeId],
  );

  return <BrandingContext.Provider value={value}>{children}</BrandingContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useBranding(): BrandingContextValue {
  const ctx = useContext(BrandingContext);
  if (!ctx) throw new Error("useBranding must be used within <BrandingProvider>");
  return ctx;
}
