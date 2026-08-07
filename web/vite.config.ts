import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Builds straight into internal/webui/dist so `//go:embed all:dist` there
// picks it up -- go:embed can't reference a directory outside its own
// package, so the frontend build output has to land inside the Go module
// tree rather than staying at web/dist.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:7777",
    },
  },
});
