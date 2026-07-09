import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Quota, QuotaAttainment } from "../api/quotas.js";

vi.mock("../api/quotas.js", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/quotas.js")>();
  return { ...actual, useTeamRollup: vi.fn() };
});
vi.mock("../../custom-fields/api/members.js", () => ({
  useMembers: () => ({
    data: {
      data: [
        { user_id: "u1", display_name: "Riya Patel" },
        { user_id: "u2", display_name: "Tomás Vidal" },
      ],
    },
  }),
}));

import { useTeamRollup } from "../api/quotas.js";
import { TeamRollupRail } from "./TeamRollupRail.js";

const QUOTA: Quota = {
  id: "q1",
  workspace_id: "ws1",
  owner_id: "u1",
  team_id: null,
  period_start: "2026-07-01",
  period_end: "2026-09-30",
  target_minor: 28000000,
  currency: "EUR",
  version: 3,
  created_at: "2026-06-28T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  archived_at: null,
};
const ATTAINMENT: QuotaAttainment = {
  quota_id: "q1",
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 113,
  gap_minor: 3387200,
  pace_pct: 64,
  band: "met",
  as_of_date: "2026-08-20",
  contributing_deals: [],
};

describe("TeamRollupRail", () => {
  it("AC-quota-8: lists each rep's attainment percent with a mini-bar and target, labels the method", () => {
    (useTeamRollup as ReturnType<typeof vi.fn>).mockReturnValue({
      reps: [
        { quota: QUOTA, attainment: ATTAINMENT, isCurrent: true },
        {
          quota: { ...QUOTA, id: "q2", owner_id: "u2", target_minor: 27000000 },
          attainment: { ...ATTAINMENT, attainment_pct: 78 },
          isCurrent: false,
        },
      ],
      isLoading: false,
      isError: false,
    });
    render(<TeamRollupRail quota={QUOTA} currentAttainment={ATTAINMENT} />);
    expect(screen.getByText("113%")).toBeInTheDocument();
    expect(screen.getByText("78%")).toBeInTheDocument();
    expect(
      screen.getByText(/team roll-up = Σ closed-won ÷ Σ targets · auditable/),
    ).toBeInTheDocument();
  });
});
