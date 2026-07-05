import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  Deal,
  Organization,
  OrganizationListResponse,
  Partner,
  Person,
} from "../../../lib/api-client/generated/index.js";

export type OrgPatch = {
  industry?: string | null;
  size_band?: Organization["size_band"];
  address?: Organization["address"];
  version?: number;
};

export function useOrganizations(opts?: { sort?: string; q?: string }) {
  return useQuery<OrganizationListResponse>({
    queryKey: ["organizations", opts?.sort, opts?.q],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/organizations", {
        params: { query: { sort: opts?.sort, q: opts?.q } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

// getOrganization is the one composite read backing the whole 360 screen: header, org-strength
// source line, deals rail, activity card, quick-facts all ride this single response.
export function useOrganization(id: string | undefined) {
  return useQuery<Organization>({
    queryKey: ["organizations", "detail", id],
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/organizations/{id}", {
        params: { path: { id: id as string } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

// A 404 here is a legitimate, expected data state (STATE-1: org just isn't a partner) — never
// treated as an error. Any other failure still surfaces as STATE-3.
export function useOrgPartner(id: string | undefined) {
  return useQuery<Partner | null>({
    queryKey: ["organizations", "partner", id],
    enabled: !!id,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET(
        "/organizations/{id}/partner",
        { params: { path: { id: id as string } } },
      );
      if (response.status === 404) return null;
      if (error) throw error;
      return data ?? null;
    },
  });
}

// Bounded N+1: no batch-by-org-id (or batch-by-ids) Person endpoint exists in the contract
// (confirmed against crm.d.ts's listPeople/getOrganization operations) — the people rail fetches
// one getPerson per contact id, in parallel via useQueries, bounded by contact_count.
export function useOrgContacts(personIds: string[]) {
  const results = useQueries({
    queries: personIds.map((id) => ({
      queryKey: ["people", "detail", id],
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/people/{id}", {
          params: { path: { id } },
        });
        if (error) throw error;
        if (!data) throw new Error("empty response");
        return data;
      },
    })),
  });
  return {
    contacts: results.map((r, i) => ({
      id: personIds[i],
      data: r.data as Person | undefined,
      isLoading: r.isLoading,
      isError: r.isError,
    })),
    isLoading: results.some((r) => r.isLoading),
  };
}

// KNOWN CONTRACT GAP (correctness, not just perf): listDeals has no partner_org_id filter
// (crm.d.ts listDeals query params — only organization_id/stage_id/owner_id/status/stalled/
// person_id), and organization_id can't substitute (sourced deals live on OTHER orgs' records).
// This fetches one bounded page (the 200 newest deals workspace-wide, default -created_at sort)
// and filters client-side. In a workspace with >200 deals this can under-report or show empty
// even when sourced deals exist — PartnerPanel renders an explicit caption naming this bound so
// it's visible in the UI, not just in this comment. Flagged in the PR description as a second gap
// alongside the missing batch-person endpoint.
export function useSourcedDeals(orgId: string | undefined) {
  return useQuery<Deal[]>({
    queryKey: ["deals", "sourced-by-partner", orgId],
    enabled: !!orgId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/deals", {
        params: { query: { limit: 200 } },
      });
      if (error) throw error;
      return (data?.data ?? []).filter((d) => d.partner_org_id === orgId);
    },
  });
}

// AC-company-12 Edit: PATCHes the fields the header/EditOrgModal exposes (industry/size_band/
// address). Sends If-Match with the org's last-seen version (optimistic concurrency per
// crm.d.ts's IfMatch parameter) so a stale edit is rejected with 409 rather than silently
// overwriting a concurrent change. The updateOrganization endpoint itself (and its
// audit_log/domain-event write) is pre-existing, already-merged backend surface (T09/T15/T16) —
// this ticket is a pure consumer and does not re-prove that Go-side invariant.
export function useUpdateOrganization(id: string) {
  const qc = useQueryClient();
  return useMutation<Organization, unknown, OrgPatch>({
    mutationFn: async ({ version, ...body }) => {
      const { data, error } = await apiClient.PATCH("/organizations/{id}", {
        params: {
          path: { id },
          header: version != null ? { "If-Match": String(version) } : undefined,
        },
        body,
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(["organizations", "detail", id], data);
    },
  });
}

export function useArchiveOrganization(id: string) {
  const qc = useQueryClient();
  return useMutation<Organization, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.DELETE("/organizations/{id}", {
        params: { path: { id } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(["organizations", "detail", id], data);
      void qc.invalidateQueries({ queryKey: ["organizations"] });
    },
  });
}

export function useRestoreOrganization(id: string) {
  const qc = useQueryClient();
  return useMutation<Organization, unknown, void>({
    mutationFn: async () => {
      const { data, error } = await apiClient.POST("/organizations/{id}/restore", {
        params: { path: { id } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(["organizations", "detail", id], data);
      void qc.invalidateQueries({ queryKey: ["organizations"] });
    },
  });
}
