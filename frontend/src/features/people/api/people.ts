import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { PersonListResponse } from "../../../lib/api-client/generated/index.js";

export function usePeople(opts?: { sort?: string; q?: string }) {
  return useQuery<PersonListResponse>({
    queryKey: ["people", opts?.sort, opts?.q],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/people", {
        params: { query: { sort: opts?.sort, q: opts?.q } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}
