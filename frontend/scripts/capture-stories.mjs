// Capture each Storybook story to a PNG for visual review (the `ui-design-reviewer`
// agent reads these and compares them against the mockup rendered from
// design/margince-mockups.pen via the pencil MCP). This is an on-demand artifact
// producer — NOT a test gate and NOT wired into `make check`. It is deliberately
// not e2e: it renders each component story in isolation and grabs one frame.
//
// Reuses the Chromium that @vitest/browser-playwright already installed (resolved
// from the pnpm store) so it adds no dependency. Usage:
//   node frontend/scripts/capture-stories.mjs [idFilter]   # idFilter = substring match on story id
import { spawnSync } from "node:child_process";
import { createReadStream, existsSync, globSync, mkdirSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { dirname, extname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const staticDir = join(repoRoot, "frontend/storybook-static");
const outDir = join(repoRoot, "frontend/.shots");
const idFilter = process.argv[2] ?? "";

const require = createRequire(import.meta.url);
function loadPlaywright() {
  const hits = globSync("node_modules/.pnpm/playwright@*/node_modules/playwright/index.js", {
    cwd: repoRoot,
  });
  if (hits.length === 0) {
    throw new Error("playwright not found in the pnpm store — run `make fe-install` first");
  }
  return require(join(repoRoot, hits[0]));
}

// Build the static Storybook if it isn't there (or `--build` forces a rebuild).
if (!existsSync(join(staticDir, "index.json")) || process.argv.includes("--build")) {
  console.log("Building static Storybook (one-time, ~30-60s)…");
  const r = spawnSync(
    "pnpm",
    ["--filter", "@gradion/crm-web", "exec", "storybook", "build", "-o", "storybook-static"],
    { cwd: repoRoot, stdio: "inherit" },
  );
  if (r.status !== 0) process.exit(r.status ?? 1);
}

const MIME = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".json": "application/json",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".woff2": "font/woff2",
};
const server = createServer((req, res) => {
  const path = decodeURIComponent((req.url ?? "/").split("?")[0]);
  const file = join(staticDir, path === "/" ? "index.html" : path);
  if (!file.startsWith(staticDir) || !existsSync(file)) {
    res.writeHead(404).end();
    return;
  }
  res.writeHead(200, { "content-type": MIME[extname(file)] ?? "application/octet-stream" });
  createReadStream(file).pipe(res);
});

await new Promise((r) => server.listen(0, r));
const port = server.address().port;

const index = JSON.parse(readFileSync(join(staticDir, "index.json"), "utf8"));
const stories = Object.values(index.entries).filter(
  (e) => e.type === "story" && e.id.includes(idFilter),
);

mkdirSync(outDir, { recursive: true });
const pw = loadPlaywright();
const browser = await pw.chromium.launch();
const page = await browser.newPage({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 2 });

for (const story of stories) {
  const url = `http://localhost:${port}/iframe.html?id=${story.id}&viewMode=story`;
  await page.goto(url, { waitUntil: "networkidle" });
  await page.waitForSelector("#storybook-root > *", { timeout: 10_000 }).catch(() => {});
  // Let any play() interaction (clicks, portals) settle before the frame.
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(outDir, `${story.id}.png`) });
  console.log(`✓ ${story.id}`);
}

await browser.close();
server.close();
console.log(`\n${stories.length} shot(s) → frontend/.shots/`);
