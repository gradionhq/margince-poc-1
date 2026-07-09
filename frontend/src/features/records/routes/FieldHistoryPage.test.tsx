import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), PATCH: vi.fn() },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import { FieldHistoryPage } from "./FieldHistoryPage.js";

const DEAL = { id: "d1", workspace_id: "ws1", name: "BÄR Pharma — Packaging QA", amount_minor: 17707200, currency: "EUR", stage_id: "s1", owner_id: "u1", version: 3, source: "test", captured_by: "human:x", created_at: "t", updated_at: "t" };

const ENTRIES = [
  { id: "e1", entity_type: "deal", entity_id: "d1", field: "amount_minor", old_value: "21200000", new_value: "17707200", changed_at: "2026-06-18T09:42:00Z", actor_type: "agent", actor_id: "a1", passport_id: "psp1", evidence: { quote: "offer accepted", confidence: "high" } },
  { id: "e2", entity_type: "deal", entity_id: "d1", field: "stage_id", old_value: "Discovery", new_value: "Qualified", changed_at: "2026-06-12T11:20:00Z", actor_type: "human", actor_id: "u1", passport_id: null, evidence: null },
];

beforeEach(() => vi.clearAllMocks());

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children: _c }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/records/deal/d1/field-history"]}>
          <Routes>
            <Route path="/records/:entityType/:entityId/field-history" element={<FieldHistoryPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe("FieldHistoryPage", () => {
  it("STATE-2: chrome renders immediately, groups area shows a skeleton while loading", () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));
    render(<FieldHistoryPage />, { wrapper: wrapper() });
    expect(screen.getByText(/field change history/i)).toBeInTheDocument();
    expect(screen.getByTestId("field-history-skeleton")).toBeInTheDocument();
  });

  it("AC-field-history-1: header reads 'N fields · M changes' and the source-of-truth note", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation(async (path: string) => {
      if (path === "/field-history") return { data: { data: ENTRIES, page: { next_cursor: null } }, error: undefined, response: { status: 200 } };
      return { data: DEAL, error: undefined, response: { status: 200 } };
    });
    render(<FieldHistoryPage />, { wrapper: wrapper() });
    await waitFor(() => expect(screen.getByText("5 fields · 2 changes")).toBeInTheDocument());
    expect(screen.getByText(/reconstructed from the append-only audit log/i)).toBeInTheDocument();
    expect(screen.getByText(/read-only projection, not editable here/i)).toBeInTheDocument();
  });

  it("STATE-3: a field-history fetch failure shows the honest error card, never a partial timeline", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation(async (path: string) => {
      if (path === "/field-history") return { data: undefined, error: new Error("boom"), response: { status: 500 } };
      return { data: DEAL, error: undefined, response: { status: 200 } };
    });
    render(<FieldHistoryPage />, { wrapper: wrapper() });
    await waitFor(
      () => expect(screen.getByText(/couldn't load the change history/i)).toBeInTheDocument(),
      { timeout: 5000 },
    );
  });

  it("STATE-4: a 403 on the field-history fetch shows the distinct no-access card, not the generic error", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation(async (path: string) => {
      if (path === "/field-history") return { data: undefined, error: { code: "forbidden" }, response: { status: 403 } };
      return { data: DEAL, error: undefined, response: { status: 200 } };
    });
    render(<FieldHistoryPage />, { wrapper: wrapper() });
    await waitFor(() => expect(screen.getByText(/you don't have access/i)).toBeInTheDocument());
    expect(screen.queryByText(/couldn't load the change history/i)).not.toBeInTheDocument();
  });

  it("AC-field-history-3/4/5: filtering and Clear filters compose end-to-end", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation(async (path: string) => {
      if (path === "/field-history") return { data: { data: ENTRIES, page: { next_cursor: null } }, error: undefined, response: { status: 200 } };
      return { data: DEAL, error: undefined, response: { status: 200 } };
    });
    render(<FieldHistoryPage />, { wrapper: wrapper() });
    await waitFor(() => expect(screen.getByTestId("field-history-group-amount_minor")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("radio", { name: /^agent$/i }));
    expect(screen.queryByTestId("field-history-group-stage_id")).not.toBeInTheDocument();
    await userEvent.type(screen.getByPlaceholderText(/search fields/i), "zzz-no-match");
    expect(screen.getByText(/no changes match this filter/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(screen.getByTestId("field-history-group-stage_id")).toBeInTheDocument();
  });
});
