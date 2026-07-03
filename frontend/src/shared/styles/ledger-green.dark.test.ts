import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

const CSS_PATH = path.resolve(__dirname, "ledger-green.css");

function readCss(): string {
  return fs.readFileSync(CSS_PATH, "utf8");
}

// Extract declarations from a selector block, handling multi-selector rules.
// Looks for a block containing `selectorSnippet` and returns its declarations.
function extractBlock(
  css: string,
  selectorSnippet: string,
): Map<string, string> {
  const tokens = new Map<string, string>();
  // Escape for regex, find block that includes this snippet
  const escaped = selectorSnippet.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const re = new RegExp(`[^}]*${escaped}[^{]*\\{([^}]+)\\}`);
  const match = css.match(re);
  if (!match) return tokens;
  const block = match[1];
  for (const line of block.split("\n")) {
    const m = line.match(/--([a-zA-Z0-9-]+)\s*:\s*(.+?)\s*;/);
    if (m) {
      tokens.set(`--${m[1]}`, m[2].trim());
    }
  }
  return tokens;
}

describe("ledger-green.css — dark block", () => {
  it("declares brand overrides in [data-theme='dark'] / .dark block", () => {
    const css = readCss();
    const dark = extractBlock(css, 'data-theme="dark"');

    expect(dark.size, "dark block must not be empty").toBeGreaterThan(0);

    // Brand accent must flip to lighter green
    expect(dark.get("--gf-accent"), "--gf-accent dark").toBe("#16A34A");

    // Rail stays deep ink-green (same as light)
    expect(dark.get("--gf-bg-rail"), "--gf-bg-rail dark").toBe("#13231D");

    // accent-light and accent-med must use color-mix referencing var(--gf-accent), not raw rgba
    const accentLight = dark.get("--gf-accent-light") ?? "";
    expect(accentLight, "--gf-accent-light must use color-mix").toContain(
      "color-mix",
    );
    expect(
      accentLight,
      "--gf-accent-light must reference var(--gf-accent)",
    ).toContain("var(--gf-accent)");

    const accentMed = dark.get("--gf-accent-med") ?? "";
    expect(accentMed, "--gf-accent-med must use color-mix").toContain(
      "color-mix",
    );
    expect(
      accentMed,
      "--gf-accent-med must reference var(--gf-accent)",
    ).toContain("var(--gf-accent)");
  });

  it("does NOT re-declare surface/text tokens with light hex in dark block (delegates to Forge .dark)", () => {
    const css = readCss();
    const dark = extractBlock(css, 'data-theme="dark"');

    // These are light-mode values — they must NOT appear in the dark block
    const lightSurfaceValues = [
      "#FBFCFB", // --gf-bg-page light
      "#15201B", // --gf-text-primary light
      "#EEF1F0", // --gf-bg-card light
      "#36433D", // --gf-text-content light
      "#E5E9E7", // --gf-border-subtle light
    ];

    for (const [prop, val] of dark.entries()) {
      for (const lightVal of lightSurfaceValues) {
        expect(
          val,
          `dark block prop ${prop} must not use light surface hex ${lightVal}`,
        ).not.toBe(lightVal);
      }
    }
  });

  it("contains no Gradion orange hex anywhere in the file", () => {
    const css = readCss();
    const forbidden = [
      "#ff8c2b",
      "#FF8C2B",
      "#ff6b00",
      "#FF6B00",
      "f97316",
      "fb923c",
    ];
    for (const hex of forbidden) {
      expect(css, `must not contain Gradion orange: ${hex}`).not.toContain(hex);
    }
  });
});

describe("ledger-green.css — dark cascade order test", () => {
  it("dark block selector contains data-theme and .dark together", () => {
    const css = readCss();
    // The selector must cover BOTH [data-theme="dark"] AND .dark
    // so Forge's .dark neutrals are engaged and the brand overrides win
    expect(css).toMatch(/\[data-theme="dark"\]/);
    expect(css).toMatch(/\.dark/);

    // Both must appear in the SAME rule (before the first { after the combined selector)
    // Find the dark block selector line
    const blockMatch = css.match(
      /(\[data-theme="dark"\][^{]*\.dark|\.dark[^{]*\[data-theme="dark"\])[^{]*\{/,
    );
    expect(
      blockMatch,
      "dark block must use combined [data-theme='dark'], .dark selector",
    ).not.toBeNull();
  });

  it("dark block comes AFTER the :root block (override wins cascade)", () => {
    const css = readCss();
    const rootIdx = css.indexOf(":root");
    const darkIdx = css.indexOf('[data-theme="dark"]');
    expect(rootIdx, ":root must exist").toBeGreaterThan(-1);
    expect(darkIdx, "[data-theme='dark'] block must exist").toBeGreaterThan(-1);
    expect(darkIdx, "dark block must come after :root block").toBeGreaterThan(
      rootIdx,
    );
  });
});
