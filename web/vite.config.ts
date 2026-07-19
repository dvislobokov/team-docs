import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// В dev фронт крутится на :5173, а запросы к /api проксируются на Go-бэкенд
// (:8080). В prod всё собирается в web/dist и встраивается в один бинарь.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/auth": "http://localhost:8080", // встроенный OAuth-логин
      "/mcp": "http://localhost:8080",
    },
  },
});
