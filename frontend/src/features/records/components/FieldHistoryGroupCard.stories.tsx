import type { Meta, StoryObj } from "@storybook/react-vite";
import type { FieldHistoryEntry } from "../api/fieldHistory.js";
import type { FieldHistoryGroup } from "../hooks/useFieldHistoryView.js";
import { FieldHistoryGroupCard } from "./FieldHistoryGroupCard.js";

function entry(overrides: Partial<FieldHistoryEntry>): FieldHistoryEntry {
  return {
    id: "e1",
    entity_type: "deal",
    entity_id: "deal-baer-pharma",
    field: "stage_id",
    old_value: "Discovery",
    new_value: "Qualified",
    changed_at: "2026-06-12T11:20:00Z",
    actor_type: "human",
    actor_id: "u-anna",
    passport_id: null,
    evidence: null,
    ...overrides,
  };
}

const STAGE_GROUP: FieldHistoryGroup = {
  field: "stage_id",
  label: "Stage id",
  currentValue: "Proposal sent",
  allEntries: [
    entry({
      id: "e2",
      changed_at: "2026-06-12T11:20:00Z",
      old_value: "Qualified",
      new_value: "Proposal sent",
    }),
    entry({
      id: "e1",
      changed_at: "2026-05-29T16:09:00Z",
      old_value: null,
      new_value: "Discovery",
    }),
  ],
  visibleEntries: [
    entry({
      id: "e2",
      changed_at: "2026-06-12T11:20:00Z",
      old_value: "Qualified",
      new_value: "Proposal sent",
    }),
    entry({
      id: "e1",
      changed_at: "2026-05-29T16:09:00Z",
      old_value: null,
      new_value: "Discovery",
    }),
  ],
};

const OWNER_GROUP: FieldHistoryGroup = {
  field: "owner_id",
  label: "Owner id",
  currentValue: "u-anna",
  allEntries: [],
  visibleEntries: [],
};

const amountEntry = entry({
  id: "e-amount",
  field: "amount_minor",
  actor_type: "agent",
  actor_id: "a-passport",
  passport_id: "psp_7Q3f",
  old_value: "21200000",
  new_value: "17707200",
  changed_at: "2026-06-18T09:42:00Z",
  evidence: {
    quote:
      "offer.accepted · offer_id=of_8842 · gross_minor=17707200 · currency=EUR",
    source_url: "https://example.com/offer/8842",
    confidence: "high",
    confidence_note: "computed, not inferred",
  },
});

const AMOUNT_GROUP: FieldHistoryGroup = {
  field: "amount_minor",
  label: "Amount",
  currentValue: 17707200,
  allEntries: [amountEntry],
  visibleEntries: [amountEntry],
};

const meta: Meta<typeof FieldHistoryGroupCard> = {
  component: FieldHistoryGroupCard,
  title: "CRM/Records/FieldHistoryGroupCard",
  // Fluid card, fills its parent column — pin a realistic content width.
  decorators: [
    (Story) => (
      <div className="max-w-2xl">
        <Story />
      </div>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof FieldHistoryGroupCard>;

export const WithDiffTimeline: Story = {
  args: { group: STAGE_GROUP, currency: "EUR" },
};

export const HonestEmptyHistory: Story = {
  args: { group: OWNER_GROUP, currency: "EUR" },
};

export const ComputedMoneyFieldWithExplainBox: Story = {
  args: { group: AMOUNT_GROUP, currency: "EUR" },
};
