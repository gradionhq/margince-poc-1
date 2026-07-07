// Shared harness for the Storybook capture scripts (capture-stories.mjs and
// fe-uat.mjs): build the static Storybook, serve it locally, resolve the
// pnpm-store Chromium, and read the story index. Extracted so the two callers
// share ONE implementation — no duplication — and one path-traversal-safe file
// server. Reuses the Chromium @vitest/browser-playwright already installed (no
// new dependency).
import { spawnSync } from "node:child_process";
import { createReadStream, existsSync, globSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { extname, join, resolve, sep } from "node:path";

const MIME = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".json": "application/json",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".map": "application/json",
};

// buildStaticStorybook builds storybook-static if it is absent, or unconditionally
// when force is set (fe-uat forces a fresh build so it renders the current diff).
export function buildStaticStorybook(repoRoot, staticDir, { force = false } = {}) {
  if (existsSync(join(staticDir, "index.json")) && !force) return;
  console.log("Building static Storybook (~30-60s)…");
  const r = spawnSync(
    "pnpm",
    ["--filter", "@gradion/crm-web", "exec", "storybook", "build", "-o", "storybook-static"],
    { cwd: repoRoot, stdio: "inherit" },
  );
  if (r.status !== 0) process.exit(r.status ?? 1);
}

// serveStaticStorybook serves staticDir on an ephemeral port. The request path is
// resolved and confined to staticDir — a `..` that escapes the root is rejected
// (path-traversal safe) rather than read from disk. Returns {port, close}.
export function serveStaticStorybook(staticDir) {
  const root = resolve(staticDir);
  const server = createServer((req, res) => {
    const urlPath = decodeURIComponent((req.url ?? "/").split("?")[0]);
    const rel = urlPath === "/" ? "index.html" : urlPath.replace(/^\/+/, "");
    const file = resolve(root, rel);
    if (file !== root && !file.startsWith(root + sep)) {
      res.writeHead(403).end();
      return;
    }
    if (!existsSync(file)) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, { "content-type": MIME[extname(file)] ?? "application/octet-stream" });
    createReadStream(file).pipe(res);
  });
  return new Promise((r) =>
    server.listen(0, () => r({ port: server.address().port, close: () => server.close() })),
  );
}

// loadPlaywright resolves the Playwright already in the pnpm store.
export function loadPlaywright(repoRoot) {
  const require = createRequire(import.meta.url);
  const hits = globSync("node_modules/.pnpm/playwright@*/node_modules/playwright/index.js", {
    cwd: repoRoot,
  });
  if (hits.length === 0) {
    throw new Error("playwright not found in the pnpm store — run `make fe-install` first");
  }
  return require(join(repoRoot, hits[0]));
}

// readStoryIndex returns the story entries from the built storybook-static index.
export function readStoryIndex(staticDir) {
  return Object.values(JSON.parse(readFileSync(join(staticDir, "index.json"), "utf8")).entries);
}
