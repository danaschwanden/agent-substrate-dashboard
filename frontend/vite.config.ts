import { resolve } from "node:path";
import { defineConfig } from "vite";

export default defineConfig({
  publicDir: resolve(import.meta.dirname, "../api/fixtures"),
  build: {
    sourcemap: true,
    rollupOptions: {
      input: {
        dashboard: resolve(import.meta.dirname, "index.html"),
        gallery: resolve(import.meta.dirname, "gallery.html"),
      },
    },
  },
});
