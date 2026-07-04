import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OrgStrengthCell } from "./OrgStrengthCell.js";

describe("OrgStrengthCell", () => {
  it("renders an honest no-signal state when org_strength is null", () => {
    render(<OrgStrengthCell strength={null} contactCount={3} />);
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
  });

  it("renders the score, bucket color, and 'max over {N} contacts' caption", () => {
    render(
      <OrgStrengthCell
        strength={{
          score: 81,
          bucket: "strong",
          top_person_id: "p1",
          top_person_name: "Dana Buyer",
        }}
        contactCount={4}
      />,
    );
    expect(screen.getByText("81")).toBeInTheDocument();
    expect(screen.getByText(/max over 4 contacts/)).toBeInTheDocument();
  });

  it("hover popover names the top contact and the max-not-average assertion", () => {
    render(
      <OrgStrengthCell
        strength={{
          score: 81,
          bucket: "strong",
          top_person_id: "p1",
          top_person_name: "Dana Buyer",
        }}
        contactCount={4}
      />,
    );
    fireEvent.mouseEnter(screen.getByTestId("org-strength-cell"));
    expect(screen.getByText(/Dana Buyer/)).toBeInTheDocument();
    expect(
      screen.getByText(
        /the org score is the max over its contacts — not an average that hides one warm champion\./,
      ),
    ).toBeInTheDocument();
  });
});
