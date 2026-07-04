import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  Deal,
  DealDetail,
  DealListResponse,
  Pipeline,
  PipelineRollup,
  Stage,
} from "../../../lib/api-client/generated/index.js";

export const dealsKeys = {
  all: ["deals"] as const,
  list: (pipelineId?: string, stageId?: string, status?: string) =>
    ["deals", "list", pipelineId, stageId, status] as const,
  rollup: (pipelineId?: string) => ["deals", "rollup", pipelineId] as const,
  detail: (id?: string) => ["deals", "detail", id] as const,
  pipelines: ["pipelines"] as const,
  stages: (pipelineId?: string) => ["stages", pipelineId] as const,
};

export function useDefaultPipeline() {
  return useQuery<Pipeline | undefined>({
    queryKey: dealsKeys.pipelines,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/pipelines", {
        params: { query: {} },
      });
      if (error) throw error;
      return data?.data.find((p) => p.is_default) ?? data?.data[0];
    },
  });
}

export function useStages(pipelineId: string | undefined) {
  return useQuery<Stage[]>({
    queryKey: dealsKeys.stages(pipelineId),
    enabled: !!pipelineId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/stages", {
        params: { query: { pipeline_id: pipelineId } },
      });
      if (error) throw error;
      return (data?.data ?? []).slice().sort((a, b) => a.position - b.position);
    },
  });
}

export function useDeals(filters: {
  pipelineId?: string;
  stageId?: string;
  status?: "open" | "won" | "lost";
}) {
  return useQuery<DealListResponse>({
    queryKey: dealsKeys.list(filters.pipelineId, filters.stageId, filters.status),
    enabled: !!filters.pipelineId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/deals", {
        params: {
          query: {
            pipeline_id: filters.pipelineId,
            stage_id: filters.stageId,
            status: filters.status,
          },
        },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function usePipelineRollup(pipelineId: string | undefined) {
  return useQuery<PipelineRollup>({
    queryKey: dealsKeys.rollup(pipelineId),
    enabled: !!pipelineId,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(
        `/pipelines/${pipelineId}/rollup` as "/pipelines/{id}/rollup",
        { params: { path: { id: pipelineId as string } } },
      );
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useDeal(id: string | undefined) {
  return useQuery<DealDetail>({
    queryKey: dealsKeys.detail(id),
    enabled: !!id,
    queryFn: async () => {
      const { data, error } = await apiClient.GET(`/deals/${id}` as "/deals/{id}", {
        params: { path: { id: id as string } },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export type { Deal };
