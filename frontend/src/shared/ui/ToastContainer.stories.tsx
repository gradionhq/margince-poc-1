import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { ToastContainer } from "./ToastContainer.js";

type Toast = { id: string; message: string; variant?: string };

function Demo({ initial }: { initial: Toast[] }) {
  const [toasts, setToasts] = useState<Toast[]>(initial);
  return (
    <ToastContainer
      toasts={toasts}
      onDismiss={(id) => setToasts((t) => t.filter((x) => x.id !== id))}
    />
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "gw-ui/ToastContainer",
};
export default meta;
type Story = StoryObj<typeof Demo>;

export const Default: Story = {
  args: {
    initial: [
      { id: "1", message: "Saved successfully", variant: "success" },
      { id: "2", message: "Something went wrong", variant: "error" },
    ],
  },
};
