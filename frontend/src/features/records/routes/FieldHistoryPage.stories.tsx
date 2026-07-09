import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { apiClient } from "../../../lib/api-client/client.js";
import { setAuth } from "../../identity/store/authStore.js";
import { fieldHistoryKeys } from "../api/fieldHistory.js";
import { FieldHistoryPage } from "./FieldHistoryPage.js";

const ENTITY_TYPE = "deal";
const ENTITY_ID = "deal-baer-pharma";

const DEAL = {
  id: ENTITY_ID,
  workspace_id: "ws-storybook",
  name: "BÄR Pharma — Packaging QA",
  amount_minor: 17707200,
  currency: "EUR",
  stage_id: "s-proposal",
  owner_id: "u-anna",
  version: 6,
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-05-29T16:08:00Z",
  updated_at: "2026-06-18T09:42:00Z",
};

const ENTRIES = [
  {
    id: "e1",
    entity_type: ENTITY_TYPE,
    entity_id: ENTITY_ID,
    field: "amount_minor",
    old_value: "21200000",
    new_value: "17707200",
    changed_at: "2026-06-18T09:42:00Z",
    actor_type: "agent",
    actor_id: "a1",
    passport_id: "psp_7Q3f",
    evidence: {
      quote:
        "offer.accepted · offer_id=of_8842 · gross_minor=17707200 · currency=EUR",
      source_url: "https://example.com/offer/8842",
      confidence: "high",
      confidence_note: "computed, not inferred",
    },
  },
  {
    id: "e2",
    entity_type: ENTITY_TYPE,
    entity_id: ENTITY_ID,
    field: "amount_minor",
    old_value: null,
    new_value: "21200000",
    changed_at: "2026-05-29T16:08:00Z",
    actor_type: "human",
    actor_id: "u-anna",
    passport_id: null,
    evidence: null,
  },
  {
    id: "e3",
    entity_type: ENTITY_TYPE,
    entity_id: ENTITY_ID,
    field: "stage_id",
    old_value: "Qualified",
    new_value: "Proposal sent",
    changed_at: "2026-06-12T11:20:00Z",
    actor_type: "human",
    actor_id: "u-anna",
    passport_id: null,
    evidence: null,
  },
  {
    id: "e4",
    entity_type: ENTITY_TYPE,
    entity_id: ENTITY_ID,
    field: "stage_id",
    old_value: null,
    new_value: "Discovery",
    changed_at: "2026-05-29T16:09:00Z",
    actor_type: "human",
    actor_id: "u-anna",
    passport_id: null,
    evidence: null,
  },
];

function makeClient(
  opts: { entries?: typeof ENTRIES; deal?: typeof DEAL } = {},
) {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  qc.setQueryData(
    fieldHistoryKeys.list(ENTITY_TYPE, ENTITY_ID),
    opts.entries ?? ENTRIES,
  );
  qc.setQueryData(
    fieldHistoryKeys.entity(ENTITY_TYPE, ENTITY_ID),
    opts.deal ?? DEAL,
  );
  return qc;
}

function Demo({ client }: { client: QueryClient }) {
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
  return (
    <QueryClientProvider client={client}>
      <MemoryRouter
        initialEntries={[`/records/${ENTITY_TYPE}/${ENTITY_ID}/field-history`]}
      >
        <Routes>
          <Route
            path="/records/:entityType/:entityId/field-history"
            element={<FieldHistoryPage />}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/FieldHistoryPage",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof Demo>;

export const AllFields: Story = { args: { client: makeClient() } };

export const HonestEmptyField: Story = {
  args: {
    client: makeClient({
      entries: ENTRIES.filter((e) => e.field !== "stage_id"),
    }),
  },
};

// Storybook doesn't mock apiClient the way *.test.tsx does via vi.mock, so a
// pending query with no seeded QueryClient data still calls the real
// apiClient.GET, which fetch()es and 404s against the static Storybook
// server (2 console errors, failing the fe-uat render check).
//
// Registering a stub queryFn via qc.setQueryDefaults(key, { queryFn }) does
// NOT work here: TanStack Query's defaultQueryOptions merges call-site
// options over registered query defaults (`{ ...defaults, ...options }`),
// so useFieldHistory/useEntityRecord's own explicit, in-source queryFn
// always wins regardless of what's registered on the client.
//
// Instead, stub apiClient.GET itself so both queries genuinely stay pending
// without ever hitting the network. The stub is installed synchronously
// during render, not inside a useEffect here: FieldHistoryPage's own
// query-triggering effects are deeper in the tree and — because React
// fires effects child-to-parent on mount — would already run (and issue
// the real fetch) before an effect placed in this component fires. Doing
// it during render guarantees the stub is in place before any descendant
// mounts. The real implementation is restored on unmount so no other story
// in this Storybook session is affected.
function LoadingDemo({ client }: { client: QueryClient }) {
  const originalGetRef = useRef(apiClient.GET);
  apiClient.GET = (() => new Promise(() => {})) as typeof apiClient.GET;
  useEffect(() => {
    return () => {
      apiClient.GET = originalGetRef.current;
    };
  }, []);
  return <Demo client={client} />;
}

export const Loading: Story = {
  args: {
    client: new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  },
  render: (args) => <LoadingDemo client={args.client} />,
};
