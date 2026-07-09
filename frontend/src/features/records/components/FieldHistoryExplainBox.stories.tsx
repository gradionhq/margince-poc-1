import type { Meta, StoryObj } from "@storybook/react-vite";
import { FieldHistoryExplainBox } from "./FieldHistoryExplainBox.js";

const meta: Meta<typeof FieldHistoryExplainBox> = {
  component: FieldHistoryExplainBox,
  title: "CRM/Records/FieldHistoryExplainBox",
  parameters: { surface: "centered" },
};
export default meta;
type Story = StoryObj<typeof FieldHistoryExplainBox>;

// The open/closed toggle is interactive local state, not separately
// story-able as distinct args — one story showing the default closed state
// (the "Explain this number" trigger + provenance chip) is representative.
export const Default: Story = {
  args: { grossMinor: 17707200, currency: "EUR" },
};
