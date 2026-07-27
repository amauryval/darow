import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";

// Base is /ui/ so the built assets resolve correctly once served by the Go
// router under that prefix — kept separate from the JSON API paths
// (/tables, /sources, ...) to avoid any route collision.
export default defineConfig({
  base: "/ui/",
  plugins: [vue(), tailwindcss()],
  // Built straight into internal/webui/dist: go:embed paths are relative
  // to the embedding package's own directory, so the Go side embeds from
  // there directly rather than needing a copy/symlink step.
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/tables": "http://localhost:8080",
      "/sources": "http://localhost:8080",
    },
  },
});
