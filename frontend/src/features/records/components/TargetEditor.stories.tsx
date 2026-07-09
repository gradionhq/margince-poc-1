import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Quota } from "../api/quotas.js";
import { TargetEditor } from "./TargetEditor.js";

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

function Demo({ quota }: { quota: Quota }) {
  // A fresh, unseeded QueryClient — TargetEditor calls useUpdateQuotaTarget internally, which
  // needs a provider to not throw at render time, but this story never clicks "Save target" (that
  // would attempt a real, unmocked PATCH against the static Storybook file server), so no
  // mutation-response seeding is needed. The PATCH wiring itself is TargetEditor.test.tsx's job.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={client}>
      <TargetEditor quota={quota} onToast={() => {}} />
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/Records/TargetEditor",
};
export default meta;
type Story = StoryObj<typeof Demo>;

// AC-quota-6: the target input pre-filled from the quota's own target, Save button, helper copy.
export const Default: Story = {
  args: { quota: FIXTURE_QUOTA },
};
