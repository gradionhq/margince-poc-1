import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { MemberListResponse } from "../../../lib/api-client/generated/index.js";

export function useMembers() {
  return useQuery<MemberListResponse>({
    queryKey: ["members"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/members", {
        params: { query: {} },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}
