import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/deals.js", () => ({
  useDefaultPipeline: () => ({ data: { id: "p1" } }),
  useStages: () => ({
    data: [
      { id: "s0", name: "New", position: 0, semantic: "open", win_probability: 10 },
    ],
  }),
  useDeals: () => ({ data: { data: [] }, isLoading: false, isError: false, refetch: vi.fn() }),
  usePipelineRollup: () => ({ data: undefined, isLoading: false, isError: false }),
  useAdvanceDeal: () => ({ mutate: vi.fn(), isPending: false }),
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
