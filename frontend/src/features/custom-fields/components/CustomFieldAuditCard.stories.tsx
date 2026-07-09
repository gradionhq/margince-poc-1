import type { Meta, StoryObj } from "@storybook/react-vite";
import type {
  CustomField,
  Member,
} from "../../../lib/api-client/generated/index.js";
import { CustomFieldAuditCard } from "./CustomFieldAuditCard.js";

const MEMBERS: Member[] = [
  {
    user_id: "user-1",
    email: "alice@example.com",
    display_name: "Alice Smith",
    status: "active",
    is_agent: false,
    roles: ["admin"],
  },
  {
    user_id: "user-2",
    email: "bob@example.com",
    display_name: "Bob Johnson",
    status: "active",
    is_agent: false,
    roles: ["rep"],
  },
];

const FIELDS: CustomField[] = [
  {
    id: "field-1",
    workspace_id: "ws-1",
    object: "deal",
    label: "Renewal Date",
    slug: "renewal_date",
    type: "date",
    status: "active",
    column_name: "cf_renewal_date",
    created_by: "user-1",
    created_at: "2026-07-01T10:00:00Z",
    updated_at: "2026-07-01T10:00:00Z",
  },
  {
    id: "field-2",
    workspace_id: "ws-1",
    object: "deal",
    label: "Old Field",
    slug: "old_field",
    type: "text",
    status: "retired",
    column_name: "cf_old_field",
    created_by: "user-2",
    created_at: "2026-06-01T10:00:00Z",
    updated_at: "2026-07-05T14:30:00Z",
  },
];

const meta: Meta<typeof CustomFieldAuditCard> = {
  component: CustomFieldAuditCard,
  title: "CRM/CustomFields/CustomFieldAuditCard",
};
export default meta;
type Story = StoryObj<typeof CustomFieldAuditCard>;

// Admin sees the real actor name (FieldGuard visible) on both an "added" and
// a "retired" entry (a retired field derives both).
export const WithEntries: Story = {
  args: { fields: FIELDS, members: MEMBERS, role: "admin", isLoading: false, isError: false },
};

// Non-admin: actor name masked via FieldGuard.
export const Masked: Story = {
  args: { fields: FIELDS, members: MEMBERS, role: "rep", isLoading: false, isError: false },
};

export const Empty: Story = {
  args: { fields: [], members: MEMBERS, role: "admin", isLoading: false, isError: false },
};

export const Loading: Story = {
  args: { fields: [], members: MEMBERS, role: "admin", isLoading: true, isError: false },
};

export const ErrorState: Story = {
  args: { fields: [], members: MEMBERS, role: "admin", isLoading: false, isError: true },
};
