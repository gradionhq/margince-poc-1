import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

const CSS_PATH = path.resolve(__dirname, "ledger-green.css");

function readCss(): string {
  return fs.readFileSync(CSS_PATH, "utf8");
}

// Extract all declarations from a specific CSS selector block.
// Returns a map of property → value (trimmed, no semicolon).
function extractBlock(
  css: string,
  selectorPattern: RegExp,
): Map<string, string> {
  const tokens = new Map<string, string>();
  // Find the block for the selector
  const match = css.match(
    new RegExp(selectorPattern.source + "\\s*\\{([^}]+)\\}"),
  );
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

describe("ledger-green.css — light :root block", () => {
  it("declares all §2 Ledger-Green light tokens with exact spec values", () => {
    const css = readCss();

    const expected: Record<string, string> = {
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

    // Parse :root block
    const rootBlock = extractBlock(css, /:root/);
    expect(rootBlock.size, "root block must not be empty").toBeGreaterThan(0);

    for (const [prop, val] of Object.entries(expected)) {
      expect(rootBlock.get(prop), `${prop} must be ${val} in :root`).toBe(val);
    }
  });

  it("contains no Gradion orange or Dispact warm-stone hex anywhere in the file", () => {
    const css = readCss();
    const forbidden = [
      "#ff8c2b",
      "#FF6B00",
      "#ff6b00",
      "f97316",
      "fb923c",
      "orange-500",
    ];
    for (const hex of forbidden) {
      expect(
        css,
        `must not contain Gradion orange/stone: ${hex}`,
      ).not.toContain(hex);
    }
  });
});
