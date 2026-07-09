import { useQuery } from "@tanstack/react-query";
import { apiClient } from "../../../lib/api-client/client.js";
import type { components } from "../../../lib/api-client/generated/index.js";
import { formatMoneyDeDE } from "../components/RollupTilesBand.js";

export type FieldHistoryEntry = components["schemas"]["FieldHistoryEntry"];
export type EntityType = FieldHistoryEntry["entity_type"];
export type ActorType = FieldHistoryEntry["actor_type"];

export const fieldHistoryKeys = {
  list: (entityType?: string, entityId?: string) =>
    ["field-history", "list", entityType, entityId] as const,
  entity: (entityType?: string, entityId?: string) =>
    ["field-history", "entity", entityType, entityId] as const,
};

export class FieldHistoryForbiddenError extends Error {
  constructor() {
    super("You don't have access to this record's field history.");
    this.name = "FieldHistoryForbiddenError";
  }
}

export function shouldRetryFieldHistory(failureCount: number, error: unknown): boolean {
  if (error instanceof FieldHistoryForbiddenError) return false;
  return failureCount < 2;
}

export function useFieldHistory(entityType: EntityType | undefined, entityId: string | undefined) {
  return useQuery<FieldHistoryEntry[]>({
    queryKey: fieldHistoryKeys.list(entityType, entityId),
    enabled: !!entityType && !!entityId,
    retry: shouldRetryFieldHistory,
    queryFn: async () => {
      const { data, error, response } = await apiClient.GET("/field-history", {
        params: {
          query: { entity_type: entityType as EntityType, entity_id: entityId as string, limit: 200 },
        },
      });
      if (response?.status === 403) throw new FieldHistoryForbiddenError();
      if (error) throw error;
      return data?.data ?? [];
    },
  });
}

export function useEntityRecord(entityType: EntityType | undefined, entityId: string | undefined) {
  return useQuery<Record<string, unknown>>({
    queryKey: fieldHistoryKeys.entity(entityType, entityId),
    enabled: !!entityType && !!entityId,
    retry: (failureCount, error) => {
      if (error instanceof FieldHistoryForbiddenError) return false;
      return failureCount < 2;
    },
    queryFn: async () => {
      const id = entityId as string;
      let result: { data?: unknown; error?: unknown; response?: { status: number } };
      switch (entityType as EntityType) {
        case "person":
          result = await apiClient.GET("/people/{id}", { params: { path: { id } } });
          break;
        case "organization":
          result = await apiClient.GET("/organizations/{id}", { params: { path: { id } } });
          break;
        case "deal":
          result = await apiClient.GET("/deals/{id}", { params: { path: { id } } });
          break;
        case "lead":
          result = await apiClient.GET("/leads/{id}", { params: { path: { id } } });
          break;
        case "activity":
          result = await apiClient.GET("/activities/{id}", { params: { path: { id } } });
          break;
        default:
          result = { data: undefined, error: new Error(`unknown entity_type: ${String(entityType)}`) };
      }
      if (result.response?.status === 403) throw new FieldHistoryForbiddenError();
      if (result.error) throw result.error;
      if (!result.data) throw new Error("empty response");
      return result.data as Record<string, unknown>;
    },
  });
}

export const SYSTEM_FIELD_KEYS = new Set([
  "id", "workspace_id", "version", "created_at", "updated_at", "archived_at", "source", "captured_by",
]);

export function isDiffableFieldValue(v: unknown): boolean {
  return v === null || v === undefined || typeof v !== "object";
}

export function scalarFieldKeys(record: Record<string, unknown> | undefined): string[] {
  if (!record) return [];
  return Object.keys(record).filter(
    (k) => !SYSTEM_FIELD_KEYS.has(k) && isDiffableFieldValue(record[k]),
  );
}

export function humanizeFieldKey(field: string): string {
  const words = field.replace(/_minor$/, "").split("_").filter(Boolean);
  if (words.length === 0) return field;
  return words.map((w, i) => (i === 0 ? w.charAt(0).toUpperCase() + w.slice(1) : w)).join(" ");
}

export const COMPUTED_MONEY_FIELD = "amount_minor";

export function parseMinorUnits(value: string | null | undefined): number | null {
  if (value == null) return null;
  const n = Number(value);
  return Number.isFinite(n) ? Math.round(n) : null;
}

export function computeNetTaxGross(grossMinor: number): { netMinor: number; taxMinor: number } {
  const netMinor = Math.round(grossMinor / 1.19);
  return { netMinor, taxMinor: grossMinor - netMinor };
}

export function formatCurrentFieldValue(field: string, value: unknown, currency: string): string {
  if (value === null || value === undefined || value === "") return "— empty —";
  if (field === COMPUTED_MONEY_FIELD && typeof value === "number") {
    return formatMoneyDeDE(value, currency);
  }
  return String(value);
}

export function formatDiffFieldValue(field: string, value: string, currency: string): string {
  if (field === COMPUTED_MONEY_FIELD) {
    const minor = parseMinorUnits(value);
    if (minor !== null) return formatMoneyDeDE(minor, currency);
  }
  return value;
}

export function originLabel(
  entry: FieldHistoryEntry,
  fieldEntriesNewestFirst: FieldHistoryEntry[],
): "— empty —" | "— created —" {
  const oldest = fieldEntriesNewestFirst[fieldEntriesNewestFirst.length - 1];
  return oldest?.id === entry.id ? "— created —" : "— empty —";
}
