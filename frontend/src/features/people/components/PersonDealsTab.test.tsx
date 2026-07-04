import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/person.js", () => ({ usePersonDeals: vi.fn() }));

import * as personApi from "../api/person.js";
import { PersonDealsTab } from "./PersonDealsTab.js";

const mockDeals = vi.mocked(personApi.usePersonDeals);

function renderTab() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <PersonDealsTab personId="p1" />
    </QueryClientProvider>,
  );
}

describe("PersonDealsTab", () => {
  it("renders a Skeleton while loading (STATE-2)", () => {
    mockDeals.mockReturnValue({ data: undefined, isLoading: true, isError: false } as never);
    renderTab();
    expect(screen.getByTestId("person-deals-loading")).toBeInTheDocument();
  });

  it("renders the honest empty state with no deals (STATE-1)", () => {
    mockDeals.mockReturnValue({ data: [], isLoading: false, isError: false } as never);
    renderTab();
    expect(screen.getByText(/no deals for this person yet/i)).toBeInTheDocument();
  });

  it("renders a read-only deal row: name, status badge, formatted amount", () => {
    mockDeals.mockReturnValue({
      data: [
        {
          id: "d1",
          name: "Acme renewal",
          status: "open",
          amount_minor: 500000,
          currency: "USD",
        },
      ],
      isLoading: false,
      isError: false,
    } as never);
    renderTab();
    expect(screen.getByText("Acme renewal")).toBeInTheDocument();
    expect(screen.getByText("open")).toBeInTheDocument();
    expect(screen.getByText("$5,000.00")).toBeInTheDocument();
  });
});
