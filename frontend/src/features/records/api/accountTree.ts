import type { Organization } from "../../../lib/api-client/generated/index.js";
import { primaryDomainUrl } from "../../organizations/api/orgSelectors.js";

export type AccountTreeNode = {
  org: Organization;
  children: AccountTreeNode[];
};

// Builds the subtree rooted at rootId from a flat org list, by grouping on parent_org_id.
// Returns null if rootId isn't present in orgs (caller must still fetch/pass the root's own
// Organization from useOrganization(rootId) separately if it might fall outside the bounded
// page — see Task 5).
export function buildAccountTree(
  orgs: Organization[],
  rootId: string,
): AccountTreeNode | null {
  const byId = new Map<string, Organization>();
  for (const org of orgs) {
    byId.set(org.id, org);
  }

  const root = byId.get(rootId);
  if (!root) return null;

  const childrenByParent = new Map<string, Organization[]>();
  for (const org of orgs) {
    if (org.parent_org_id) {
      const list = childrenByParent.get(org.parent_org_id) ?? [];
      list.push(org);
      childrenByParent.set(org.parent_org_id, list);
    }
  }

  function buildNode(org: Organization): AccountTreeNode {
    const children = (childrenByParent.get(org.id) ?? []).map(buildNode);
    return { org, children };
  }

  return buildNode(root);
}

// Flattens a tree into depth-annotated rows for DataTable, honoring a caller-provided
// expanded-node-id set (collapsed parents' children are omitted, not just visually hidden —
// AC-4).
export function flattenTree(
  root: AccountTreeNode,
  expandedIds: ReadonlySet<string>,
): Array<{ node: AccountTreeNode; depth: number; hasChildren: boolean }> {
  const result: Array<{
    node: AccountTreeNode;
    depth: number;
    hasChildren: boolean;
  }> = [];

  function walk(node: AccountTreeNode, depth: number) {
    result.push({ node, depth, hasChildren: node.children.length > 0 });
    if (expandedIds.has(node.org.id)) {
      for (const child of node.children) {
        walk(child, depth + 1);
      }
    }
  }

  walk(root, 0);
  return result;
}

// Counts every node in the FULL tree, independent of any expand/collapse UI state (AC-8's
// budget bar — "depth N · M nodes · P% of P11 budget" — must describe the whole tree, not
// whatever rows are currently visible; flattenTree's row count shrinks as subtrees collapse,
// which is the wrong input for this).
export function countTreeNodes(root: AccountTreeNode): number {
  return (
    1 + root.children.reduce((sum, child) => sum + countTreeNodes(child), 0)
  );
}

// Deepest level in the FULL tree (root is depth 0), independent of expand/collapse UI state —
// same rationale as countTreeNodes above.
export function treeDepth(root: AccountTreeNode): number {
  return root.children.reduce(
    (max, child) => Math.max(max, 1 + treeDepth(child)),
    0,
  );
}

// Architecture design point 3: an orphan org sharing the root's normalized primary domain,
// not already in the tree.
export function findSuggestedEdgeCandidates(
  orgs: Organization[],
  root: Organization,
  treeIds: ReadonlySet<string>,
): Organization[] {
  const rootDomain = primaryDomainUrl(root.domains);
  if (!rootDomain) return [];

  return orgs.filter((org) => {
    if (treeIds.has(org.id)) return false;
    if (org.parent_org_id != null) return false;
    const domain = primaryDomainUrl(org.domains);
    return domain === rootDomain;
  });
}
