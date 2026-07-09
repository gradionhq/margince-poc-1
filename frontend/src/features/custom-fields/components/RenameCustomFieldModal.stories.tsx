import type { Meta, StoryObj } from "@storybook/react-vite";
import type { CustomField } from "../../../lib/api-client/generated/index.js";
import { RenameCustomFieldModal } from "./RenameCustomFieldModal.js";

const FIELD: CustomField = {
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
};

const meta: Meta<typeof RenameCustomFieldModal> = {
  component: RenameCustomFieldModal,
  title: "CRM/CustomFields/RenameCustomFieldModal",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof RenameCustomFieldModal>;

// Save starts disabled: the input is pre-filled with the field's current
// label, so an untouched form is the "unchanged" guard, not the empty guard.
export const Default: Story = {
  args: { open: true, field: FIELD, onClose: () => {}, onSave: () => {} },
};

export const Saving: Story = {
  args: {
    open: true,
    field: FIELD,
    onClose: () => {},
    onSave: () => {},
    isLoading: true,
  },
};
