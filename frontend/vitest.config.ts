import path from "node:path";
import { fileURLToPath } from "node:url";
import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import react from "@vitejs/plugin-react";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig } from "vitest/config";

const dirname =
  typeof __dirname !== "undefined"
    ? __dirname
    : path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  test: {
    projects: [
      {
        // Unit project: jsdom + Testing Library.
        // extends:true inherits the root react() plugin — without it this
        // project would run with no JSX/Fast-Refresh plugin and rely solely
        // on esbuild + tsconfig `jsx`, leaving root `plugins:[react()]` dead.
        extends: true,
        test: {
          name: "unit",
          environment: "jsdom",
          globals: true,
          setupFiles: ["./src/test/setup.ts"],
          include: ["src/**/*.test.{ts,tsx}"],
          css: false,
          maxWorkers: 4,
        },
      },
      {
        // Storybook project: real Chromium via Playwright
        extends: true,
        // Pre-bundle @dnd-kit/core (the deals feature's drag-and-drop dep) so Vite
        // never lazily re-optimizes it mid-run: an on-the-fly "optimized dependencies
        // changed, reloading" invalidates the dev server mid-request, which kills
        // @storybook/addon-vitest's setup-file import ("Vitest failed to find the
        // runner") and fails every story suite that hadn't already loaded. Only
        // @dnd-kit/core is imported directly by source (PipelineBoard/StageColumn/
        // DealCard) -- its own transitive deps (utilities, accessibility) aren't
        // resolvable as bare top-level specifiers under this pnpm layout (they're
        // nested inside @dnd-kit/core's own tree), but esbuild's dependency scanner
        // already follows core's import graph once core itself is listed, so they
        // don't need (and can't take) an explicit entry here.
        optimizeDeps: {
          include: ["@dnd-kit/core"],
        },
        plugins: [
          storybookTest({ configDir: path.join(dirname, ".storybook") }),
        ],
        test: {
          name: "storybook",
          fileParallelism: false,
          isolate: false,
          sequence: { groupOrder: 1 },
          browser: {
            enabled: true,
            headless: true,
            provider: playwright({}),
            instances: [{ browser: "chromium" }],
          },
          setupFiles: [".storybook/vitest.setup.ts"],
        },
      },
    ],
  },
});
