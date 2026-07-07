// Capture each Storybook story to a PNG for visual review (the `ui-design-reviewer`
// agent reads these and compares them against the mockup rendered from
// design/margince-mockups.pen via the pencil MCP). This is an on-demand artifact
// producer — NOT a test gate and NOT wired into `make check`. It is deliberately
// not e2e: it renders each component story in isolation and grabs one frame.
//
// Usage: node frontend/scripts/capture-stories.mjs [idFilter] [--build]
//   idFilter = substring match on story id; --build forces a fresh Storybook build.
import { mkdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  buildStaticStorybook,
  loadPlaywright,
  readStoryIndex,
  serveStaticStorybook,
} from "./lib/storybook-harness.mjs";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const staticDir = join(repoRoot, "frontend/storybook-static");
const outDir = join(repoRoot, "frontend/.shots");
const idFilter = process.argv[2] && !process.argv[2].startsWith("--") ? process.argv[2] : "";

buildStaticStorybook(repoRoot, staticDir, { force: process.argv.includes("--build") });

const stories = readStoryIndex(staticDir).filter((e) => e.type === "story" && e.id.includes(idFilter));

mkdirSync(outDir, { recursive: true });
const { port, close } = await serveStaticStorybook(staticDir);
const pw = loadPlaywright(repoRoot);
const browser = await pw.chromium.launch();
const page = await browser.newPage({ viewport: { width: 1024, height: 720 }, deviceScaleFactor: 2 });

for (const story of stories) {
  await page.goto(`http://localhost:${port}/iframe.html?id=${story.id}&viewMode=story`, {
    waitUntil: "networkidle",
  });
  await page.waitForSelector("#storybook-root > *", { timeout: 10_000 }).catch(() => {});
  // Let any play() interaction (clicks, portals) settle before the frame.
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(outDir, `${story.id}.png`) });
  console.log(`✓ ${story.id}`);
}

await browser.close();
close();
console.log(`\n${stories.length} shot(s) → frontend/.shots/`);
