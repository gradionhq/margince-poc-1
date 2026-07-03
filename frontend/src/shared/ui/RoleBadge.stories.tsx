import type { Meta, StoryObj } from "@storybook/react-vite";
import { RoleBadge } from "./RoleBadge.js";

const meta: Meta<typeof RoleBadge> = {
  component: RoleBadge,
  title: "CRM/RoleBadge",
};
export default meta;
type Story = StoryObj<typeof RoleBadge>;

export const Admin: Story = { args: { role: "admin" } };
export const Manager: Story = { args: { role: "manager" } };
export const Rep: Story = { args: { role: "rep" } };
export const ReadOnly: Story = { args: { role: "read_only" } };
export const Ops: Story = { args: { role: "ops" } };
