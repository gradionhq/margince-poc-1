import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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

export function useAdvanceDeal(pipelineId: string | undefined) {
  const qc = useQueryClient();
  const listKey = dealsKeys.list(pipelineId, undefined, "open");

  return useMutation<
    Deal,
    unknown,
    {
      dealId: string;
      toStageId: string;
      status?: "open" | "won" | "lost";
      lostReason?: string | null;
    },
    { previous: DealListResponse | undefined }
  >({
    mutationFn: async ({ dealId, toStageId, status, lostReason }) => {
      // Stage moves call advanceDeal, not updateDeal — advanceDeal is the verb that writes
      // deal_stage_history + emits deal.stage_changed (DEAL-WIRE-4/DEAL-WIRE-9). KNOWN CONTRACT
      // GAP: advanceDeal carries no If-Match/version param (crm.d.ts ~L5729-5788), so this call
      // cannot send the deal's version despite DEAL-AC-B2 — flagged in the PR description with a
      // follow-up ticket to add If-Match to advanceDeal in crm.yaml. We do not fabricate a
      // version field the contract doesn't accept.
      const { data, error } = await apiClient.POST("/deals/{id}/advance", {
        params: { path: { id: dealId } },
        body: { to_stage_id: toStageId, status, lost_reason: lostReason ?? undefined },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onMutate: async ({ dealId, toStageId }) => {
      const previous = qc.getQueryData<DealListResponse>(listKey);
      if (previous) {
        qc.setQueryData<DealListResponse>(listKey, {
          ...previous,
          data: previous.data.map((d) =>
            d.id === dealId
              ? { ...d, stage_id: toStageId, stage_entered_at: new Date().toISOString() }
              : d,
          ),
        });
      }
      // Cancellation doesn't need to block the synchronous cache patch above — the
      // optimistic value must be visible to the caller the instant mutate() returns.
      qc.cancelQueries({ queryKey: listKey });
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        qc.setQueryData(listKey, context.previous);
      }
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: listKey });
      qc.invalidateQueries({ queryKey: dealsKeys.rollup(pipelineId) });
    },
  });
}

export type { Deal };
