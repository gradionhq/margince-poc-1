import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), PATCH: vi.fn() },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  COMPUTED_MONEY_FIELD,
  FieldHistoryForbiddenError,
  computeNetTaxGross,
  formatCurrentFieldValue,
  formatDiffFieldValue,
  humanizeFieldKey,
  isDiffableFieldValue,
  originLabel,
  parseMinorUnits,
  scalarFieldKeys,
  shouldRetryFieldHistory,
  useEntityRecord,
  useFieldHistory,
} from "./fieldHistory.js";

beforeEach(() => vi.clearAllMocks());

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

const ENTRY_BASE = {
  id: "e1", entity_type: "deal" as const, entity_id: "d1", field: "amount_minor",
  changed_at: "2026-06-18T09:42:00Z", actor_type: "agent" as const, actor_id: "a1",
  passport_id: "psp1", evidence: { quote: "offer accepted", confidence: "high" },
};

describe("useFieldHistory", () => {
  it("calls GET /field-history with entity_type/entity_id/limit:200", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [ENTRY_BASE], page: { next_cursor: null } }, error: undefined, response: { status: 200 },
    });
    const { result } = renderHook(() => useFieldHistory("deal", "d1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/field-history",
      expect.objectContaining({
        params: { query: { entity_type: "deal", entity_id: "d1", limit: 200 } },
      }),
    );
    expect(result.current.data).toEqual([ENTRY_BASE]);
  });

  it("STATE-4: 403 surfaces FieldHistoryForbiddenError, never retried", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: undefined, error: { code: "forbidden" }, response: { status: 403 },
    });
    const { result } = renderHook(() => useFieldHistory("deal", "d1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(FieldHistoryForbiddenError);
    expect(apiClient.GET).toHaveBeenCalledTimes(1);
  });

  it("AC-field-history-8: 200 {data:[]} for a record with no history is honest empty, not an error", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { data: [], page: { next_cursor: null } }, error: undefined, response: { status: 200 },
    });
    const { result } = renderHook(() => useFieldHistory("deal", "d1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});

describe("useEntityRecord", () => {
  it.each([
    ["person", "/people/{id}"], ["organization", "/organizations/{id}"], ["deal", "/deals/{id}"],
    ["lead", "/leads/{id}"], ["activity", "/activities/{id}"],
  ] as const)("routes entity_type=%s to %s", async (entityType, path) => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "x1", name: "X" }, error: undefined, response: { status: 200 },
    });
    const { result } = renderHook(() => useEntityRecord(entityType, "x1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(path, expect.objectContaining({ params: { path: { id: "x1" } } }));
  });

  it("STATE-4: 403 surfaces FieldHistoryForbiddenError (same sentinel as useFieldHistory)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: undefined, error: { code: "forbidden" }, response: { status: 403 },
    });
    const { result } = renderHook(() => useEntityRecord("deal", "d1"), { wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error).toBeInstanceOf(FieldHistoryForbiddenError);
  });
});

describe("shouldRetryFieldHistory", () => {
  it("never retries FieldHistoryForbiddenError", () => {
    expect(shouldRetryFieldHistory(0, new FieldHistoryForbiddenError())).toBe(false);
  });
  it("bounds retries for any other error", () => {
    expect(shouldRetryFieldHistory(0, new Error("blip"))).toBe(true);
    expect(shouldRetryFieldHistory(2, new Error("blip"))).toBe(false);
  });
});

describe("scalarFieldKeys / isDiffableFieldValue", () => {
  it("excludes system metadata and object/array-valued keys, keeps scalars", () => {
    const record = {
      id: "d1", workspace_id: "ws1", version: 3, created_at: "t", updated_at: "t",
      source: "test", captured_by: "human:x",
      name: "BÄR Pharma", amount_minor: 17707200, owner_id: null,
      domains: [{ id: "x" }], meta: { a: 1 },
    };
    const keys = scalarFieldKeys(record);
    expect(keys).toEqual(expect.arrayContaining(["name", "amount_minor", "owner_id"]));
    expect(keys).not.toEqual(expect.arrayContaining(["id", "workspace_id", "domains", "meta"]));
  });
  it("isDiffableFieldValue: null/primitive true, object/array false", () => {
    expect(isDiffableFieldValue(null)).toBe(true);
    expect(isDiffableFieldValue(42)).toBe(true);
    expect(isDiffableFieldValue("x")).toBe(true);
    expect(isDiffableFieldValue({})).toBe(false);
    expect(isDiffableFieldValue([])).toBe(false);
  });
});

describe("humanizeFieldKey", () => {
  it("amount_minor -> Amount (drops the _minor unit suffix)", () => {
    expect(humanizeFieldKey("amount_minor")).toBe("Amount");
  });
  it("close_date -> Close date (sentence case)", () => {
    expect(humanizeFieldKey("close_date")).toBe("Close date");
  });
  it("owner_id -> Owner id", () => {
    expect(humanizeFieldKey("owner_id")).toBe("Owner id");
  });
});

describe("parseMinorUnits", () => {
  it("parses Go's decimal float64 stringification", () => {
    expect(parseMinorUnits("17707200")).toBe(17707200);
  });
  it("parses Go's scientific-notation float64 stringification (valid JS numeric syntax too)", () => {
    expect(parseMinorUnits("1.77072e+07")).toBe(17707200);
  });
  it("returns null for null/non-numeric input", () => {
    expect(parseMinorUnits(null)).toBeNull();
    expect(parseMinorUnits("Discovery")).toBeNull();
  });
});

describe("computeNetTaxGross", () => {
  it("AC-field-history-7's exact worked example: 17707200 -> net 14880000, tax 2827200", () => {
    expect(computeNetTaxGross(17707200)).toEqual({ netMinor: 14880000, taxMinor: 2827200 });
  });
});

describe("formatCurrentFieldValue / formatDiffFieldValue", () => {
  it("formats the computed money field via formatMoneyDeDE, others as plain strings", () => {
    expect(formatCurrentFieldValue(COMPUTED_MONEY_FIELD, 17707200, "EUR")).toMatch(/177\.072,00/);
    expect(formatCurrentFieldValue("stage_id", "Proposal sent", "EUR")).toBe("Proposal sent");
  });
  it("null/undefined/empty current value renders — empty —", () => {
    expect(formatCurrentFieldValue("owner_id", null, "EUR")).toBe("— empty —");
  });
  it("diff-token money field reparses the raw stringified minor units and reformats", () => {
    expect(formatDiffFieldValue(COMPUTED_MONEY_FIELD, "17707200", "EUR")).toMatch(/177\.072,00/);
  });
  it("diff-token non-money field renders the raw API string verbatim (never reformatted/guessed)", () => {
    expect(formatDiffFieldValue("stage_id", "Discovery", "EUR")).toBe("Discovery");
  });
});

describe("originLabel", () => {
  it("the oldest entry in a field's own timeline with old_value=null renders — created —", () => {
    const newest = { ...ENTRY_BASE, id: "e2", old_value: "Qualified", new_value: "Proposal", changed_at: "2026-06-12T00:00:00Z" };
    const oldest = { ...ENTRY_BASE, id: "e1", old_value: null, new_value: "Discovery", changed_at: "2026-05-29T16:09:00Z" };
    const timeline = [newest, oldest];
    expect(originLabel(oldest, timeline)).toBe("— created —");
  });
  it("a non-oldest null-old_value entry (cleared then re-set) renders — empty —", () => {
    const oldest = { ...ENTRY_BASE, id: "e1", old_value: null, new_value: "first", changed_at: "2026-01-01T00:00:00Z" };
    const middle = { ...ENTRY_BASE, id: "e2", old_value: null, new_value: "second", changed_at: "2026-02-01T00:00:00Z" };
    const timeline = [middle, oldest];
    expect(originLabel(middle, timeline)).toBe("— empty —");
  });
});
