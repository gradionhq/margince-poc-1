import type { Meta, StoryObj } from "@storybook/react-vite";
import { userEvent, within } from "storybook/test";
import { NewCustomFieldModal } from "./NewCustomFieldModal.js";

const meta: Meta<typeof NewCustomFieldModal> = {
  component: NewCustomFieldModal,
  title: "CRM/CustomFields/NewCustomFieldModal",
  // fullscreen: the native <dialog> top-layer reads best without the padded
  // catalog gutter competing with the modal's own centered chrome.
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof NewCustomFieldModal>;

// Empty label — Confirm disabled, API key/DDL preview both blank.
export const Default: Story = {
  args: { open: true, object: "deal", onClose: () => {}, onConfirm: () => {} },
};

// Currency type surfaces the ISO-4217 code field and its minor-units caption.
export const CurrencyType: Story = {
  args: { open: true, object: "deal", onClose: () => {}, onConfirm: () => {} },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(canvas.getAllByRole("textbox")[0], "List price");
    await userEvent.selectOptions(canvas.getByRole("combobox"), "currency");
  },
};

// Picklist type surfaces the options editor (Add/Remove option rows).
export const PicklistType: Story = {
  args: {
    open: true,
    object: "organization",
    onClose: () => {},
    onConfirm: () => {},
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(canvas.getAllByRole("textbox")[0], "Tier");
    await userEvent.selectOptions(canvas.getByRole("combobox"), "picklist");
  },
};

// A structural-word label (AC-custom-fields refusal banner) permanently
// disables Confirm until the label is edited — no dismiss control on the
// banner itself.
export const StructuralWordRefusal: Story = {
  args: { open: true, object: "deal", onClose: () => {}, onConfirm: () => {} },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(canvas.getAllByRole("textbox")[0], "Link to account");
  },
};

// Confirm is in-flight (isPending on the create mutation).
export const Loading: Story = {
  args: {
    open: true,
    object: "deal",
    onClose: () => {},
    onConfirm: () => {},
    isLoading: true,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(canvas.getAllByRole("textbox")[0], "Renewal date");
  },
};
