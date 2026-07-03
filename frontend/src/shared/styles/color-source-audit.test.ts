import { execSync } from "node:child_process";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

// Color-source audit: runs scripts/check-ds-purity.sh via child_process and
// asserts exit 0. This binds the ds-purity gate into the Vitest run so that
// a future raw-hex regression fails `pnpm test`, not only `make ds-purity`.

const REPO_ROOT = path.resolve(__dirname, "../../../..");
const SCRIPT = path.join(REPO_ROOT, "scripts/check-ds-purity.sh");

describe("color-source audit", () => {
  it("ds-purity gate passes on frontend/src (no raw hex, no unmapped palettes)", () => {
    let output = "";
    let exitCode = 0;
    try {
      output = execSync(`bash "${SCRIPT}"`, {
        cwd: REPO_ROOT,
        encoding: "utf8",
        stdio: ["pipe", "pipe", "pipe"],
      });
    } catch (err: unknown) {
      const e = err as { status?: number; stdout?: string; stderr?: string };
      exitCode = e.status ?? 1;
      output = (e.stdout ?? "") + (e.stderr ?? "");
    }

    expect(exitCode, `ds-purity failed — fix the violations:\n${output}`).toBe(
      0,
    );
    expect(output).toContain("PASS");
  });
});
