import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { AccountSignalCard } from "./AccountSignalCard.js";

const baseOrg: Organization = {
  id: "org1",
  workspace_id: "w1",
  display_name: "Acme",
  source: "manual",
  captured_by: "human:u1",
  created_at: "",
  updated_at: "",
};

describe("AccountSignalCard", () => {
  it("cites the real stalled deal with an open-the-deal link when one exists", () => {
    render(
      <MemoryRouter>
        <AccountSignalCard
          org={{
            ...baseOrg,
            deals: [
              {
                id: "d1",
                workspace_id: "w1",
                name: "Renewal",
                pipeline_id: "p1",
                stage_id: "s1",
                status: "open",
                stalled: true,
                source: "manual",
                captured_by: "human:u1",
                created_at: "",
                updated_at: "",
              },
            ],
          }}
        />
      </MemoryRouter>,
    );
    expect(
      screen.getByRole("link", { name: /open the deal/i }),
    ).toHaveAttribute("href", "/deals/d1");
  });

  it("omits the stalled sub-line entirely when nothing is grounded (STATE-5)", () => {
    render(
      <MemoryRouter>
        <AccountSignalCard org={{ ...baseOrg, deals: [] }} />
      </MemoryRouter>,
    );
    expect(
      screen.queryByRole("link", { name: /open the deal/i }),
    ).not.toBeInTheDocument();
  });
});
