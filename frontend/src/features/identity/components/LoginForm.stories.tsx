import type { Meta, StoryObj } from "@storybook/react-vite";
import { LoginForm } from "./LoginForm.js";

const meta: Meta<typeof LoginForm> = {
  component: LoginForm,
  title: "CRM/LoginForm",
};
export default meta;
type Story = StoryObj<typeof LoginForm>;

export const Default: Story = { args: { onSubmit: () => {} } };
export const Submitting: Story = {
  args: { onSubmit: () => {}, isSubmitting: true },
};
export const ValidationError: Story = {
  args: { onSubmit: () => {}, error: "Invalid credentials" },
};
