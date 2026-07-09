import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { buildAccountTree, flattenTree } from "../api/accountTree.js";
import { AccountTree } from "./AccountTree.js";

function makeOrg(id: string, parentId: string | null): Organization {
  return {
    id,
    workspace_id: "ws-1",
    display_name: `Org ${id}`,
    source: "test",
    captured_by: "human:test",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    parent_org_id: parentId,
    domains: [],
  };
}

const rootOrg = makeOrg("root", null);
const child1 = makeOrg("c1", "root");
const child2 = makeOrg("c2", "root");
const orgs = [rootOrg, child1, child2];

function makeRows(expandedIds: Set<string>) {
  const tree = buildAccountTree(orgs, "root");
  return flattenTree(tree!, expandedIds);
}

describe("AccountTree", () => {
  it("renders one row per flattened node", () => {
    const rows = makeRows(new Set(["root"]));
    render(
      <AccountTree
        rows={rows}
        expandedIds={new Set(["root"])}
        onToggleExpand={vi.fn()}
        restrictedNodes={[]}
      />,
    );
    expect(screen.getByText("Org root")).toBeInTheDocument();
    expect(screen.getByText("Org c1")).toBeInTheDocument();
    expect(screen.getByText("Org c2")).toBeInTheDocument();
  });

  it("a parent node with children shows a twist/chevron that calls onToggleExpand", async () => {
    const rows = makeRows(new Set(["root"]));
    const onToggle = vi.fn();
    render(
      <AccountTree
        rows={rows}
        expandedIds={new Set(["root"])}
        onToggleExpand={onToggle}
        restrictedNodes={[]}
      />,
    );
    const rootRow = screen.getByText("Org root").closest("tr");
    const toggle = rootRow?.querySelector("button");
    expect(toggle).not.toBeNull();
    await userEvent.click(toggle!);
    expect(onToggle).toHaveBeenCalledWith("root");
  });

  it("a leaf node shows no actionable twist (AC-4)", () => {
    const rows = makeRows(new Set(["root"]));
    render(
      <AccountTree
        rows={rows}
        expandedIds={new Set(["root"])}
        onToggleExpand={vi.fn()}
        restrictedNodes={[]}
      />,
    );
    // c1 is a leaf
    const c1Row = screen.getByText("Org c1").closest("tr");
    expect(c1Row?.querySelector("button")).toBeNull();
  });

  it("renders restricted nodes tagged 'restricted' with lock icon and masked cells (AC-5)", () => {
    const rows = makeRows(new Set(["root"]));
    render(
      <AccountTree
        rows={rows}
        expandedIds={new Set(["root"])}
        onToggleExpand={vi.fn()}
        restrictedNodes={[{ id: "r1", display_name: "Restricted Corp" }]}
      />,
    );
    expect(screen.getByText("Restricted Corp")).toBeInTheDocument();
    // FieldGuard masked token
    expect(screen.getAllByTestId("field-guard-masked").length).toBeGreaterThan(0);
    // Excluded note
    expect(
      screen.getByText(/excluded from roll-up/i),
    ).toBeInTheDocument();
  });

  it("renders the bound caption (Architecture design point 1)", () => {
    const rows = makeRows(new Set(["root"]));
    render(
      <AccountTree
        rows={rows}
        expandedIds={new Set(["root"])}
        onToggleExpand={vi.fn()}
        restrictedNodes={[]}
      />,
    );
    expect(screen.getByText(/up to 200 accounts/i)).toBeInTheDocument();
  });
});
