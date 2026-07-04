import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActivityCard } from "./ActivityCard.js";

describe("ActivityCard", () => {
  it("tags each row with a SourceChip and shows the fixed footer", () => {
    render(
      <ActivityCard
        activities={[{ id: "a1", kind: "email", subject: "Intro call", occurred_at: "2026-06-01T00:00:00Z" }]}
        source="email:gmail"
        capturedBy="connector:gmail"
      />,
    );
    expect(screen.getByText("Intro call")).toBeInTheDocument();
    expect(screen.getByText(/connector/i)).toBeInTheDocument();
    expect(
      screen.getByText(/you logged none of this — capture linked every item/i),
    ).toBeInTheDocument();
  });

  it("renders an honest empty state with no activities", () => {
    render(<ActivityCard activities={[]} source="manual" capturedBy="human:u1" />);
    expect(screen.getByText(/no activity yet/i)).toBeInTheDocument();
  });
});
