import { describe, expect, it } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import {
  buildAccountTree,
  countTreeNodes,
  findSuggestedEdgeCandidates,
  flattenTree,
  treeDepth,
} from "./accountTree.js";

function makeOrg(
  id: string,
  parentId: string | null,
  domain?: string,
): Organization {
  return {
    id,
    workspace_id: "ws-1",
    display_name: `Org ${id}`,
    source: "test",
    captured_by: "human:test",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    parent_org_id: parentId,
    domains: domain
      ? [
          {
            id: `d-${id}`,
            organization_id: id,
            domain,
            is_primary: true,
            source: "test",
            captured_by: "human:test",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
            archived_at: null,
          },
        ]
      : [],
  };
}

describe("buildAccountTree", () => {
  it("builds a 3-level tree correctly", () => {
    const root = makeOrg("root", null);
    const child1 = makeOrg("c1", "root");
    const child2 = makeOrg("c2", "root");
    const grandchild = makeOrg("gc1", "c1");
    const orgs = [root, child1, child2, grandchild];

    const tree = buildAccountTree(orgs, "root");
    expect(tree).not.toBeNull();
    expect(tree?.org.id).toBe("root");
    expect(tree?.children).toHaveLength(2);
    const c1Node = tree?.children.find((n) => n.org.id === "c1");
    expect(c1Node).toBeDefined();
    expect(c1Node?.children).toHaveLength(1);
    expect(c1Node?.children[0].org.id).toBe("gc1");
    const c2Node = tree?.children.find((n) => n.org.id === "c2");
    expect(c2Node?.children).toHaveLength(0);
  });

  it("returns null when rootId is not in orgs", () => {
    const orgs = [makeOrg("other", null)];
    expect(buildAccountTree(orgs, "missing")).toBeNull();
  });
});

describe("flattenTree", () => {
  const root = makeOrg("root", null);
  const child1 = makeOrg("c1", "root");
  const child2 = makeOrg("c2", "root");
  const grandchild = makeOrg("gc1", "c1");
  const orgs = [root, child1, child2, grandchild];

  it("a leaf node shows hasChildren: false", () => {
    const tree = buildAccountTree(
      [makeOrg("root", null), makeOrg("leaf", "root")],
      "root",
    );
    const rows = flattenTree(tree!, new Set(["root"]));
    const leafRow = rows.find((r) => r.node.org.id === "leaf");
    expect(leafRow).toBeDefined();
    expect(leafRow?.hasChildren).toBe(false);
  });

  it("collapses children when parent id not in expandedIds", () => {
    const tree = buildAccountTree(orgs, "root");
    // Only root is expanded, not c1 — so gc1 should not appear
    const rows = flattenTree(tree!, new Set(["root"]));
    const ids = rows.map((r) => r.node.org.id);
    expect(ids).toContain("root");
    expect(ids).toContain("c1");
    expect(ids).toContain("c2");
    expect(ids).not.toContain("gc1");
  });

  it("keeps the parent row itself even when collapsed", () => {
    const tree = buildAccountTree(orgs, "root");
    const rows = flattenTree(tree!, new Set());
    expect(rows).toHaveLength(1);
    expect(rows[0].node.org.id).toBe("root");
  });

  it("includes all nodes when everything is expanded", () => {
    const tree = buildAccountTree(orgs, "root");
    const rows = flattenTree(tree!, new Set(["root", "c1", "c2"]));
    const ids = rows.map((r) => r.node.org.id);
    expect(ids).toContain("root");
    expect(ids).toContain("c1");
    expect(ids).toContain("c2");
    expect(ids).toContain("gc1");
  });

  it("depth reflects nesting level", () => {
    const tree = buildAccountTree(orgs, "root");
    const rows = flattenTree(tree!, new Set(["root", "c1"]));
    const rootRow = rows.find((r) => r.node.org.id === "root");
    const c1Row = rows.find((r) => r.node.org.id === "c1");
    const gc1Row = rows.find((r) => r.node.org.id === "gc1");
    expect(rootRow?.depth).toBe(0);
    expect(c1Row?.depth).toBe(1);
    expect(gc1Row?.depth).toBe(2);
  });
});

describe("countTreeNodes", () => {
  it("counts every node in a 3-level tree, regardless of expand/collapse state", () => {
    const root = makeOrg("root", null);
    const child1 = makeOrg("c1", "root");
    const child2 = makeOrg("c2", "root");
    const grandchild = makeOrg("gc1", "c1");
    const orgs = [root, child1, child2, grandchild];
    const tree = buildAccountTree(orgs, "root")!;

    expect(countTreeNodes(tree)).toBe(4);

    // Collapsing everything (or anything) must not change the full-tree count — AC-8's budget
    // bar describes the whole tree, not currently-visible rows.
    const mostlyCollapsedRows = flattenTree(tree, new Set());
    expect(mostlyCollapsedRows).toHaveLength(1);
    expect(countTreeNodes(tree)).toBe(4);

    const partiallyExpandedRows = flattenTree(tree, new Set(["root"]));
    expect(partiallyExpandedRows).toHaveLength(3);
    expect(countTreeNodes(tree)).toBe(4);
  });

  it("counts a single-node tree as 1", () => {
    const tree = buildAccountTree([makeOrg("root", null)], "root")!;
    expect(countTreeNodes(tree)).toBe(1);
  });
});

describe("treeDepth", () => {
  it("reports the deepest full-tree level, regardless of expand/collapse state", () => {
    const root = makeOrg("root", null);
    const child1 = makeOrg("c1", "root");
    const child2 = makeOrg("c2", "root");
    const grandchild = makeOrg("gc1", "c1");
    const orgs = [root, child1, child2, grandchild];
    const tree = buildAccountTree(orgs, "root")!;

    expect(treeDepth(tree)).toBe(2);

    // Same invariant as countTreeNodes: collapsing hides rows, not tree structure.
    const collapsedRows = flattenTree(tree, new Set());
    expect(collapsedRows).toHaveLength(1);
    expect(treeDepth(tree)).toBe(2);
  });

  it("is 0 for a single-node tree", () => {
    const tree = buildAccountTree([makeOrg("root", null)], "root")!;
    expect(treeDepth(tree)).toBe(0);
  });
});

describe("findSuggestedEdgeCandidates", () => {
  it("returns orphans sharing the root's primary domain, excluding treeIds", () => {
    const root = makeOrg("root", null, "acme.com");
    const inTree = makeOrg("in-tree", "root", "acme.com");
    const orphan = makeOrg("orphan", null, "acme.com");
    const unrelated = makeOrg("unrelated", null, "other.com");
    const orgs = [root, inTree, orphan, unrelated];
    const treeIds = new Set(["root", "in-tree"]);

    const candidates = findSuggestedEdgeCandidates(orgs, root, treeIds);
    expect(candidates).toHaveLength(1);
    expect(candidates[0].id).toBe("orphan");
  });

  it("excludes orgs already in treeIds", () => {
    const root = makeOrg("root", null, "acme.com");
    const orphan = makeOrg("orphan", null, "acme.com");
    const treeIds = new Set(["root", "orphan"]);
    const candidates = findSuggestedEdgeCandidates(
      [root, orphan],
      root,
      treeIds,
    );
    expect(candidates).toHaveLength(0);
  });

  it("excludes non-orphan orgs (already a child)", () => {
    const root = makeOrg("root", null, "acme.com");
    const childWithSameDomain = makeOrg("child", "other", "acme.com");
    const candidates = findSuggestedEdgeCandidates(
      [root, childWithSameDomain],
      root,
      new Set(["root"]),
    );
    expect(candidates).toHaveLength(0);
  });

  it("returns no candidates if root has no domains", () => {
    const root = makeOrg("root", null);
    const orphan = makeOrg("orphan", null, "acme.com");
    const candidates = findSuggestedEdgeCandidates(
      [root, orphan],
      root,
      new Set(["root"]),
    );
    expect(candidates).toHaveLength(0);
  });

  it("an org with no domains never matches", () => {
    const root = makeOrg("root", null, "acme.com");
    const noDomainOrphan = makeOrg("no-domain", null);
    const candidates = findSuggestedEdgeCandidates(
      [root, noDomainOrphan],
      root,
      new Set(["root"]),
    );
    expect(candidates).toHaveLength(0);
  });
});
