import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        // Follow the backend's port so a per-worktree UAT env (make uat_env)
        // proxies to ITS OWN backend, not the default :8080 of another worktree.
        // Unset (plain `make fe-dev`) → :8080, unchanged.
        target: `http://localhost:${process.env.BACKEND_PORT ?? "8080"}`,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ""),
      },
    },
  },
});
