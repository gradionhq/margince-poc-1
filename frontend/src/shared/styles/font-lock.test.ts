import { execSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

const SCRIPT = path.resolve(__dirname, "../../../scripts/check-font-lock.sh");

// Resolve the frontend/src root so the script can scan it
const WEB_SRC = path.resolve(__dirname, "../../../src");

function runScript(targetDir: string): { exitCode: number; output: string } {
  try {
    const output = execSync(`bash "${SCRIPT}" "${targetDir}"`, {
      encoding: "utf8",
      stdio: ["pipe", "pipe", "pipe"],
    });
    return { exitCode: 0, output };
  } catch (err: unknown) {
    const e = err as { status?: number; stdout?: string; stderr?: string };
    return {
      exitCode: e.status ?? 1,
      output: (e.stdout ?? "") + (e.stderr ?? ""),
    };
  }
}

let badFixtureDir: string;
let cleanFixtureDir: string;

beforeAll(() => {
  // Bad fixture: a CSS file with a disallowed font-family
  badFixtureDir = fs.mkdtempSync(path.join(os.tmpdir(), "font-lock-bad-"));
  fs.writeFileSync(
    path.join(badFixtureDir, "bad.css"),
    "p { font-family: Arial, sans-serif; }\n",
  );

  // Clean fixture: only allowed font families
  cleanFixtureDir = fs.mkdtempSync(path.join(os.tmpdir(), "font-lock-clean-"));
  fs.writeFileSync(
    path.join(cleanFixtureDir, "clean.css"),
    [
      ':root { --font-display: "Outfit", system-ui, sans-serif; }',
      '.heading { font-family: "DM Sans", sans-serif; }',
      '.code { font-family: "JetBrains Mono", monospace; }',
    ].join("\n"),
  );
});

afterAll(() => {
  fs.rmSync(badFixtureDir, { recursive: true, force: true });
  fs.rmSync(cleanFixtureDir, { recursive: true, force: true });
});

describe("check-font-lock.sh", () => {
  it("exits non-zero and names the offending family for a bad fixture", () => {
    const result = runScript(badFixtureDir);
    expect(result.exitCode, "bad fixture must exit non-zero").not.toBe(0);
    expect(result.output, "output must mention Arial").toContain("Arial");
  });

  it("exits 0 for a clean fixture (only Outfit / DM Sans / JetBrains Mono)", () => {
    const result = runScript(cleanFixtureDir);
    expect(
      result.exitCode,
      `clean fixture must exit 0\noutput: ${result.output}`,
    ).toBe(0);
  });

  it("exits 0 on the real frontend/src tree (no fourth font family exists)", () => {
    const result = runScript(WEB_SRC);
    expect(
      result.exitCode,
      `real tree must pass\noutput: ${result.output}`,
    ).toBe(0);
  });
});
