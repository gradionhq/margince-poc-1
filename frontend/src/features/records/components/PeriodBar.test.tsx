import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Quota } from "../api/quotas.js";
import { PeriodBar } from "./PeriodBar.js";

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

describe("PeriodBar", () => {
  it("AC-quota-7: renders the current quota's own quarter as the only active chip", () => {
    render(<PeriodBar quota={QUOTA} onToast={vi.fn()} />);
    expect(screen.getByText(/Q3 2026/)).toBeInTheDocument();
  });

  it("AC-quota-7: clicking the prior-quarter chip toasts read-only/closed", async () => {
    const onToast = vi.fn();
    render(<PeriodBar quota={QUOTA} onToast={onToast} />);
    await userEvent.click(screen.getByText(/Q2 2026/));
    expect(onToast).toHaveBeenCalledWith(expect.stringMatching(/closed/i));
  });

  it("AC-quota-7: clicking the next-quarter chip toasts not-yet-set", async () => {
    const onToast = vi.fn();
    render(<PeriodBar quota={QUOTA} onToast={onToast} />);
    await userEvent.click(screen.getByText(/Q4 2026/));
    expect(onToast).toHaveBeenCalledWith(expect.stringMatching(/not yet set/i));
  });
});
