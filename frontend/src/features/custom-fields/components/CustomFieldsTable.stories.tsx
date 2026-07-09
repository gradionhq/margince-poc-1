import type { Meta, StoryObj } from "@storybook/react-vite";
import type {
  CustomField,
  Member,
} from "../../../lib/api-client/generated/index.js";
import { CustomFieldsTable } from "./CustomFieldsTable.js";

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
    label: "Priority",
    slug: "priority",
    type: "text",
    status: "active",
    column_name: "cf_priority",
    created_by: "user-2",
    created_at: "2026-07-02T10:00:00Z",
    updated_at: "2026-07-02T10:00:00Z",
  },
  {
    id: "field-3",
    workspace_id: "ws-1",
    object: "deal",
    label: "Old Field",
    slug: "old_field",
    type: "text",
    status: "retired",
    column_name: "cf_old_field",
    created_by: "user-1",
    created_at: "2026-06-01T10:00:00Z",
    updated_at: "2026-07-05T14:30:00Z",
  },
];

const meta: Meta<typeof CustomFieldsTable> = {
  component: CustomFieldsTable,
  title: "CRM/CustomFields/CustomFieldsTable",
};
export default meta;
type Story = StoryObj<typeof CustomFieldsTable>;

// Admin sees the row-action menu and the actual "Added by" name (FieldGuard
// visible).
export const Default: Story = {
  args: {
    fields: FIELDS,
    members: MEMBERS,
    selectedObject: "deal",
    role: "admin",
  },
};

// Non-admin: row actions omitted entirely (not disabled) and "Added by" masked.
export const NonAdmin: Story = {
  args: {
    fields: FIELDS,
    members: MEMBERS,
    selectedObject: "deal",
    role: "rep",
  },
};

// No fields on the selected object and no staged row → EmptyState.
export const Empty: Story = {
  args: {
    fields: [],
    members: MEMBERS,
    selectedObject: "organization",
    role: "admin",
  },
};

// A field mid-create renders an optimistic "writing…" row above the real rows.
export const StagedRow: Story = {
  args: {
    fields: FIELDS.slice(0, 2),
    members: MEMBERS,
    selectedObject: "deal",
    role: "admin",
    stagedRow: { label: "New Field", type: "text" },
  },
};
