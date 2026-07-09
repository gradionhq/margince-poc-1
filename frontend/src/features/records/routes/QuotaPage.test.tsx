import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
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

  // Names below are pinned verbatim — workspace/manual-test/rd-t12.md's `-t` filters match
  // against these exact titles.

  it("STATE-2: renders chrome immediately with a loading skeleton, then the ring once data resolves", async () => {
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
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      if (path === "/quotas") {
        return Promise.resolve({ data: { data: [quota] }, error: undefined });
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

    // Chrome (header) renders before/independent of the ring's own fetched data.
    expect(screen.getByText(/quota & attainment/i)).toBeInTheDocument();
    expect(screen.getByTestId("attainment-ring-skeleton")).toBeInTheDocument();

    // 112.1 rounds to 112 — the ring once data resolves.
    expect(await screen.findByText("112%")).toBeInTheDocument();
  });

  it("STATE-1: 422 attainment_target_zero renders the honest 'set a target' message, not the generic error", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({
          data: { ...quota, target_minor: 0 },
          error: undefined,
          response: { status: 200 },
        });
      }
      if (path === "/quotas/{id}/attainment") {
        return Promise.resolve({
          data: undefined,
          error: { code: "attainment_target_zero" },
          response: { status: 422 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      return Promise.resolve({ data: { data: [] }, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    expect(await screen.findByText(/no target set/i)).toBeInTheDocument();
    expect(screen.queryByText(/couldn't recompute/i)).not.toBeInTheDocument();
  });

  it("STATE-4: a 403 on attainment renders the honest no-access message", async () => {
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
          error: { code: "forbidden" },
          response: { status: 403 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      return Promise.resolve({ data: { data: [] }, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    expect(await screen.findByText(/don't have access/i)).toBeInTheDocument();
  });

  it("STATE-4 (PLAN-review finding): a 403 on GET /quotas/{id} itself renders the honest no-access message, never the generic 'quota not found' fallback", async () => {
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
      if (path === "/quotas/{id}/attainment") {
        return Promise.resolve({
          data: undefined,
          error: { code: "forbidden" },
          response: { status: 403 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      return Promise.resolve({ data: { data: [] }, error: undefined });
    });

    render(<QuotaPage />, { wrapper: wrapper(qc) });

    expect(await screen.findByText(/don't have access/i)).toBeInTheDocument();
    expect(screen.queryByText(/quota not found/i)).not.toBeInTheDocument();
  });

  it("STATE-3: once a prior successful fetch happened, a later generic attainment error states the last successful compute time, not stale attainment figures", async () => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
    let attainmentCalls = 0;
    (apiClient.GET as ReturnType<typeof vi.fn>).mockImplementation((path: string) => {
      if (path === "/quotas/{id}") {
        return Promise.resolve({ data: quota, error: undefined, response: { status: 200 } });
      }
      if (path === "/quotas/{id}/attainment") {
        attainmentCalls += 1;
        if (attainmentCalls === 1) {
          return Promise.resolve({
            data: attainment,
            error: undefined,
            response: { status: 200 },
          });
        }
        // A generic (non-forbidden, non-target-zero) attainment error — proves the caption isn't
        // gated behind the "attainment_computation_failed" code specifically.
        return Promise.resolve({
          data: undefined,
          error: { code: "attainment_computation_failed" },
          response: { status: 422 },
        });
      }
      if (path === "/members") {
        return Promise.resolve({ data: { data: [] }, error: undefined });
      }
      if (path === "/quotas") {
        return Promise.resolve({ data: { data: [quota] }, error: undefined });
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

    expect(await screen.findByText("112%")).toBeInTheDocument();

    await qc.invalidateQueries({ queryKey: ["quotas", "attainment", QUOTA_ID] });

    const caption = await screen.findByText(/Last successful compute:/i);
    expect(caption).toBeInTheDocument();
    // Scoped to the attainment-ring panel itself: the honest error card replaces the ring's own
    // figures rather than continuing to show a stale "112%" as if it were current (the team
    // roll-up rail's own reuse of the last-known attainment is a separate, out-of-scope concern).
    const ringPanel = caption.closest("section") as HTMLElement;
    expect(within(ringPanel).getByText(/couldn't recompute attainment/i)).toBeInTheDocument();
    expect(within(ringPanel).queryByText("112%")).not.toBeInTheDocument();
  });
});
