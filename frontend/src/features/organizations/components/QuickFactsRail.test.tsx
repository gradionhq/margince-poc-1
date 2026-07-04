import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { QuickFactsRail } from "./QuickFactsRail.js";

describe("QuickFactsRail", () => {
  it("shows Owner/Open deals/Won lifetime/People known/First seen/Last touch", () => {
    render(
      <QuickFactsRail
        org={{
          id: "org1",
          workspace_id: "w1",
          display_name: "Acme",
          source: "manual",
          captured_by: "human:u1",
          owner_id: "u1",
          open_deal_count: 3,
          contact_count: 5,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-06-01T00:00:00Z",
          deals: [
            { id: "d1", workspace_id: "w1", name: "Won deal", pipeline_id: "p1", stage_id: "s1", status: "won", amount_minor: 100_000, currency: "EUR", source: "manual", captured_by: "human:u1", created_at: "", updated_at: "" },
          ],
        }}
      />,
    );
    expect(screen.getByText(/owner/i)).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText(/€1,000|€1.000/)).toBeInTheDocument();
  });
});
