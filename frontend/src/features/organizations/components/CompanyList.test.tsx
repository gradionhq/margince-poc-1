import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CompanyList } from "./CompanyList.js";

describe("CompanyList", () => {
  it("renders an honest empty state, no fabricated counts", () => {
    render(
      <CompanyList
        companies={[]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.queryByRole("row", { name: /./ })).not.toBeInTheDocument();
    expect(screen.getByText(/no companies/i)).toBeInTheDocument();
  });
  it("shows a skeleton while loading", () => {
    render(
      <CompanyList
        companies={[]}
        isLoading={true}
        isError={false}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByTestId("company-list-skeleton")).toBeInTheDocument();
  });
  it("shows an error card with a working retry on failure", () => {
    const onRetry = vi.fn();
    render(
      <CompanyList
        companies={[]}
        isLoading={false}
        isError={true}
        onRetry={onRetry}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });
  it("renders company/contacts/open-deals/org-strength columns for a seeded row", () => {
    render(
      <CompanyList
        companies={[
          {
            id: "o1",
            display_name: "Acme Inc",
            industry: "Software",
            contact_count: 4,
            open_deal_count: 2,
            org_strength: {
              score: 81,
              bucket: "strong",
              top_person_id: "p1",
              top_person_name: "Dana Buyer",
            },
          } as never,
        ]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByText("Acme Inc")).toBeInTheDocument();
    expect(screen.getByText("Software")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("81")).toBeInTheDocument();
  });
  it("renders an honest no-signal state for a company with null org_strength (STATE-5)", () => {
    render(
      <CompanyList
        companies={[
          {
            id: "o2",
            display_name: "Silent Corp",
            industry: "Finance",
            contact_count: 1,
            open_deal_count: 0,
            org_strength: null,
          } as never,
        ]}
        isLoading={false}
        isError={false}
        onRetry={vi.fn()}
      />,
    );
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
  });
  it("renders an honest error card (STATE-3) without crashing on 403 (STATE-4)", () => {
    const onRetry = vi.fn();
    render(
      <CompanyList
        companies={[]}
        isLoading={false}
        isError={true}
        onRetry={onRetry}
      />,
    );
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });
});
