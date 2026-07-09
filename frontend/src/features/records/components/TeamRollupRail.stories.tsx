import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Quota, QuotaAttainment } from "../api/quotas.js";
import { quotasKeys } from "../api/quotas.js";
import { TeamRollupRail } from "./TeamRollupRail.js";

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

const FIXTURE_ATTAINMENT: QuotaAttainment = {
  quota_id: FIXTURE_QUOTA.id,
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

// A sibling quota sharing FIXTURE_QUOTA's exact period, a different owner — the rail's own
// useTeamRollup hook keeps quotas sharing the current quota's period_start/period_end.
const FIXTURE_SIBLING_QUOTA: Quota = {
  ...FIXTURE_QUOTA,
  id: "quota-tomas-q3",
  owner_id: "u-tomas",
  target_minor: 27000000,
  version: 1,
};

const FIXTURE_SIBLING_ATTAINMENT: QuotaAttainment = {
  ...FIXTURE_ATTAINMENT,
  quota_id: FIXTURE_SIBLING_QUOTA.id,
  closed_won_minor: 21060000,
  attainment_pct: 78,
  band: "accent",
};

// Pre-seeded so useTeamRollup's internal listQuotas + per-sibling attainment useQueries never fire
// a real network call — mirrors QuotaPage.stories.tsx's own makeClient() shape.
function makeClient(): QueryClient {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  qc.setQueryData(quotasKeys.list(), [FIXTURE_QUOTA, FIXTURE_SIBLING_QUOTA]);
  qc.setQueryData(
    quotasKeys.attainment(FIXTURE_SIBLING_QUOTA.id),
    FIXTURE_SIBLING_ATTAINMENT,
  );
  qc.setQueryData(["members"], {
    data: [
      { user_id: "u-riya", display_name: "Riya Patel" },
      { user_id: "u-tomas", display_name: "Tomás Vidal" },
    ],
  });
  return qc;
}

function Demo({
  quota,
  currentAttainment,
}: {
  quota: Quota;
  currentAttainment: QuotaAttainment;
}) {
  return (
    <QueryClientProvider client={makeClient()}>
      <TeamRollupRail quota={quota} currentAttainment={currentAttainment} />
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/TeamRollupRail",
};
export default meta;
type Story = StoryObj<typeof Demo>;

// AC-quota-8: the current rep's row plus one sibling sharing the same period, each with a
// mini-bar/percent, and the "team roll-up = Σ closed-won ÷ Σ targets · auditable" method caption.
export const Default: Story = {
  args: { quota: FIXTURE_QUOTA, currentAttainment: FIXTURE_ATTAINMENT },
};
