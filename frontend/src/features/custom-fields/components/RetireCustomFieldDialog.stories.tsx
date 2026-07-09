import type { Meta, StoryObj } from "@storybook/react-vite";
import { RetireCustomFieldDialog } from "./RetireCustomFieldDialog.js";

const meta: Meta<typeof RetireCustomFieldDialog> = {
  component: RetireCustomFieldDialog,
  title: "CRM/CustomFields/RetireCustomFieldDialog",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof RetireCustomFieldDialog>;

export const Default: Story = {
  args: {
    open: true,
    fieldLabel: "Renewal Date",
    objectDisplayName: "Deal",
    onConfirm: () => {},
    onCancel: () => {},
  },
};

export const Retiring: Story = {
  args: {
    open: true,
    fieldLabel: "Renewal Date",
    objectDisplayName: "Deal",
    onConfirm: () => {},
    onCancel: () => {},
    isLoading: true,
  },
};
