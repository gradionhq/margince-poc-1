import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  Deal,
  Person,
  components,
} from "../../../lib/api-client/generated/index.js";

// PersonStrengthBreakdown has no named alias in generated/index.ts (unlike Person/Deal/
// Organization) — reach it via components["schemas"] like Task 5's ActivityRef does, not by
// inventing a top-level alias export that doesn't exist.
type PersonStrengthBreakdown = components["schemas"]["PersonStrengthBreakdown"];

export function usePerson(id: string) {
  return useQuery<Person>({
    queryKey: ["person", id],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/people/{id}", {
        params: { path: { id } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function usePersonStrengthBreakdown(id: string, enabled: boolean) {
  return useQuery<PersonStrengthBreakdown>({
    queryKey: ["person-strength-breakdown", id],
    enabled,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        "/people/{id}/strength-breakdown",
        { params: { path: { id } } },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

// DEAL-EXT-2 reverse lookup — a dedicated read for the Deals tab rather than reusing
// `Person.deals` from the composite read, since that field's shape/pagination isn't guaranteed
// to match this tab's read-only-list needs (see plan Global Constraints).
export function usePersonDeals(personId: string) {
  return useQuery<Deal[]>({
    queryKey: ["person-deals", personId],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/deals", {
        params: { query: { person_id: personId } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data.data;
    },
  });
}

// The header's "{title} · {company}" needs the org's *display name*; Relationship only carries
// organization_id (no denormalized name, same gap as T21's Deal->org name). Lazy, keyed to the
// current-primary employment relationship's organization_id — undefined disables the query.
export function useOrganizationName(organizationId: string | undefined) {
  return useQuery<string>({
    queryKey: ["organization-name", organizationId],
    enabled: !!organizationId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/organizations/{id}", {
        params: { path: { id: organizationId as string } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data.display_name;
    },
  });
}

// mergePerson has no If-Match/version param on the wire (crm.d.ts ~L80-133) — nothing to send.
// Its 409 uses the generic Conflict schema, not a dedicated VersionConflict; callers must read
// the actual `code` off the thrown error rather than assume `version_skew` (PO-AC-M5 gap, flagged
// in the PR description).
export function useMergePerson(id: string) {
  const queryClient = useQueryClient();
  return useMutation<Person, unknown, { targetId: string }>({
    mutationFn: async ({ targetId }) => {
      const { data, error } = await apiClient.POST("/people/{id}/merge", {
        params: { path: { id } },
        body: { target_id: targetId },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (survivor) => {
      void queryClient.invalidateQueries({ queryKey: ["person", id] });
      void queryClient.invalidateQueries({ queryKey: ["person", survivor.id] });
    },
  });
}

export function useUpdatePerson(id: string) {
  const queryClient = useQueryClient();
  return useMutation<
    Person,
    unknown,
    { body: { full_name?: string; title?: string | null }; ifMatch?: string }
  >({
    mutationFn: async ({ body, ifMatch }) => {
      const { data, error } = await apiClient.PATCH("/people/{id}", {
        params: {
          path: { id },
          header: ifMatch ? { "If-Match": ifMatch } : undefined,
        },
        body,
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (updated) => {
      queryClient.setQueryData(["person", id], updated);
    },
  });
}
