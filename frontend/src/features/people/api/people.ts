import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { PersonListResponse } from "../../../lib/api-client/generated/index.js";

export function usePeople() {
  return useQuery<PersonListResponse>({
    queryKey: ["people"],
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/people");
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}
