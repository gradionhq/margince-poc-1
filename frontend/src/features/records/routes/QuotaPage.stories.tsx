import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { setAuth } from "../../identity/store/authStore.js";
import { QuotaPage } from "./QuotaPage.js";

const QUOTA_ID = "q-story";

const QUOTA = {
  id: QUOTA_ID,
  workspace_id: "ws-storybook",
  owner_id: "u-story",
  team_id: null,
  period_start: "2026-07-01",
  period_end: "2026-09-30",
  target_minor: 28000000,
  currency: "EUR",
  version: 3,
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  archived_at: null,
};

const ATTAINMENT = {
  quota_id: QUOTA_ID,
  closed_won_minor: 31387200,
  target_minor: 28000000,
  currency: "EUR",
  attainment_pct: 112.1,
  gap_minor: 3387200,
  pace_pct: 92.4,
  band: "met" as const,
  as_of_date: "2026-07-09",
  contributing_deals: [
    { deal_id: "d1", base_value_minor: 17707200 },
    { deal_id: "d2", base_value_minor: 13680000 },
  ],
};

function makeClient() {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  qc.setQueryData(["quotas", "detail", QUOTA_ID], QUOTA);
  qc.setQueryData(["quotas", "attainment", QUOTA_ID], ATTAINMENT);
  qc.setQueryData(["members"], {
    data: [{ user_id: "u-story", display_name: "Riya Mehta" }],
  });
  qc.setQueryData(["deals", "detail", "d1"], {
    id: "d1",
    name: "BÄR Pharma — Packaging QA",
    closed_at: "2026-08-14T00:00:00Z",
  });
  qc.setQueryData(["deals", "detail", "d2"], {
    id: "d2",
    name: "Nordlicht GmbH — Renewal",
    closed_at: "2026-08-20T00:00:00Z",
  });
  return qc;
}

function Demo() {
  setAuth(
    {
      id: "user-demo",
      workspace_id: "ws-storybook",
      email: "demo@acme.com",
      display_name: "Demo User",
      timezone: "UTC",
      status: "active",
      is_agent: false,
    },
    "admin",
    ["admin"],
  );

  const client = makeClient();
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

export const Default: Story = {
  args: {},
};
