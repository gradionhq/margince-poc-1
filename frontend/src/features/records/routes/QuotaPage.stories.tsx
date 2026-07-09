import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { setAuth } from "../../identity/store/authStore.js";
import type { Quota, QuotaAttainment } from "../api/quotas.js";
import { quotasKeys } from "../api/quotas.js";
import { QuotaPage } from "./QuotaPage.js";

const QUOTA_ID = "quota-riya-q3";

const FIXTURE_QUOTA: Quota = {
  id: QUOTA_ID,
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
  quota_id: QUOTA_ID,
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

function makeClient(): QueryClient {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  qc.setQueryData(quotasKeys.detail(QUOTA_ID), FIXTURE_QUOTA);
  qc.setQueryData(quotasKeys.attainment(QUOTA_ID), FIXTURE_ATTAINMENT);
  qc.setQueryData(quotasKeys.list(), [FIXTURE_QUOTA]);
  qc.setQueryData(["members"], {
    data: [{ user_id: "u-riya", display_name: "Riya Patel" }],
  });
  qc.setQueryData(["deals", "detail", "deal-baer"], {
    id: "deal-baer",
    name: "BÄR Pharma — Packaging QA",
    closed_at: "2026-08-14T00:00:00Z",
  });
  qc.setQueryData(["deals", "detail", "deal-brandt"], {
    id: "deal-brandt",
    name: "Brandt — Line QA Retrofit",
    closed_at: "2026-07-29T00:00:00Z",
  });
  qc.setQueryData(["deals", "detail", "deal-meyer"], {
    id: "deal-meyer",
    name: "Meyer Logistik — Audit Trail",
    closed_at: "2026-09-02T00:00:00Z",
  });
  return qc;
}

function Demo({ client }: { client: QueryClient }) {
  setAuth(
    {
      id: "u-riya",
      workspace_id: "ws-storybook",
      email: "riya@acme.com",
      display_name: "Riya Patel",
      timezone: "UTC",
      status: "active",
      is_agent: false,
    },
    "rep",
    ["rep"],
  );

  return (
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[`/quotas/${QUOTA_ID}`]}>
        <Routes>
          <Route path="/quotas/:id" element={<QuotaPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/QuotaPage",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof Demo>;

// AC-quota-1..8: the happy-path attainment state — ring, pace, explain box, contributing deals,
// target editor, period bar, team roll-up all rendered from one pre-seeded fixture set. Seeding
// quotasKeys.list() keeps TeamRollupRail's useTeamRollup hook fully mocked — no real network call
// ever fires against the static Storybook file server.
export const Attainment: Story = { args: { client: makeClient() } };
