import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { OrgStrengthCard } from "./OrgStrengthCard.js";

function renderCard(props: React.ComponentProps<typeof OrgStrengthCard>) {
  return render(
    <MemoryRouter>
      <OrgStrengthCard {...props} />
    </MemoryRouter>,
  );
}

describe("OrgStrengthCard", () => {
  it("shows the plain max score, the MAX chip, and the strongest-contact source line", () => {
    renderCard({
      orgStrength: {
        score: 78,
        bucket: "strong",
        top_person_id: "p1",
        top_person_name: "Jordan Ellis",
      },
      contacts: [
        {
          id: "p1",
          data: {
            id: "p1",
            full_name: "Jordan Ellis",
            strength: {
              score: 78,
              bucket: "strong",
              recency: 0,
              frequency: 0,
              reciprocity: 0,
            },
          } as never,
          isLoading: false,
          isError: false,
        },
      ],
    });
    expect(screen.getByText("78/100")).toBeInTheDocument();
    expect(
      screen.getByText(/computed.*MAX over contacts/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/not an average, not a black box/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /jordan ellis/i })).toHaveAttribute(
      "href",
      "/people/p1",
    );
    // The AC's literal "capped/normalized...recency decay" copy is named drift — must not appear.
    expect(
      screen.queryByText(/capped|normalized|recency decay/i),
    ).not.toBeInTheDocument();
  });

  it("expands per-contact scores on click, toggles label, no cap/normalize step", () => {
    renderCard({
      orgStrength: {
        score: 78,
        bucket: "strong",
        top_person_id: "p1",
        top_person_name: "Jordan Ellis",
      },
      contacts: [
        {
          id: "p1",
          data: {
            id: "p1",
            full_name: "Jordan Ellis",
            strength: {
              score: 78,
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
            strength: {
              score: 40,
              bucket: "moderate",
              recency: 0,
              frequency: 0,
              reciprocity: 0,
            },
          } as never,
          isLoading: false,
          isError: false,
        },
      ],
    });
    const toggle = screen.getByRole("button", {
      name: /show the per-contact scores/i,
    });
    toggle.click();
    expect(screen.getByText("Jordan Ellis")).toBeInTheDocument();
    expect(screen.getByText("40/100")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /hide the per-contact scores/i }),
    ).toBeInTheDocument();
  });

  it("renders honest no-signal STATE-1 when org_strength is null", () => {
    renderCard({ orgStrength: null, contacts: [] });
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
    expect(screen.queryByText(/computed.*MAX/i)).not.toBeInTheDocument();
  });
});
