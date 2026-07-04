import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActivityTab } from "./ActivityTab.js";

describe("ActivityTab", () => {
  it("renders the honest empty state when there are no activities (STATE-1)", () => {
    render(<ActivityTab activities={[]} />);
    expect(screen.getByText(/no activity captured yet/i)).toBeInTheDocument();
  });

  it("renders each row's kind, subject, occurred_at, and the fixed caption (AC-person-7/8) — no fabricated per-row provenance chip", () => {
    render(
      <ActivityTab
        activities={[
          {
            id: "a1",
            kind: "email",
            subject: "Kickoff call",
            occurred_at: "2026-05-01T00:00:00Z",
          } as never,
        ]}
      />,
    );
    expect(screen.getByText(/email/i)).toBeInTheDocument();
    expect(screen.getByText("Kickoff call")).toBeInTheDocument();
    expect(screen.getByText(/2026-05-01/)).toBeInTheDocument();
    // ActivityRef (crm.d.ts) carries id/kind/subject/occurred_at only — no source/captured_by. A
    // real per-row SourceChip-style chip would have to be fabricated; this asserts it is NOT
    // rendered (the gap is flagged in a code comment + the PR description instead of faked).
    expect(screen.queryByText(/connector ·/i)).not.toBeInTheDocument();
    expect(
      screen.getByText(/you logged none of this — every row carries its source/i),
    ).toBeInTheDocument();
  });
});
