import type { Meta, StoryObj } from "@storybook/react-vite";
import { UserAvatar } from "./UserAvatar.js";

const meta: Meta<typeof UserAvatar> = {
  component: UserAvatar,
  title: "gw-ui/UserAvatar",
  parameters: { surface: "centered", design: { node: "V0eTi" } }, // Avatar Md
};
export default meta;
type Story = StoryObj<typeof UserAvatar>;

export const NoPresence: Story = { args: { name: "Alice" } };
export const Online: Story = { args: { name: "Alice", presence: "online" } };
export const Away: Story = { args: { name: "Bob", presence: "away" } };
export const Offline: Story = { args: { name: "Carol", presence: "offline" } };
