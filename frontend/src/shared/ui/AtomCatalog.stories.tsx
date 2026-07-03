import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  Badge,
  Button,
  Divider,
  Icon,
  IconButton,
  Kbd,
  PresenceIndicator,
  SectionHeader,
  Skeleton,
  StatusBadge,
  StatusDot,
  TextInput,
} from "./forge.js";

function Catalog() {
  return (
    <div className="flex flex-col gap-gf-md">
      <SectionHeader label="Buttons" />
      <div className="flex gap-gf-sm items-center">
        <Button>Primary</Button>
        <IconButton icon="Check" label="Confirm" />
        <IconButton icon="Plus" label="Add" variant="ghost" size="sm" />
      </div>
      <Divider />
      <SectionHeader label="Status" />
      <div className="flex gap-gf-sm items-center">
        <Badge count={5} />
        <Badge count={99} variant="dot" />
        <StatusBadge label="OK" variant="success" />
        <StatusBadge label="Warning" variant="warning" />
        <StatusBadge label="Error" variant="error" />
        <StatusDot state="success" />
        <StatusDot state="running" />
        <StatusDot state="error" />
        <PresenceIndicator status="online" />
        <PresenceIndicator status="away" />
        <PresenceIndicator status="offline" />
      </div>
      <Divider />
      <SectionHeader label="Input" />
      <TextInput placeholder="Type…" value="" onChange={() => {}} />
      <TextInput
        placeholder="Search…"
        value=""
        onChange={() => {}}
        leadingIcon="Search"
      />
      <Kbd keys={["⌘", "K"]} />
      <Divider />
      <SectionHeader label="Icon" />
      <div className="flex gap-gf-sm items-center">
        <Icon name="Search" size={20} />
        <Icon name="Bell" size={20} />
        <Icon name="Settings" size={20} />
        <Icon name="Check" size={20} />
      </div>
      <Divider />
      <SectionHeader label="Skeleton" />
      <Skeleton className="h-4 w-32" />
      <Skeleton variant="circle" />
      <Skeleton variant="block" className="h-16 w-48" />
    </div>
  );
}

const meta: Meta<typeof Catalog> = {
  component: Catalog,
  title: "gw-ui/Atom Catalog",
};
export default meta;
type Story = StoryObj<typeof Catalog>;

export const Default: Story = {};
