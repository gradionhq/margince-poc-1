import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { OrganizationListResponse } from "../../../lib/api-client/generated/index.js";

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
