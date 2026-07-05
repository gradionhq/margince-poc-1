import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const advanceMutate = vi.fn();
const archiveMutate = vi.fn();
const restoreMutate = vi.fn();
const refetchDeal = vi.fn();
const dealData = {
  id: "d1",
  name: "Acme deal",
  amount_minor: 1000000,
  currency: "USD",
  status: "open",
  archived_at: null,
  pipeline_id: "pipe1",
  stage_id: "s2",
  organization_id: "org1",
  stakeholder_count: 2,
  stakeholders: [
    { id: "r1", person_id: "p1", role: "champion" },
    { id: "r2", person_id: "p2", role: "economic_buyer" },
  ],
  timeline: [],
};

vi.mock("../api/deals.js", () => ({
  dealsKeys: { history: (id?: string) => ["deals", "history", id] },
  useDeal: () => ({
    data: dealData,
    isLoading: false,
    isError: false,
    refetch: refetchDeal,
  }),
  useStages: () => ({
    data: [
      {
        id: "s1",
        name: "New",
        position: 0,
        semantic: "open",
        win_probability: 10,
      },
      {
        id: "s2",
        name: "Discovery",
        position: 1,
        semantic: "open",
        win_probability: 40,
      },
      {
        id: "s3",
        name: "Proposal",
        position: 2,
        semantic: "open",
        win_probability: 60,
      },
      {
        id: "won",
        name: "Closed Won",
        position: 100,
        semantic: "won",
        win_probability: 100,
      },
      {
        id: "lost",
        name: "Closed Lost",
        position: 101,
        semantic: "lost",
        win_probability: 0,
      },
    ],
    isLoading: false,
  }),
  useAdvanceDeal: () => ({ mutate: advanceMutate, isPending: false }),
  useArchiveDeal: () => ({ mutate: archiveMutate, isPending: false }),
  useRestoreDeal: () => ({ mutate: restoreMutate, isPending: false }),
  useDealActivities: () => ({ data: [], isLoading: false, isError: false }),
  useDealHistory: () => ({ data: [], isLoading: false, isError: false }),
}));

vi.mock("../../people/api/people.js", () => ({
  usePerson: (id: string) => ({
    data: { id, full_name: id === "p1" ? "Dana Lee" : "Sam Ito" },
    isLoading: false,
  }),
}));

import { DealDetailPage } from "./DealDetailPage.js";

function renderPage() {
  const qc = new QueryClient();
  const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
  render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/deals/d1"]}>
        <Routes>
          <Route path="/deals/:id" element={<DealDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { invalidateSpy };
}

const originalStageId = dealData.stage_id;
const originalStatus = dealData.status;

describe("DealDetailPage", () => {
  beforeEach(() => {
    advanceMutate.mockReset();
    archiveMutate.mockReset();
    restoreMutate.mockReset();
    refetchDeal.mockReset();
  });

  afterEach(() => {
    dealData.stage_id = originalStageId;
    dealData.status = originalStatus;
    dealData.archived_at = null;
  });

  it("renders header, stepper, weighted-value, stakeholders rail, tasks/timeline/history cards", () => {
    renderPage();
    expect(screen.getByText("Acme deal")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /view company/i })).toHaveAttribute(
      "href",
      "/companies/org1",
    );
    expect(screen.getByTestId("stage-stepper")).toBeInTheDocument();
    expect(screen.getByText("Dana Lee")).toBeInTheDocument();
    expect(screen.getByText("Multi-threaded")).toBeInTheDocument();
    expect(screen.getByTestId("activity-timeline-card")).toBeInTheDocument();
    expect(screen.getByTestId("stage-history-card")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-card")).toBeInTheDocument();
  });

  it("Advance to an open stage calls advanceDeal directly, no dialog", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    expect(advanceMutate).toHaveBeenCalledWith(
      expect.objectContaining({ dealId: "d1", toStageId: "s3" }),
      expect.anything(),
    );
    expect(screen.queryByText(/confirm the outcome/i)).not.toBeInTheDocument();
  });

  it("a successful advance refetches the deal detail and invalidates its history query — the screen must not go stale after its own mutation", async () => {
    advanceMutate.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    const { invalidateSpy } = renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    expect(refetchDeal).toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["deals", "history", "d1"],
    });
  });

  it("Advance into a terminal stage opens OutcomeDialog; confirming Won closes at 100% weighted", async () => {
    dealData.stage_id = "s3";
    advanceMutate.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    expect(screen.getByText(/confirm the outcome/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^won$/i }));

    expect(advanceMutate).toHaveBeenCalledWith(
      {
        dealId: "d1",
        toStageId: "won",
        status: "won",
      },
      expect.anything(),
    );
    expect(screen.getByText(/weighted 100%/i)).toBeInTheDocument();
  });

  it("Advance into a terminal stage, confirming Lost with a reason closes at 0% weighted", async () => {
    dealData.stage_id = "s3";
    advanceMutate.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    const input = screen.getByPlaceholderText(/reason/i);
    await userEvent.type(input, "Budget cut");
    await userEvent.click(
      screen.getByRole("button", { name: /confirm lost/i }),
    );

    expect(advanceMutate).toHaveBeenCalledWith(
      {
        dealId: "d1",
        toStageId: "lost",
        status: "lost",
        lostReason: "Budget cut",
      },
      expect.anything(),
    );
    expect(screen.getByText(/weighted 0%/i)).toBeInTheDocument();
  });

  it("a failed terminal-close renders an honest error toast via advanceErrorMessage", async () => {
    dealData.stage_id = "s3";
    advanceMutate.mockImplementation((_vars, opts) =>
      opts?.onError?.({ detail: "Stage locked by another user" }),
    );
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^advance$/i }));
    await userEvent.click(screen.getByRole("button", { name: /^won$/i }));

    expect(
      screen.getByText(/stage locked by another user/i),
    ).toBeInTheDocument();
  });

  it("Reopen renders for a closed deal; confirming calls advanceDeal to the first open stage and refreshes detail + history", async () => {
    dealData.status = "won";
    advanceMutate.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    const { invalidateSpy } = renderPage();

    expect(
      screen.queryByRole("button", { name: /^advance$/i }),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /^reopen$/i }));
    expect(screen.getByText(/reopen this deal\?/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^confirm$/i }));

    expect(advanceMutate).toHaveBeenCalledWith(
      { dealId: "d1", toStageId: "s1", status: "open" },
      expect.anything(),
    );
    expect(screen.getByText(/deal reopened/i)).toBeInTheDocument();
    expect(refetchDeal).toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: ["deals", "history", "d1"],
    });
  });

  it("shows the archived banner and a restore action for an archived deal", async () => {
    dealData.archived_at = "2026-07-05T00:00:00Z";
    renderPage();

    expect(screen.getByTestId("archived-banner")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^restore$/i }),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^restore$/i }));

    expect(restoreMutate).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({
        onSuccess: expect.any(Function),
        onError: expect.any(Function),
      }),
    );
  });

  it("renders a generic restore toast on a 409 and no existing-record link", async () => {
    dealData.archived_at = "2026-07-05T00:00:00Z";
    restoreMutate.mockImplementation((_vars, opts) =>
      opts?.onError?.({
        status: 409,
        code: "duplicate_name",
        detail: "A live deal already has this name.",
      }),
    );

    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /^restore$/i }));

    expect(
      screen.getByText(/a live deal already has this name/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /already live as a different record/i }),
    ).not.toBeInTheDocument();
  });

  it("shows Archive when the deal is live and not the archived banner", async () => {
    renderPage();

    expect(screen.queryByTestId("archived-banner")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^archive…$/i }),
    ).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^archive…$/i }));
    expect(
      screen.getByText(/acme deal will be removed from the default list/i),
    ).toBeInTheDocument();
  });
});
