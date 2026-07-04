import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import type {
  Organization,
  Relationship,
} from "../../../lib/api-client/generated/index.js";
import { PeopleRail } from "./PeopleRail.js";

const org: Organization = {
  id: "org1",
  workspace_id: "w1",
  display_name: "Acme",
  source: "manual",
  captured_by: "human:u1",
  created_at: "",
  updated_at: "",
  deals: [
    {
      id: "d1",
      workspace_id: "w1",
      name: "Deal",
      pipeline_id: "p1",
      stage_id: "s1",
      status: "open",
      source: "manual",
      captured_by: "human:u1",
      created_at: "",
      updated_at: "",
    },
  ],
  relationships: [
    {
      id: "r1",
      workspace_id: "w1",
      kind: "deal_stakeholder",
      person_id: "p1",
      deal_id: "d1",
      role: "champion",
      is_current_primary: false,
      source: "manual",
      captured_by: "human:u1",
      created_at: "",
      updated_at: "",
    } as Relationship,
  ],
};

describe("PeopleRail", () => {
  it("shows avatar/name/title/score, flags the champion, links to /people/:id", () => {
    render(
      <MemoryRouter>
        <PeopleRail
          org={org}
          contacts={[
            {
              id: "p1",
              data: {
                id: "p1",
                full_name: "Jordan Ellis",
                title: "VP Sales",
                strength: {
                  score: 91,
                  bucket: "strong",
                  recency: 0,
                  frequency: 0,
                  reciprocity: 0,
                },
              } as never,
              isLoading: false,
              isError: false,
            },
            {
              id: "p2",
              data: {
                id: "p2",
                full_name: "Sam Lowe",
                title: null,
                strength: null,
              } as never,
              isLoading: false,
              isError: false,
            },
          ]}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText("Jordan Ellis")).toBeInTheDocument();
    expect(screen.getByText("VP Sales")).toBeInTheDocument();
    expect(screen.getByText("91/100")).toBeInTheDocument();
    expect(screen.getByText("Champion")).toBeInTheDocument();
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /jordan ellis/i })).toHaveAttribute(
      "href",
      "/people/p1",
    );
  });

  it("isolates a single contact's fetch failure without blanking the rail (STATE-3)", () => {
    render(
      <MemoryRouter>
        <PeopleRail
          org={org}
          contacts={[
            { id: "p1", data: undefined, isLoading: false, isError: true },
            {
              id: "p2",
              data: { id: "p2", full_name: "Sam Lowe" } as never,
              isLoading: false,
              isError: false,
            },
          ]}
        />
      </MemoryRouter>,
    );
    expect(screen.getByText(/couldn.t load this contact/i)).toBeInTheDocument();
    expect(screen.getByText("Sam Lowe")).toBeInTheDocument();
  });
});
