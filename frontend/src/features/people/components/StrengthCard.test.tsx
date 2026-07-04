import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/person.js", () => ({
  usePersonStrengthBreakdown: vi.fn(),
}));

import * as personApi from "../api/person.js";
import { StrengthCard } from "./StrengthCard.js";

const mockBreakdown = vi.mocked(personApi.usePersonStrengthBreakdown);

function renderCard(strength: Parameters<typeof StrengthCard>[0]["strength"]) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <StrengthCard personId="p1" strength={strength} />
    </QueryClientProvider>,
  );
}

describe("StrengthCard", () => {
  it("renders the honest no-signal state when strength is null (STATE-1)", () => {
    mockBreakdown.mockReturnValue({ data: undefined, isLoading: false } as never);
    renderCard(null);
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
    expect(screen.queryByText("Recency")).not.toBeInTheDocument();
  });

  it("renders score/100, the deterministic chip, Team-wide caption, and 3 factor tiles (AC-person-2/3)", () => {
    mockBreakdown.mockReturnValue({ data: undefined, isLoading: false } as never);
    renderCard({ score: 72, bucket: "strong", recency: 0.9, frequency: 0.6, reciprocity: 0.8 });
    expect(screen.getByText("72/100")).toBeInTheDocument();
    expect(screen.getByText(/computed · deterministic/i)).toBeInTheDocument();
    expect(screen.getByText(/team-wide/i)).toBeInTheDocument();
    expect(screen.getByText("Recency")).toBeInTheDocument();
    expect(screen.getByText(/30-day half-life/i)).toBeInTheDocument();
    expect(screen.getByText("Frequency")).toBeInTheDocument();
    expect(screen.getByText(/saturates at 20/i)).toBeInTheDocument();
    expect(screen.getByText("Reciprocity")).toBeInTheDocument();
  });

  it("expands the evidence box lazily on click and toggles the trigger label (AC-person-4)", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    mockBreakdown.mockReturnValue({
      data: {
        person_id: "p1",
        score: 72,
        recency: 0.9,
        frequency: 0.6,
        reciprocity: 0.8,
        contributing_activities: [
          { id: "a1", kind: "email", subject: "Intro", occurred_at: "2026-05-01T00:00:00Z" },
        ],
      },
      isLoading: false,
    } as never);
    renderCard({ score: 72, bucket: "strong", recency: 0.9, frequency: 0.6, reciprocity: 0.8 });

    expect(mockBreakdown).toHaveBeenCalledWith("p1", false);
    const trigger = screen.getByRole("button", {
      name: /show the activities behind this score/i,
    });
    await user.click(trigger);
    expect(mockBreakdown).toHaveBeenCalledWith("p1", true);
    expect(
      screen.getByText("Score = 100 × 0.9 × 0.6 × 0.8 = 72"),
    ).toBeInTheDocument();
    expect(screen.getByText("Intro")).toBeInTheDocument();
    expect(screen.getByText(/formula §4/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /hide the activities behind this score/i }),
    ).toBeInTheDocument();
  });
});
