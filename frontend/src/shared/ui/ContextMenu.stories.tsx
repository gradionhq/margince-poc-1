import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";
import { ContextMenu } from "./ContextMenu.js";

function Demo() {
  return (
    <ContextMenu
      trigger={
        <button
          type="button"
          className="rounded-md bg-gf-elevated px-gf-md py-gf-sm text-gf-primary"
        >
          Open menu
        </button>
      }
      items={[
        { id: "edit", label: "Edit", onSelect: () => {} },
        { id: "archive", label: "Archive", onSelect: () => {} },
        { id: "delete", label: "Delete", onSelect: () => {} },
      ]}
    />
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "gw-ui/ContextMenu",
};
export default meta;
type Story = StoryObj<typeof Demo>;

export const Default: Story = {};

// Opens the popover so the menu panel — the component's actual surface — is on
// the catalog. play() because the menu only exists after a real click and
// renders through a portal; neither is expressible without the browser.
export const Open: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", { name: "Open menu" }),
    );
    await expect(await screen.findByRole("menu")).toBeVisible();
  },
};
