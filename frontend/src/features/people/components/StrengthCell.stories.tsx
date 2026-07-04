import type { Meta, StoryObj } from "@storybook/react-vite";
import { StrengthCell } from "./StrengthCell.js";

const meta: Meta<typeof StrengthCell> = {
  component: StrengthCell,
  title: "CRM/StrengthCell",
};
export default meta;
type Story = StoryObj<typeof StrengthCell>;

export const Strong: Story = {
  args: {
    score: 82,
    bucket: "strong",
    recency: 0.9,
    frequency: 0.7,
    reciprocity: 0.85,
  },
};

export const Moderate: Story = {
  args: {
    score: 45,
    bucket: "moderate",
    recency: 0.5,
    frequency: 0.4,
    reciprocity: 0.5,
  },
};

export const Weak: Story = {
  args: {
    score: 18,
    bucket: "weak",
    recency: 0.2,
    frequency: 0.1,
    reciprocity: 0.15,
  },
};

export const NoSignal: Story = {
  args: { noSignalYet: true },
};
