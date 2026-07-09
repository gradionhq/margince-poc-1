import type { Meta, StoryObj } from "@storybook/react-vite";
import type { Quota } from "../api/quotas.js";
import { PeriodBar } from "./PeriodBar.js";

const FIXTURE_QUOTA: Quota = {
  id: "quota-riya-q3",
  workspace_id: "ws-storybook",
  owner_id: "u-riya",
  team_id: null,
  period_start: "2026-07-01",
  period_end: "2026-09-30",
  target_minor: 28000000,
  currency: "EUR",
  version: 3,
  created_at: "2026-06-28T16:40:00Z",
  updated_at: "2026-07-01T09:12:00Z",
  archived_at: null,
};

const meta: Meta<typeof PeriodBar> = {
  component: PeriodBar,
  title: "CRM/Records/PeriodBar",
};
export default meta;
type Story = StoryObj<typeof PeriodBar>;

// AC-quota-7: only the current quarter chip is styled active; onToast is a no-op — a static
// capture can't observe a toast anyway, and the toast wording itself is PeriodBar.test.tsx's job.
export const CurrentQuarter: Story = {
  args: { quota: FIXTURE_QUOTA, onToast: () => {} },
};
