import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type {
  components,
  Deal,
} from "../../../lib/api-client/generated/index.js";

export type Quota = components["schemas"]["Quota"];
export type QuotaAttainment = components["schemas"]["QuotaAttainment"];
export type QuotaAttainmentDeal = components["schemas"]["QuotaAttainmentDeal"];

export const quotasKeys = {
  detail: (id?: string) => ["quotas", "detail", id] as const,
  attainment: (id?: string) => ["quotas", "attainment", id] as const,
  list: () => ["quotas", "list"] as const,
};

// STATE-4: a 403 on the base quota record is its own permission failure, not the same as a
// missing row and not the same as a failing attainment sub-resource. Callers branch on this
// sentinel to keep the honest "denied" panel distinct from "not found".
export class QuotaForbiddenError extends Error {
  constructor() {
    super("You don't have access to this quota.");
    this.name = "QuotaForbiddenError";
  }
}

// STATE-4: mirrors the sibling roll-up module's sentinel pattern for the attainment call.
export class QuotaAttainmentForbiddenError extends Error {
  constructor() {
    super("You don't have access to this quota's attainment.");
    this.name = "QuotaAttainmentForbiddenError";
  }
}

// STATE-1: this quota has no target yet. The screen should ask for a target, not render a stale
// or invented attainment.
export class QuotaAttainmentTargetZeroError extends Error {
  constructor() {
    super(
      "This quota's target is zero; set a target to start tracking attainment.",
    );
    this.name = "QuotaAttainmentTargetZeroError";
  }
}

// STATE-3: the clean-core attainment computation failed. This is a real error, not a fallback.
export class QuotaAttainmentComputationFailedError extends Error {
  constructor() {
    super("The attainment query against the clean core failed.");
    this.name = "QuotaAttainmentComputationFailedError";
  }
}

export function shouldRetryQuotaAttainment(
  failureCount: number,
  error: unknown,
): boolean {
  if (
    error instanceof QuotaAttainmentForbiddenError ||
    error instanceof QuotaAttainmentTargetZeroError ||
    error instanceof QuotaAttainmentComputationFailedError
  ) {
    return false;
  }
  return failureCount < 2;
}

export function useQuota(id: string | undefined) {
  return useQuery<Quota>({
    queryKey: quotasKeys.detail(id),
    enabled: !!id,
    retry: (failureCount, error) => {
      if (error instanceof QuotaForbiddenError) return false;
      return failureCount < 2;
    },
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/quotas/{id}", {
        params: { path: { id: id as string } },
      });
      if (response?.status === 403) throw new QuotaForbiddenError();
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useQuotaAttainment(id: string | undefined) {
  return useQuery<QuotaAttainment>({
    queryKey: quotasKeys.attainment(id),
    enabled: !!id,
    retry: shouldRetryQuotaAttainment,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET(
        "/quotas/{id}/attainment",
        { params: { path: { id: id as string } } },
      );
      if (response?.status === 403) throw new QuotaAttainmentForbiddenError();
      if (response?.status === 422) {
        const code = (error as { code?: string } | undefined)?.code;
        if (code === "attainment_target_zero") {
          throw new QuotaAttainmentTargetZeroError();
        }
        throw new QuotaAttainmentComputationFailedError();
      }
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
  });
}

export function useUpdateQuotaTarget(id: string) {
  const qc = useQueryClient();
  return useMutation<Quota, unknown, { targetMinor: number; version: number }>({
    mutationFn: async ({ targetMinor, version }) => {
      const { data, error } = await apiClient.PATCH("/quotas/{id}", {
        params: {
          path: { id },
          header: { "If-Match": String(version) },
        },
        body: { target_minor: targetMinor },
      });
      if (error) throw error;
      if (!data) throw new Error("empty response");
      return data;
    },
    onSuccess: (data) => {
      qc.setQueryData(quotasKeys.detail(id), data);
      void qc.invalidateQueries({ queryKey: quotasKeys.attainment(id) });
    },
  });
}

export function parseGermanIntegerEuros(input: string): number {
  const digits = input.replace(/[^\d]/g, "");
  return digits ? parseInt(digits, 10) * 100 : 0;
}

export function useContributingDealDetails(dealIds: string[]) {
  const results = useQueries({
    queries: dealIds.map((id) => ({
      queryKey: ["deals", "detail", id],
      queryFn: async () => {
        const { data, error } = await apiClient.GET("/deals/{id}", {
          params: { path: { id } },
        });
        if (error) throw error;
        if (!data) throw new Error("empty response");
        return data;
      },
    })),
  });

  return dealIds.map((id, i) => ({
    id,
    data: results[i]?.data as Deal | undefined,
    isLoading: results[i]?.isLoading ?? false,
    isError: results[i]?.isError ?? false,
  }));
}

export function useTeamRollup(
  currentQuota: Quota | undefined,
  currentAttainment: QuotaAttainment | undefined,
) {
  const {
    data: page,
    isLoading: listLoading,
    isError: listError,
  } = useQuery<Quota[]>({
    queryKey: quotasKeys.list(),
    enabled: !!currentQuota,
    queryFn: async () => {
      const { data, error } = await apiClient.GET("/quotas", {
        params: { query: { limit: 20 } },
      });
      if (error) throw error;
      return data?.data ?? [];
    },
  });

  const siblings = (page ?? []).filter(
    (quota) =>
      currentQuota &&
      quota.id !== currentQuota.id &&
      quota.owner_id != null &&
      quota.period_start === currentQuota.period_start &&
      quota.period_end === currentQuota.period_end,
  );

  const attainmentResults = useQueries({
    queries: siblings.map((quota) => ({
      queryKey: quotasKeys.attainment(quota.id),
      queryFn: async () => {
        const { data, error, response } = await apiClient.GET(
          "/quotas/{id}/attainment",
          { params: { path: { id: quota.id } } },
        );
        if (response?.status === 403 || response?.status === 422) return null;
        if (error) throw error;
        return data ?? null;
      },
    })),
  });

  // The current quota's own row belongs at the front of the rail (AC-quota-8: "each rep's
  // attainment percent" includes the viewer's own) — its attainment is already loaded by the
  // caller (the ring), reused here rather than re-fetched.
  const currentRep =
    currentQuota && currentAttainment
      ? [
          {
            quota: currentQuota,
            attainment: currentAttainment,
            isCurrent: true,
          },
        ]
      : [];

  const reps = [
    ...currentRep,
    ...siblings.map((quota, i) => ({
      quota,
      attainment: attainmentResults[i]?.data as
        | QuotaAttainment
        | null
        | undefined,
      isCurrent: false,
    })),
  ];

  return {
    reps,
    isLoading: listLoading,
    isError: listError,
  };
}
