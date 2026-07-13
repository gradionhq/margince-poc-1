import type { Meta, StoryObj } from "@storybook/react-vite";
import { MemoryRouter } from "react-router-dom";
import { WorkspaceRail } from "./WorkspaceRail.js";

const meta: Meta<typeof WorkspaceRail> = {
  component: WorkspaceRail,
  title: "shell/WorkspaceRail",
  parameters: { surface: "fullscreen" },
  decorators: [
    (Story) => (
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof WorkspaceRail>;

export const Default: Story = {
  args: { activeId: "tasks", userName: "Ada Lovelace" },
};

export const WithBadgeCounts: Story = {
  args: {
    activeId: "tasks",
    counts: { tasks: 3, inbox: 0 },
    userName: "Ada Lovelace",
  },
};

export const InboxBadge: Story = {
  args: {
    activeId: "inbox",
    counts: { tasks: 0, inbox: 7 },
    userName: "Ada Lovelace",
  },
};

export const NoCounts: Story = {
  args: { activeId: "home", userName: "Ada Lovelace" },
};
