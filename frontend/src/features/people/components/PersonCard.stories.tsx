import type { Meta, StoryObj } from "@storybook/react-vite";
import { PersonCard } from "./PersonCard.js";

const meta: Meta<typeof PersonCard> = {
  component: PersonCard,
  title: "CRM/PersonCard",
};
export default meta;
type Story = StoryObj<typeof PersonCard>;

export const Default: Story = {
  args: { name: "Alice Müller", email: "alice@example.com" },
};

export const NoEmail: Story = {
  args: { name: "Bob Schmidt" },
};

export const LongName: Story = {
  args: {
    name: "Christoph-Alexander von Hohenzollern-Sigmaringen",
    email: "c.hohenzollern@example.com",
  },
};
