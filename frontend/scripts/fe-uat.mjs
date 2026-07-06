// Change-scoped Storybook render + capture UAT lane (B8). For fe-only swarm
// diffs: renders the CHANGED component's stories in isolation and screenshots
// them — no live stack, no DB. Unlike capture-stories.mjs (capture-all, on-
// demand, swallows render errors), this is a GATE: it fails on a render error
// and on a changed component that has no story, and writes a machine-readable
// manifest the swarm-uat-runner consumes.
//
// Reuses the Chromium @vitest/browser-playwright installed into the pnpm store
// (no new dependency). Usage:
//   node frontend/scripts/fe-uat.mjs [--build] [--allow-missing]
import { execSync, spawnSync } from "node:child_process";
import {
  createReadStream,
  existsSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { createServer } from "node:http";
import { createRequire } from "node:module";
import { dirname, extname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const staticDir = join(repoRoot, "frontend/storybook-static");
const outDir = join(repoRoot, ".tmp/fe-uat");
const allowMissing = process.argv.includes("--allow-missing");

const git = (args) => execSync(`git ${args}`, { cwd: repoRoot }).toString().trim();

// 1. Changed files on this branch vs origin/main.
let base;
try {
  base = git("merge-base origin/main HEAD");
} catch {
  console.error("fe-uat: cannot compute merge-base with origin/main (shallow/detached?) — fall back to full-stack UAT");
  process.exit(2);
}
const head = git("rev-parse HEAD");
const changed = git(`diff --name-only ${base}..HEAD`).split("\n").filter(Boolean);

// 2. In-scope story files: changed *.stories.tsx + the co-located story of any
//    changed component. A changed component with no co-located story is a gap.
const storyFiles = new Set();
const missing = [];
for (const f of changed) {
  if (!f.startsWith("frontend/src/")) continue;
  if (/\.stories\.(t|j)sx?$/.test(f)) {
    storyFiles.add(f);
  } else if (/\.(t|j)sx?$/.test(f) && !/\.(test|stories)\./.test(f)) {
    const story = f.replace(/\.(t|j)sx?$/, ".stories.tsx");
    if (existsSync(join(repoRoot, story))) storyFiles.add(story);
    else missing.push({ component: f });
  }
}

// Map story files (frontend/src/…) to Storybook importPaths (./src/…).
const wantImportPaths = new Set([...storyFiles].map((p) => "./" + p.replace(/^frontend\//, "")));

function writeManifest(stories, pass) {
  mkdirSync(outDir, { recursive: true });
  writeFileSync(
    join(outDir, "manifest.json"),
    JSON.stringify({ base, head, stories, missing, pass }, null, 2) + "\n",
  );
}

// Empty scope (no component/story touched) → nothing to render; pass.
if (storyFiles.size === 0 && missing.length === 0) {
  writeManifest([], true);
  console.log("fe-uat OK — diff touches no component/story (empty scope)");
  process.exit(0);
}

// 3. Build the static Storybook if absent (or --build forces a rebuild).
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
const inScope = Object.values(index.entries).filter(
  (e) => e.type === "story" && wantImportPaths.has(e.importPath),
);

const require = createRequire(import.meta.url);
function loadPlaywright() {
  const glob = require("node:fs").globSync;
  const hits = glob("node_modules/.pnpm/playwright@*/node_modules/playwright/index.js", {
    cwd: repoRoot,
  });
  if (hits.length === 0) {
    throw new Error("playwright not found in the pnpm store — run `make fe-install` first");
  }
  return require(join(repoRoot, hits[0]));
}

mkdirSync(outDir, { recursive: true });
const pw = loadPlaywright();
const browser = await pw.chromium.launch();
const page = await browser.newPage({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 2 });

const results = [];
for (const story of inScope) {
  const errors = [];
  page.removeAllListeners("pageerror");
  page.removeAllListeners("console");
  page.on("pageerror", (e) => errors.push(String(e)));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  const url = `http://localhost:${port}/iframe.html?id=${story.id}&viewMode=story`;
  await page.goto(url, { waitUntil: "networkidle" });
  let rendered = true;
  try {
    await page.waitForSelector("#storybook-root > *", { timeout: 10_000 });
  } catch {
    rendered = false;
    errors.push("#storybook-root stayed empty (component did not render)");
  }
  // Let any play() interaction settle before the frame.
  await page.waitForTimeout(250);
  const png = join(outDir, `${story.id}.png`);
  await page.screenshot({ path: png });
  const pass = rendered && errors.length === 0;
  results.push({ id: story.id, pass, png: relative(repoRoot, png), errors });
  console.log(`${pass ? "✓" : "✗"} ${story.id}${pass ? "" : ` — ${errors.join("; ")}`}`);
}

await browser.close();
server.close();

const pass = results.every((r) => r.pass) && (allowMissing || missing.length === 0);
writeManifest(results, pass);

if (!pass) {
  const failed = results.filter((r) => !r.pass).map((r) => r.id);
  if (failed.length) console.error(`fe-uat FAIL — stories did not render clean: [${failed.join(", ")}]`);
  if (!allowMissing && missing.length) {
    console.error(`fe-uat FAIL — changed components with no story: [${missing.map((m) => m.component).join(", ")}]`);
    console.error("  (the coordinator dispatches react-dev to author a story, then re-runs — see B8)");
  }
  process.exit(1);
}
console.log(
  `fe-uat OK — ${results.length} story(ies) captured → ${relative(repoRoot, outDir)}/${missing.length ? ` (allow-missing: ${missing.length})` : ""}`,
);
