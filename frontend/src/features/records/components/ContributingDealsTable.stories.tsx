import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { QuotaAttainment } from "../api/quotas.js";
import { ContributingDealsTable } from "./ContributingDealsTable.js";

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

// Pre-seeded so useContributingDealDetails's internal useQueries never fires a real
// GET /deals/{id} — mirrors QuotaPage.stories.tsx's own makeClient() shape.
function makeClient(): QueryClient {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
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

function Demo({ attainment }: { attainment: QuotaAttainment }) {
  return (
    <QueryClientProvider client={makeClient()}>
      <ContributingDealsTable attainment={attainment} />
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/ContributingDealsTable",
};
export default meta;
type Story = StoryObj<typeof Demo>;

// AC-quota-5: three closed-won rows, "Closed-won" pill, footer counted total, exclusion caption.
export const Default: Story = {
  args: { attainment: FIXTURE_ATTAINMENT },
};
