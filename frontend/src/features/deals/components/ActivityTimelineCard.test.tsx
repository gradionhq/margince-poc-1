import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActivityTimelineCard } from "./ActivityTimelineCard.js";

describe("ActivityTimelineCard", () => {
  it("lists activities with timestamp + source_system provenance", () => {
    render(
      <ActivityTimelineCard
        activities={
          [
            {
              id: "a1",
              kind: "email",
              subject: "Intro call follow-up",
              occurred_at: "2026-01-05T10:00:00Z",
              source_system: "gmail",
            },
          ] as never[]
        }
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Intro call follow-up")).toBeInTheDocument();
    expect(screen.getByText(/via gmail/)).toBeInTheDocument();
  });

  it("honest-empty footer when there are no logged activities", () => {
    render(
      <ActivityTimelineCard
        activities={[]}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("You logged none of this")).toBeInTheDocument();
  });

  it("shows a loading skeleton, then an error state with retry-worthy message", () => {
    const { rerender } = render(
      <ActivityTimelineCard activities={[]} isLoading={true} isError={false} />,
    );
    expect(
      screen.getByTestId("activity-timeline-skeleton"),
    ).toBeInTheDocument();
    rerender(
      <ActivityTimelineCard activities={[]} isLoading={false} isError={true} />,
    );
    expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
  });
});
