import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/deals.js", () => ({
  useDeal: () => ({
    data: {
      id: "d1",
      name: "Acme deal",
      amount_minor: 1000,
      currency: "EUR",
      status: "open",
    },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
}));

import { DealDetailPage } from "./DealDetailPage.js";

describe("DealDetailPage", () => {
  it("renders the deal name at /deals/:id", () => {
    const qc = new QueryClient();
    render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={["/deals/d1"]}>
          <Routes>
            <Route path="/deals/:id" element={<DealDetailPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    expect(screen.getByText("Acme deal")).toBeInTheDocument();
  });
});
