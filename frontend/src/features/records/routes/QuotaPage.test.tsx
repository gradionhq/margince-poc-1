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
import { QuotaPage } from "./QuotaPage.js";

const QUOTA_ID = "q1";

const quota = {
  id: QUOTA_ID,
  workspace_id: "ws-1",
  owner_id: "u1",
  team_id: null,
  period_start: "2026-07-01",
  period_end: "2026-09-30",
  target_minor: 28000000,
  currency: "EUR",
  version: 3,
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  archived_at: null,
};

const attainment = {
  quota_id: QUOTA_ID,
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112.1,
  gap_minor: 3387200,
  pace_pct: 92.4,
  band: "met" as const,
  as_of_date: "2026-07-09",
  contributing_deals: [
    { deal_id: "d1", base_value_minor: 17707200 },
    { deal_id: "d2", base_value_minor: 13680000 },
  ],
};

function wrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={[`/quotas/${QUOTA_ID}`]}>
          <Routes>
            <Route path="/quotas/:id" element={<QuotaPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    );
  };
}

beforeEach(() => vi.clearAllMocks());

describe("QuotaPage", () => {
  it("renders the quota screen from the real route element", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({ data: quota, error: undefined, response: { status: 200 } });
      }
      if (path === "/quotas/{id}/attainment") {
        return Promise.resolve({
          data: attainment,
          error: undefined,
          response: { status: 200 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({
          data: { data: [{ user_id: "u1", display_name: "Riya Mehta" }] },
          error: undefined,
        });
      }
      if (path === "/quotas") {
        return Promise.resolve({
          data: { data: [quota] },
          error: undefined,
        });
      }
      if (path === "/deals/{id}") {
        return Promise.resolve({
          data: { id: "d1", name: "BÄR Pharma — Packaging QA", closed_at: "2026-08-14T00:00:00Z" },
          error: undefined,
        });
      }
      return Promise.resolve({ data: undefined, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    await waitFor(() => expect(screen.getByText(/quota & attainment/i)).toBeInTheDocument());
  });

  it("surfaces the target-zero state and keeps the target editor visible", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({ data: quota, error: undefined, response: { status: 200 } });
      }
      if (path === "/quotas/{id}/attainment") {
        return Promise.resolve({
          data: undefined,
          error: { code: "attainment_target_zero" },
          response: { status: 422 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({
          data: { data: [{ user_id: "u1", display_name: "Riya Mehta" }] },
          error: undefined,
        });
      }
      return Promise.resolve({ data: undefined, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    await waitFor(() =>
      expect(
        screen.getByText(/set a target below to start tracking attainment/i),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: /save target/i })).toBeInTheDocument();
  });

  it("shows the quota permission state for a 403", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({
          data: undefined,
          error: { code: "forbidden" },
          response: { status: 403 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      return Promise.resolve({ data: undefined, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    await waitFor(() =>
      expect(screen.getByText(/you don't have access to this quota/i)).toBeInTheDocument(),
    );
  });

  it("toasts period navigation and validates target input before save", async () => {
    const user = userEvent.setup();
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({ data: quota, error: undefined, response: { status: 200 } });
      }
      if (path === "/quotas/{id}/attainment") {
        return Promise.resolve({
          data: attainment,
          error: undefined,
          response: { status: 200 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({
          data: { data: [{ user_id: "u1", display_name: "Riya Mehta" }] },
          error: undefined,
        });
      }
      if (path === "/quotas") {
        return Promise.resolve({
          data: { data: [quota] },
          error: undefined,
        });
      }
      if (path === "/deals/{id}") {
        return Promise.resolve({
          data: { id: "d1", name: "BÄR Pharma — Packaging QA", closed_at: "2026-08-14T00:00:00Z" },
          error: undefined,
        });
      }
      return Promise.resolve({ data: undefined, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    await waitFor(() => expect(screen.getByText("Q3 2026 · current")).toBeInTheDocument());
    await user.click(screen.getByRole("button", { name: /q2 2026/i }));
    expect(screen.getByText(/q2 2026 is closed — read-only/i)).toBeInTheDocument();

    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "abc");
    await user.click(screen.getByRole("button", { name: /save target/i }));
    expect(screen.getByText(/enter a target amount in eur/i)).toBeInTheDocument();
    expect(apiClient.PATCH).not.toHaveBeenCalled();
  });
});
