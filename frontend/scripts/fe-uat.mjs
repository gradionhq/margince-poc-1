// Change-scoped Storybook render + capture UAT lane (B8). For fe-only swarm
// diffs: renders the CHANGED component's stories in isolation and screenshots
// them — no live stack, no DB. Unlike capture-stories.mjs (capture-all, on-
// demand, swallows render errors), this is a GATE: it fails on a render error,
// on a changed component that has no story, and on a changed story that the
// build does not register — and writes a machine-readable manifest the
// swarm-uat-runner consumes.
//
// Reuses the Chromium @vitest/browser-playwright installed into the pnpm store
// (no new dependency). Usage:
//   node frontend/scripts/fe-uat.mjs [--allow-missing]
import { spawnSync } from "node:child_process";
import {
  createReadStream,
  existsSync,
  globSync,
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

// git without a shell — args split on spaces (a range like "<sha>..HEAD" is one arg).
function git(args) {
  const argv = args.split(" ");
  const r = spawnSync("git", argv, { cwd: repoRoot });
  if (r.status !== 0) throw new Error(`git ${args} failed: ${r.stderr}`);
  return r.stdout.toString().trim();
}

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

function writeManifest(fields) {
  mkdirSync(outDir, { recursive: true });
  writeFileSync(join(outDir, "manifest.json"), JSON.stringify({ base, head, ...fields }, null, 2) + "\n");
}

// Empty scope (no component/story touched) → nothing to render; pass.
if (storyFiles.size === 0 && missing.length === 0) {
  writeManifest({ stories: [], missing: [], unresolved: [], pass: true });
  console.log("fe-uat OK — diff touches no component/story (empty scope)");
  process.exit(0);
}

// Render only when there are stories to capture. If the diff is purely a
// component with no story (missing), skip straight to the verdict below.
let results = [];
let unresolved = [];
if (storyFiles.size > 0) {
  // Build the static Storybook FRESH every run: a cached build would render the
  // previous component/story source and green-light a broken change (a story
  // added or a component edited since the last build is otherwise invisible).
  console.log("Building static Storybook (~30-60s)…");
  const b = spawnSync(
    "pnpm",
    ["--filter", "@gradion/crm-web", "exec", "storybook", "build", "-o", "storybook-static"],
    { cwd: repoRoot, stdio: "inherit" },
  );
  if (b.status !== 0) process.exit(b.status ?? 1);

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
  // A changed/added story file that the fresh build did not register (bad glob,
  // no exported stories, malformed meta) must FAIL — never silently drop it.
  const resolvedPaths = new Set(inScope.map((e) => e.importPath));
  unresolved = [...wantImportPaths].filter((p) => !resolvedPaths.has(p));

  const require = createRequire(import.meta.url);
  const hits = globSync("node_modules/.pnpm/playwright@*/node_modules/playwright/index.js", {
    cwd: repoRoot,
  });
  if (hits.length === 0) {
    throw new Error("playwright not found in the pnpm store — run `make fe-install` first");
  }
  const pw = require(join(repoRoot, hits[0]));

  mkdirSync(outDir, { recursive: true });
  const browser = await pw.chromium.launch();
  const page = await browser.newPage({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 2 });

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
}

const pass =
  results.every((r) => r.pass) && unresolved.length === 0 && (allowMissing || missing.length === 0);
writeManifest({ stories: results, missing, unresolved, pass });

if (!pass) {
  const failed = results.filter((r) => !r.pass).map((r) => r.id);
  if (failed.length) console.error(`fe-uat FAIL — stories did not render clean: [${failed.join(", ")}]`);
  if (unresolved.length) {
    console.error(`fe-uat FAIL — changed story files the build did not register: [${unresolved.join(", ")}]`);
  }
  if (!allowMissing && missing.length) {
    console.error(`fe-uat FAIL — changed components with no story: [${missing.map((m) => m.component).join(", ")}]`);
    console.error("  (the coordinator dispatches react-dev to author a story, then re-runs — see B8)");
  }
  process.exit(1);
}
console.log(
  `fe-uat OK — ${results.length} story(ies) captured → ${relative(repoRoot, outDir)}/${missing.length ? ` (allow-missing: ${missing.length})` : ""}`,
);
