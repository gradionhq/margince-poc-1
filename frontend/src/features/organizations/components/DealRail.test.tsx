import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import type { Deal } from "../../../lib/api-client/generated/index.js";
import { DealRail, formatMoney } from "./DealRail.js";

describe("formatMoney", () => {
  it("uses the deal's own currency, never hard-coded EUR", () => {
    expect(formatMoney(500_00, "USD")).toContain("$");
    expect(formatMoney(500_00, "USD")).not.toContain("€");
  });
});

describe("DealRail", () => {
  it("lists deals with name/amount/stage pill, links to /deals/:id", () => {
    const deals: Deal[] = [
      {
        id: "d1",
        workspace_id: "w1",
        name: "Acme renewal",
        amount_minor: 250_000,
        currency: "EUR",
        pipeline_id: "p1",
        stage_id: "s1",
        status: "open",
        stalled: true,
        stakeholder_count: 1,
        source: "manual",
        captured_by: "human:u1",
        created_at: "",
        updated_at: "",
      },
    ];
    render(
      <MemoryRouter>
        <DealRail deals={deals} />
      </MemoryRouter>,
    );
    expect(screen.getByText("Acme renewal")).toBeInTheDocument();
    expect(screen.getByText(/2[.,]500[.,]00/)).toBeInTheDocument();
    expect(screen.getByText(/stalled/i)).toBeInTheDocument();
    expect(screen.getByText(/single-threaded/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /acme renewal/i })).toHaveAttribute(
      "href",
      "/deals/d1",
    );
  });

  it("renders an honest empty state when there are no deals", () => {
    render(
      <MemoryRouter>
        <DealRail deals={[]} />
      </MemoryRouter>,
    );
    expect(screen.getByText(/no open or won deals/i)).toBeInTheDocument();
  });
});
