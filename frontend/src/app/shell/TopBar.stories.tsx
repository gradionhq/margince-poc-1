import type { Meta, StoryObj } from "@storybook/react-vite";
import { TopBar } from "./TopBar.js";

const meta: Meta<typeof TopBar> = {
  component: TopBar,
  title: "shell/TopBar",
  parameters: { surface: "padded" },
};
export default meta;
type Story = StoryObj<typeof TopBar>;

export const WithTitle: Story = { args: { title: "Contacts" } };
export const WithActions: Story = {
  args: {
    title: "Deals",
    actions: [{ id: "new", render: () => <button type="button">New deal</button> }],
  },
};
export const NoTitle: Story = { args: {} };
