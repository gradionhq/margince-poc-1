import * as fs from "node:fs";
import * as path from "node:path";
import { afterEach, describe, expect, it } from "vitest";

// Cascade resolution test — structural verification.
//
// jsdom cannot resolve @import chains or var() across separately-injected
// style sheets reliably, so we verify the structural properties that
// guarantee correct cascade behavior in a real browser: correct selector
// presence, correct import order (override file after Forge), and that the
// override does not re-declare surface tokens (keeping Forge's dark neutrals
// as the authoritative dark surface values).

const VARIABLES_CSS_PATH = path.resolve(
  __dirname,
  "../../../node_modules/@shared/token/src/tokens/web/variables.css",
);
const OVERRIDE_CSS_PATH = path.resolve(__dirname, "ledger-green.css");
const INDEX_CSS_PATH = path.resolve(__dirname, "../../app/index.css");

function read(p: string): string {
  return fs.readFileSync(p, "utf8");
}

afterEach(() => {
  delete document.documentElement.dataset["theme"];
  document.documentElement.classList.remove("dark");
});

describe("cascade resolution — structural verification", () => {
  it("Forge variables.css :root contains an orange accent (the default to override)", () => {
    const forge = read(VARIABLES_CSS_PATH);
    // Forge uses var(--gf-orange-500) for --gf-accent in :root
    expect(forge).toContain("--gf-accent: var(--gf-orange-500)");
  });

  it("Forge .dark block flips --gf-bg-page to a dark neutral (not light #FBFCFB)", () => {
    const forge = read(VARIABLES_CSS_PATH);
    // Forge's .dark block sets --gf-bg-page to var(--gf-neutral-900)
    expect(forge).toContain("--gf-bg-page: var(--gf-neutral-900)");
    // And the light value #FBFCFB must NOT appear anywhere in Forge
    expect(forge).not.toContain("#FBFCFB");
  });

  it("override :root sets green --gf-accent (supersedes Forge orange in cascade)", () => {
    const override = read(OVERRIDE_CSS_PATH);
    expect(override).toContain("--gf-accent:       #0B7A53");
  });

  it("override dark block accent is #16A34A — supersedes Forge dark orange (#ff8c2b)", () => {
    const override = read(OVERRIDE_CSS_PATH);
    expect(override).toContain("--gf-accent:       #16A34A");
    // Forge's .dark accent is #ff8c2b; our override must NOT include that
    expect(override).not.toContain("#ff8c2b");
    expect(override).not.toContain("#FF8C2B");
  });

  it("override dark block does NOT declare --gf-bg-page (surface delegated to Forge .dark)", () => {
    const override = read(OVERRIDE_CSS_PATH);
    // Extract the dark block text: everything between [data-theme="dark"] and next top-level }
    const darkBlockMatch = override.match(
      /\[data-theme="dark"\][^{]*\{([^}]+)\}/,
    );
    expect(darkBlockMatch, "dark block must exist in override").not.toBeNull();
    const darkBlock = darkBlockMatch![1];
    expect(
      darkBlock,
      "--gf-bg-page must not be in override dark block",
    ).not.toContain("--gf-bg-page");
    expect(
      darkBlock,
      "--gf-text-primary must not be in override dark block",
    ).not.toContain("--gf-text-primary");
    expect(
      darkBlock,
      "--gf-border-subtle must not be in override dark block",
    ).not.toContain("--gf-border-subtle");
  });

  it("index.css imports override AFTER Forge imports (cascade order)", () => {
    const indexCss = read(INDEX_CSS_PATH);
    const forgeIdx = indexCss.indexOf("@shared/token");
    const overrideIdx = indexCss.indexOf("ledger-green.css");
    expect(forgeIdx, "Forge import must be present").toBeGreaterThan(-1);
    expect(
      overrideIdx,
      "ledger-green.css import must be present",
    ).toBeGreaterThan(-1);
    expect(
      overrideIdx,
      "override must come after Forge imports",
    ).toBeGreaterThan(forgeIdx);
  });

  it("jsdom: data-theme=dark + .dark class both apply (DOM toggle smoke test)", () => {
    document.documentElement.dataset["theme"] = "dark";
    document.documentElement.classList.add("dark");
    expect(document.documentElement.dataset["theme"]).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);

    delete document.documentElement.dataset["theme"];
    document.documentElement.classList.remove("dark");
    expect(document.documentElement.dataset["theme"]).toBeUndefined();
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
