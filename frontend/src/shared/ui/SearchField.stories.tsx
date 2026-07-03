import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { SearchField } from "./SearchField.js";

function Controlled({ initial = "" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <SearchField
      value={value}
      onChange={setValue}
      placeholder="Search or ask…"
      onClear={() => setValue("")}
    />
  );
}

const meta: Meta<typeof Controlled> = {
  component: Controlled,
  title: "gw-ui/SearchField",
  // SearchField is fluid (fills its parent, as it does in the top bar); the
  // story pins a realistic field width so it doesn't stretch edge-to-edge.
  parameters: { surface: "centered", design: { node: "UPQPU" } }, // Search Bar
  decorators: [
    (Story) => (
      <div className="w-80">
        <Story />
      </div>
    ),
  ],
};
export default meta;
type Story = StoryObj<typeof Controlled>;

export const Empty: Story = {};
export const Filled: Story = { args: { initial: "Alice" } };
