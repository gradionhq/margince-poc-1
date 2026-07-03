import { describe, expect, it } from "vitest";
import { RAIL_NAV } from "../../app/shell/railNav.js";
import { activeNavIdForPath } from "./useActiveNavId.js";

describe("activeNavIdForPath", () => {
  it("maps each nav target to exactly its own id", () => {
    for (const item of RAIL_NAV) {
      expect(activeNavIdForPath(item.to)).toBe(item.id);
    }
  });

  it("matches nested child routes to their parent nav item", () => {
    expect(activeNavIdForPath("/deals/42")).toBe("deals");
    expect(activeNavIdForPath("/people/abc/edit")).toBe("contacts");
  });

  it("returns undefined for rail-less / unknown routes (at most one active)", () => {
    expect(activeNavIdForPath("/settings")).toBeUndefined();
    expect(activeNavIdForPath("/login")).toBeUndefined();
  });

  it("never resolves to more than one active id across any path", () => {
    const paths = [
      "/home",
      "/people",
      "/deals/1",
      "/tasks",
      "/settings",
      "/unknown",
    ];
    for (const p of paths) {
      const id = activeNavIdForPath(p);
      const matches = RAIL_NAV.filter((i) => i.id === id);
      expect(matches.length).toBeLessThanOrEqual(1);
    }
  });
});
