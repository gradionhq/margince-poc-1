import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

// Parses all --gf-* declarations from a CSS selector block.
// Looks for a block whose selector text contains `selectorSnippet`.
function parseBlock(
  css: string,
  selectorSnippet: string,
): Record<string, string> {
  const escaped = selectorSnippet.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  // Match: ...selectorSnippet... { ... }  (non-greedy content, single-depth block)
  const re = new RegExp(`[^}]*${escaped}[^{]*\\{([^}]+)\\}`);
  const match = css.match(re);
  if (!match) return {};
  const block = match[1];
  const tokens: Record<string, string> = {};
  for (const line of block.split("\n")) {
    const m = line.match(/^\s*(--[a-zA-Z0-9-]+)\s*:\s*(.+?)\s*;/);
    if (m) tokens[m[1]] = m[2];
  }
  return tokens;
}

const CSS_PATH = path.resolve(__dirname, "ledger-green.css");
const css = fs.readFileSync(CSS_PATH, "utf8");

const lightTokens = parseBlock(css, ":root");
const darkTokens = parseBlock(css, 'data-theme="dark"');

describe("Ledger-Green token snapshot — light :root", () => {
  it("matches the full §2 light token set (inline snapshot)", () => {
    expect(lightTokens).toMatchInlineSnapshot(`
      {
        "--gf-accent": "#0B7A53",
        "--gf-accent-light": "rgba(11,122,83,.09)",
        "--gf-accent-med": "rgba(11,122,83,.17)",
        "--gf-away": "#fbbf24",
        "--gf-bg-card": "#EEF1F0",
        "--gf-bg-elevated": "#ffffff",
        "--gf-bg-hover": "#F3F6F4",
        "--gf-bg-page": "#FBFCFB",
        "--gf-bg-rail": "#13231D",
        "--gf-border-strong": "#D2D8D4",
        "--gf-border-subtle": "#E5E9E7",
        "--gf-confidence-high": "#22c55e",
        "--gf-confidence-low": "#ef4444",
        "--gf-confidence-med": "#fbbf24",
        "--gf-dnd": "#ef4444",
        "--gf-online": "#22c55e",
        "--gf-teal": "#0E7490",
        "--gf-text-content": "#36433D",
        "--gf-text-muted": "#CBD2CD",
        "--gf-text-primary": "#15201B",
        "--gf-text-secondary": "#68756E",
        "--gf-text-tertiary": "#9AA6A0",
      }
    `);
  });

  // Explicit assertions per §2 spec — fails loudly if a token is dropped or wrong
  const SPEC_LIGHT: Record<string, string> = {
    "--gf-accent": "#0B7A53",
    "--gf-bg-rail": "#13231D",
    "--gf-bg-page": "#FBFCFB",
    "--gf-bg-elevated": "#ffffff",
    "--gf-bg-card": "#EEF1F0",
    "--gf-bg-hover": "#F3F6F4",
    "--gf-accent-light": "rgba(11,122,83,.09)",
    "--gf-accent-med": "rgba(11,122,83,.17)",
    "--gf-text-primary": "#15201B",
    "--gf-text-content": "#36433D",
    "--gf-text-secondary": "#68756E",
    "--gf-text-tertiary": "#9AA6A0",
    "--gf-text-muted": "#CBD2CD",
    "--gf-border-subtle": "#E5E9E7",
    "--gf-border-strong": "#D2D8D4",
    "--gf-online": "#22c55e",
    "--gf-teal": "#0E7490",
    "--gf-away": "#fbbf24",
    "--gf-dnd": "#ef4444",
  };

  for (const [prop, val] of Object.entries(SPEC_LIGHT)) {
    it(`light ${prop} = ${val}`, () => {
      expect(lightTokens[prop], `${prop} must be ${val}`).toBe(val);
    });
  }
});

describe("Ledger-Green token snapshot — dark brand overrides", () => {
  it("matches the dark brand-override token set (inline snapshot)", () => {
    // Dark block contains ONLY the brand overrides — surfaces are delegated
    // to Forge's .dark cascade (intentional; see Task 2 design decision).
    expect(darkTokens).toMatchInlineSnapshot(`
      {
        "--gf-accent": "#16A34A",
        "--gf-accent-light": "color-mix(in srgb, var(--gf-accent) 12%, transparent)",
        "--gf-accent-med": "color-mix(in srgb, var(--gf-accent) 20%, transparent)",
        "--gf-bg-rail": "#13231D",
      }
    `);
  });

  it("dark --gf-accent is #16A34A", () => {
    expect(darkTokens["--gf-accent"]).toBe("#16A34A");
  });

  it("dark --gf-bg-rail is #13231D (same as light — deep ink-green both themes)", () => {
    expect(darkTokens["--gf-bg-rail"]).toBe("#13231D");
  });

  it("dark accent-light uses color-mix referencing var(--gf-accent)", () => {
    expect(darkTokens["--gf-accent-light"]).toContain("color-mix");
    expect(darkTokens["--gf-accent-light"]).toContain("var(--gf-accent)");
  });

  it("dark block does NOT contain surface/text tokens (delegated to Forge .dark)", () => {
    const surfaceTokens = [
      "--gf-bg-page",
      "--gf-bg-elevated",
      "--gf-bg-card",
      "--gf-bg-hover",
      "--gf-text-primary",
      "--gf-text-content",
      "--gf-text-secondary",
      "--gf-text-tertiary",
      "--gf-text-muted",
      "--gf-border-subtle",
      "--gf-border-strong",
    ];
    for (const tok of surfaceTokens) {
      expect(
        darkTokens,
        `dark block must not contain ${tok}`,
      ).not.toHaveProperty(tok);
    }
  });
});
