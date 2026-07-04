import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/person.js", () => ({
  usePerson: vi.fn(),
  useOrganizationName: vi.fn(() => ({ data: undefined, isLoading: false })),
  useUpdatePerson: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
    error: null,
  })),
  usePersonStrengthBreakdown: vi.fn(() => ({
    data: undefined,
    isLoading: false,
  })),
  usePersonDeals: vi.fn(() => ({ data: [], isLoading: false, isError: false })),
  useMergePerson: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
    error: null,
    reset: vi.fn(),
  })),
}));

import * as personApi from "../api/person.js";
import { PersonDetailPage } from "./PersonDetailPage.js";

const mockUsePerson = vi.mocked(personApi.usePerson);

function renderAt(id: string) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[`/people/${id}`]}>
        <Routes>
          <Route path="/people/:id" element={<PersonDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PersonDetailPage", () => {
  it("renders a Skeleton while loading (STATE-2)", () => {
    mockUsePerson.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof personApi.usePerson>);
    renderAt("p1");
    expect(screen.getByTestId("person-detail-loading")).toBeInTheDocument();
  });

  it("renders cause + retry on fetch error, not a blank screen (STATE-3)", () => {
    const refetch = vi.fn();
    mockUsePerson.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: { detail: "Network unreachable" },
      refetch,
    } as unknown as ReturnType<typeof personApi.usePerson>);
    renderAt("p1");
    expect(screen.getByTestId("person-detail-error")).toHaveTextContent(
      "Network unreachable",
    );
    screen.getByRole("button", { name: /retry/i }).click();
    expect(refetch).toHaveBeenCalled();
  });

  it("renders the loaded body once data resolves", () => {
    mockUsePerson.mockReturnValue({
      data: {
        id: "p1",
        full_name: "Alice",
        source: "manual",
        captured_by: "human:u1",
        emails: [],
        phones: [],
        relationships: [],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof personApi.usePerson>);
    renderAt("p1");
    expect(screen.getByTestId("person-detail-loaded")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /merge/i })).toBeInTheDocument();
  });

  it("renders header, strength card, tabs, and merge trigger together on a full load", () => {
    mockUsePerson.mockReturnValue({
      data: {
        id: "p1",
        full_name: "Alice",
        source: "manual",
        captured_by: "human:u1",
        strength: null,
        activities: [],
        relationships: [],
        emails: [],
        phones: [],
      },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof personApi.usePerson>);
    renderAt("p1");
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
    expect(screen.getByRole("tablist")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /merge/i })).toBeInTheDocument();
  });
});
