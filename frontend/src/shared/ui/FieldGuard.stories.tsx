import type { Meta, StoryObj } from "@storybook/react-vite";
import { FieldGuard } from "./FieldGuard.js";

const meta: Meta<typeof FieldGuard> = {
  component: FieldGuard,
  title: "CRM/FieldGuard",
};
export default meta;
type Story = StoryObj<typeof FieldGuard>;

export const Visible: Story = {
  args: { mode: "visible", children: "alice@example.com" },
};
export const Masked: Story = {
  args: { mode: "masked", children: "alice@example.com" },
};
export const ReadOnly: Story = {
  args: { mode: "readonly", children: "alice@example.com" },
};
