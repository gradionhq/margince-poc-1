import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { Deal, Partner } from "../../../lib/api-client/generated/index.js";
import { PartnerPanel } from "./PartnerPanel.js";

const partner: Partner = {
  id: "pt1",
  workspace_id: "w1",
  organization_id: "org1",
  cert_status: "certified",
  partner_role: "consulting",
  certified_staff: 4,
  source: "manual",
  captured_by: "human:u1",
  created_at: "",
  updated_at: "",
};

describe("PartnerPanel", () => {
  it("renders cert_status/partner_role/margin_tier + deals it sourced when present", () => {
    const deals: Deal[] = [
      {
        id: "d1",
        workspace_id: "w1",
        name: "Sourced deal",
        pipeline_id: "p1",
        stage_id: "s1",
        status: "open",
        partner_org_id: "org1",
        source: "manual",
        captured_by: "human:u1",
        created_at: "",
        updated_at: "",
      },
    ];
    render(<PartnerPanel partner={partner} sourcedDeals={deals} />);
    expect(screen.getByText(/certified/i)).toBeInTheDocument();
    expect(screen.getByText(/consulting/i)).toBeInTheDocument();
    expect(screen.getByText("Sourced deal")).toBeInTheDocument();
    expect(
      screen.getByText(/200 most recent deals workspace-wide/i),
    ).toBeInTheDocument();
  });

  it("is entirely absent when partner is null (STATE-1, not an error card)", () => {
    const { container } = render(
      <PartnerPanel partner={null} sourcedDeals={[]} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing while partner is still loading (undefined), avoiding a flash", () => {
    const { container } = render(
      <PartnerPanel partner={undefined} sourcedDeals={[]} />,
    );
    expect(container).toBeEmptyDOMElement();
  });
});
