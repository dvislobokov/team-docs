/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        paper: "rgb(var(--c-paper) / <alpha-value>)",
        card: "rgb(var(--c-card) / <alpha-value>)",
        ink: "rgb(var(--c-ink) / <alpha-value>)",
        body: "rgb(var(--c-body) / <alpha-value>)",
        muted: "rgb(var(--c-muted) / <alpha-value>)",
        faint: "rgb(var(--c-faint) / <alpha-value>)",
        line: "rgb(var(--c-line) / <alpha-value>)",
        accent: "rgb(var(--c-accent) / <alpha-value>)",
        accentSoft: "rgb(var(--c-accent-soft) / <alpha-value>)",
        marker: "rgb(var(--c-marker) / <alpha-value>)",
      },
      fontFamily: {
        display: ['"PT Serif"', "Georgia", "serif"],
        sans: ['"Inter Variable"', "Inter", "system-ui", "sans-serif"],
        mono: ['"JetBrains Mono Variable"', "monospace"],
      },
      // Макет использует классы font-500/font-600 — регистрируем их как
      // утилиты веса (иначе Tailwind их не генерирует и вес не применяется).
      fontWeight: {
        400: "400",
        500: "500",
        600: "600",
      },
    },
  },
  plugins: [],
};
