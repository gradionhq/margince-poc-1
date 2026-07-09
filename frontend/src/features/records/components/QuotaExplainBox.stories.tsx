import type { Meta, StoryObj } from "@storybook/react-vite";
import type { QuotaAttainment } from "../api/quotas.js";
import { QuotaExplainBox } from "./QuotaExplainBox.js";

const FIXTURE_ATTAINMENT: QuotaAttainment = {
  quota_id: "quota-riya-q3",
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112.1,
  gap_minor: 3387200,
  pace_pct: 64,
  band: "met",
  as_of_date: "2026-08-20",
  contributing_deals: [
    { deal_id: "deal-baer", base_value_minor: 17707200 },
    { deal_id: "deal-brandt", base_value_minor: 9450000 },
    { deal_id: "deal-meyer", base_value_minor: 4230000 },
  ],
};

const meta: Meta<typeof QuotaExplainBox> = {
  component: QuotaExplainBox,
  title: "CRM/Records/QuotaExplainBox",
};
export default meta;
type Story = StoryObj<typeof QuotaExplainBox>;

// AC-quota-4: the collapsed toggle plus the "computed server-side" provenance chip. Every exported
// story needs deterministic args — the toggled-open content is proven by
// QuotaExplainBox.test.tsx's own userEvent click, not duplicated here as a play function.
export const Default: Story = {
  args: { attainment: FIXTURE_ATTAINMENT },
};
