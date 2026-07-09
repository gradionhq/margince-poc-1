import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { setAuth } from "../../identity/store/authStore.js";
import { recordsKeys } from "../api/records.js";
import { AccountHierarchyPage } from "./AccountHierarchyPage.js";

// Fixture data — Acme Group hierarchy used consistently across all stories.
const ROOT_ID = "org-acme-root";

const FIXTURE_ROOT: Organization = {
  id: ROOT_ID,
  workspace_id: "ws-storybook",
  display_name: "Acme Group",
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  parent_org_id: null,
  version: 3,
  domains: [
    {
      id: "d-root",
      organization_id: ROOT_ID,
      domain: "acme.com",
      is_primary: true,
      source: "test",
      captured_by: "human:demo",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      archived_at: null,
    },
  ],
};

const FIXTURE_CHILD_1: Organization = {
  id: "org-acme-defense",
  workspace_id: "ws-storybook",
  display_name: "Acme Defense Systems",
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  parent_org_id: ROOT_ID,
  version: 2,
  domains: [],
};

const FIXTURE_CHILD_2: Organization = {
  id: "org-acme-mobility",
  workspace_id: "ws-storybook",
  display_name: "Acme E-Mobility AG",
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  parent_org_id: ROOT_ID,
  version: 1,
  domains: [],
};

const FIXTURE_GRANDCHILD: Organization = {
  id: "org-acme-defense-eu",
  workspace_id: "ws-storybook",
  display_name: "Acme Defense EU",
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  parent_org_id: "org-acme-defense",
  version: 1,
  domains: [],
};

// An orphan sharing the root's domain — candidate for SuggestedEdge stories.
const FIXTURE_ORPHAN: Organization = {
  id: "org-acme-ventures",
  workspace_id: "ws-storybook",
  display_name: "Acme Ventures LLC",
  source: "test",
  captured_by: "human:demo",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  parent_org_id: null,
  version: 1,
  domains: [
    {
      id: "d-orphan",
      organization_id: "org-acme-ventures",
      domain: "acme.com",
      is_primary: true,
      source: "test",
      captured_by: "human:demo",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
      archived_at: null,
    },
  ],
};

const TREE_ROLLUP = {
  root_id: ROOT_ID,
  scope: "tree" as const,
  weighted_pipeline: { amount_minor: 38_500_00, currency: "EUR" },
  closed_won: { amount_minor: 12_000_00, currency: "EUR" },
  activity_count_30d: 14,
  aggregated_account_count: 4,
  restricted_excluded: [] as Array<{ id: string; display_name: string }>,
  computed_at: "2026-07-09T00:00:00Z",
};

const SELF_ROLLUP = {
  ...TREE_ROLLUP,
  scope: "self" as const,
  weighted_pipeline: { amount_minor: 10_000_00, currency: "EUR" },
  closed_won: { amount_minor: 5_000_00, currency: "EUR" },
  aggregated_account_count: 1,
};

const RESTRICTED_ROLLUP = {
  ...TREE_ROLLUP,
  restricted_excluded: [
    { id: "org-restricted", display_name: "Restricted Subsidiary" },
  ],
};

const ALL_ORGS = [
  FIXTURE_ROOT,
  FIXTURE_CHILD_1,
  FIXTURE_CHILD_2,
  FIXTURE_GRANDCHILD,
];

// A fresh QueryClient per story, pre-seeded with the exact query keys the page's hooks read.
// staleTime: Infinity prevents react-query from ever issuing a real network refetch.
function makeClient(opts: {
  orgs?: Organization[];
  treeRollup?: typeof TREE_ROLLUP;
  selfRollup?: typeof SELF_ROLLUP;
  rootOrg?: Organization;
}): QueryClient {
  const qc = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  qc.setQueryData(
    recordsKeys.rollup(ROOT_ID, "tree"),
    opts.treeRollup ?? TREE_ROLLUP,
  );
  qc.setQueryData(
    recordsKeys.rollup(ROOT_ID, "self"),
    opts.selfRollup ?? SELF_ROLLUP,
  );
  qc.setQueryData(recordsKeys.treeOrgs(), opts.orgs ?? ALL_ORGS);
  qc.setQueryData(
    ["organizations", "detail", ROOT_ID],
    opts.rootOrg ?? FIXTURE_ROOT,
  );
  return qc;
}

function Demo({
  client,
  initialAcceptedCandidateIds,
}: {
  client: QueryClient;
  // Forwarded to AccountHierarchyPage's Storybook-only seam — lets a story render
  // SuggestedEdgeCard's "accepted" branch without a real (unmocked) PATCH. See
  // SuggestedEdgeAccepted below.
  initialAcceptedCandidateIds?: ReadonlySet<string>;
}) {
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
      <MemoryRouter initialEntries={[`/companies/${ROOT_ID}/hierarchy`]}>
        <Routes>
          <Route
            path="/companies/:id/hierarchy"
            element={
              <AccountHierarchyPage
                initialAcceptedCandidateIds={initialAcceptedCandidateIds}
              />
            }
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/AccountHierarchyPage",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof Demo>;

// AC-1: tree scope shows tree-aggregated totals + "aggregated over N accounts" badge.
export const TreeScope: Story = {
  args: { client: makeClient({}) },
};

// AC-3: self scope shows this-account-only figures (click "This account only" to activate).
export const SelfScope: Story = {
  args: { client: makeClient({}) },
};

// AC-2: ExplainExpanded — the "Explain this roll-up" box is open (interacted via the button).
export const ExplainExpanded: Story = {
  args: { client: makeClient({}) },
};

// AC-5: RestrictedNode — restricted_excluded entries render as their own disclosed rows.
export const RestrictedNode: Story = {
  args: {
    client: makeClient({ treeRollup: RESTRICTED_ROLLUP }),
  },
};

// AC-6/7: SuggestedEdgeStaged — an orphan with matching domain renders the "Accept edge" card.
export const SuggestedEdgeStaged: Story = {
  args: {
    client: makeClient({ orgs: [...ALL_ORGS, FIXTURE_ORPHAN] }),
  },
};

// AC-6: SuggestedEdgeAccepted — the accepted state ("edge written · audited"). Rendered
// directly via AccountHierarchyPage's initialAcceptedCandidateIds seam rather than by
// clicking "Accept edge": clicking would fire a real PATCH against the Storybook dev
// server, which has no backend behind it and 404s — that's not a demonstration of the
// accepted UI state, it's an error toast. Per this file's own convention (one Story per
// state, render-only, no interaction test — docs/quality/testing.md's "visual states a
// human reviews" rung), seeding the state directly is the correct fix.
export const SuggestedEdgeAccepted: Story = {
  args: {
    client: makeClient({ orgs: [...ALL_ORGS, FIXTURE_ORPHAN] }),
    initialAcceptedCandidateIds: new Set([FIXTURE_ORPHAN.id]),
  },
};

// AC-7: SuggestedEdgeDismissed — after dismissal, the card is gone and no PATCH was sent.
export const SuggestedEdgeDismissed: Story = {
  args: {
    client: makeClient({ orgs: ALL_ORGS }),
  },
};

// STATE-1: Loading skeleton — query not yet resolved.
export const Loading: Story = {
  args: {
    client: new QueryClient({
      defaultOptions: { queries: { retry: false } },
    }),
  },
};

// STATE-2: Empty — root with no children and no restricted nodes.
export const Empty: Story = {
  args: {
    client: makeClient({
      orgs: [FIXTURE_ROOT],
      treeRollup: { ...TREE_ROLLUP, aggregated_account_count: 1 },
      selfRollup: { ...SELF_ROLLUP, aggregated_account_count: 1 },
    }),
  },
};

// STATE-3: RollupError — rollup query failure; tiles band shows honest error card.
export const RollupError: Story = {
  args: {
    client: (() => {
      const qc = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      qc.setQueryData(recordsKeys.treeOrgs(), [FIXTURE_ROOT]);
      qc.setQueryData(["organizations", "detail", ROOT_ID], FIXTURE_ROOT);
      // No rollup data — will show error state on initial render.
      return qc;
    })(),
  },
};

// STATE-4: NoPermission — intended to demonstrate a 403 from the rollup fetch rendering its
// own distinct "you don't have access" card (see RollupTilesBand's isForbidden branch and
// AccountHierarchyPage.test.tsx's STATE-4 unit test for the real, deterministic proof of this).
// This story can't force a real 403 status: Storybook doesn't mock apiClient (unlike
// *.test.tsx's vi.mock), so this renders whatever the unmocked fetch to a nonexistent backend
// actually resolves to — a real 403 must come from vi.mock'ing `response.status` in a jsdom test.
export const NoPermission: Story = {
  args: {
    client: (() => {
      const qc = new QueryClient({
        defaultOptions: { queries: { retry: false } },
      });
      qc.setQueryData(recordsKeys.treeOrgs(), [FIXTURE_ROOT]);
      qc.setQueryData(["organizations", "detail", ROOT_ID], FIXTURE_ROOT);
      return qc;
    })(),
  },
};

// AC-8: MoneyFormatting — confirms de-DE locale money values and the EUR caption.
export const MoneyFormatting: Story = {
  args: { client: makeClient({}) },
};
