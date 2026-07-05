import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

const archiveMutate = vi.fn();

vi.mock("../api/deals.js", () => ({
  useDefaultPipeline: () => ({ data: { id: "p1" } }),
  useStages: () => ({
    data: [
      {
        id: "s0",
        name: "New",
        position: 0,
        semantic: "open",
        win_probability: 10,
      },
    ],
  }),
  useDeals: () => ({
    data: {
      data: [
        {
          id: "d1",
          name: "Acme deal",
          amount_minor: 100000,
          currency: "USD",
          pipeline_id: "p1",
          stage_id: "s0",
          status: "open",
          source: "manual",
          captured_by: "human:u1",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-01T00:00:00Z",
          stalled: false,
          stakeholder_count: 2,
          stage_entered_at: "2026-07-01T00:00:00Z",
        },
      ],
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  usePipelineRollup: () => ({
    data: undefined,
    isLoading: false,
    isError: false,
  }),
  useAdvanceDeal: () => ({ mutate: vi.fn(), isPending: false }),
  useArchiveDeal: () => ({
    mutate: archiveMutate,
    isPending: false,
  }),
}));

import { PipelinePage } from "./PipelinePage.js";

function renderPage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PipelinePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PipelinePage", () => {
  it("opens the archive dialog from the board and shows a success toast after confirm", async () => {
    archiveMutate.mockImplementation((_vars, opts) => opts?.onSuccess?.());
    const userEventModule = await import("@testing-library/user-event");
    const user = userEventModule.default.setup();

    renderPage();

    await user.click(screen.getByRole("button", { name: /row actions/i }));
    await user.click(screen.getByRole("menuitem", { name: "Archive" }));

    expect(screen.getByText(/acme deal will be removed/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Archive" }));

    expect(archiveMutate).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({
        onSuccess: expect.any(Function),
        onError: expect.any(Function),
      }),
    );
    expect(screen.getByText("Acme deal archived")).toBeInTheDocument();
  });

  it("renders the Deals heading and the New stage column", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: /deals/i })).toBeInTheDocument();
    expect(screen.getByTestId("stage-column-s0")).toBeInTheDocument();
  });

  it("toggles to Table view without losing the pipeline/stage filter, and back", async () => {
    const userEventModule = await import("@testing-library/user-event");
    const user = userEventModule.default.setup();
    renderPage();
    await user.click(screen.getByRole("radio", { name: /table/i }));
    expect(screen.getByRole("table")).toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: /board/i }));
    expect(screen.getByTestId("stage-column-s0")).toBeInTheDocument();
  });
});
