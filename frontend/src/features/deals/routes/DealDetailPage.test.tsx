import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

const advanceMutate = vi.fn();
const refetchDeal = vi.fn();
const dealData = {
  id: "d1",
  name: "Acme deal",
  amount_minor: 1000000,
  currency: "USD",
  status: "open",
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

describe("DealDetailPage", () => {
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
});
