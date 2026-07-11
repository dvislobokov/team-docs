import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router-dom";

// Локальные шрифты (совпадают с font-family в tailwind.config.js).
// PT Serif (дисплейный) — с кириллицей, в отличие от Fraunces.
import "@fontsource/pt-serif/400.css";
import "@fontsource/pt-serif/700.css";
import "@fontsource-variable/inter";
import "@fontsource-variable/jetbrains-mono";

// Стили редактора и глобальные стили/палитра.
import "@blocknote/mantine/style.css";
import "./index.css";

import { initTheme } from "./lib/theme";
import { router } from "./routes";

// Тему применяем до первого рендера, чтобы не было вспышки светлой темы.
initTheme();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
