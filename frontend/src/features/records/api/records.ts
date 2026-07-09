import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  components,
  Organization,
} from "../../../lib/api-client/generated/index.js";

export type OrganizationHierarchyRollup =
  components["schemas"]["OrganizationHierarchyRollup"];

export const recordsKeys = {
  rollup: (rootId?: string, scope?: "tree" | "self") =>
    ["records", "hierarchy-rollup", rootId, scope] as const,
  treeOrgs: () => ["records", "tree-orgs"] as const,
};

export function useOrganizationHierarchyRollup(
  rootId: string | undefined,
  scope: "tree" | "self",
) {
  return useQuery<OrganizationHierarchyRollup>({
    queryKey: recordsKeys.rollup(rootId, scope),
    enabled: !!rootId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        "/organizations/{id}/hierarchy-rollup",
        { params: { path: { id: rootId as string }, query: { scope } } },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

// KNOWN CONTRACT GAP (same class useSourcedDeals already documents): listOrganizations has no
// parent_org_id filter (crm.d.ts confirmed at plan-authoring time — cursor/limit/sort/
// include_archived/owner_id/domain/classification/relevance_gte/q only), and there is no
// tree-structure read either. This fetches one bounded page (limit: 200 — RD-PARAM-1's own
// ≤200-org tree-size bound, not an arbitrary number the way useSourcedDeals's 200 is) and
// builds the tree client-side from parent_org_id. AccountTree renders an explicit caption
// naming this bound so it's visible on the surface, never silent.
export function useAccountTreeOrgs() {
  return useQuery<Organization[]>({
    queryKey: recordsKeys.treeOrgs(),
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/organizations", {
        params: { query: { limit: 200 } },
      });
      if (error) throw error;
      return data?.data ?? [];
    },
  });
}
