import type { Meta, StoryObj } from "@storybook/react-vite";
import type { QuotaAttainment } from "../api/quotas.js";
import { AttainmentRing } from "./AttainmentRing.js";

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

const meta: Meta<typeof AttainmentRing> = {
  component: AttainmentRing,
  title: "CRM/Records/AttainmentRing",
};
export default meta;
type Story = StoryObj<typeof AttainmentRing>;

// AC-quota-1..3: the happy-path ring — attained %, closed-won/target/gap stat rows, pace line.
export const Attained: Story = {
  args: {
    attainment: FIXTURE_ATTAINMENT,
    isLoading: false,
    isError: false,
  },
};
