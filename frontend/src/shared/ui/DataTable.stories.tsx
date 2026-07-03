import type { Meta, StoryObj } from "@storybook/react-vite";
import { DataTable } from "./DataTable.js";

type Person = { id: string; name: string; role: string };

const ROWS: Person[] = [
  { id: "1", name: "Alice", role: "Engineer" },
  { id: "2", name: "Bob", role: "Designer" },
  { id: "3", name: "Carol", role: "Product" },
];

function Demo() {
  return (
    <DataTable<Person>
      columns={[
        { key: "name", header: "Name", render: (r) => r.name },
        { key: "role", header: "Role", render: (r) => r.role },
      ]}
      rows={ROWS}
      getRowKey={(r) => r.id}
    />
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "gw-ui/DataTable",
  parameters: { design: { node: "zCnvG" } }, // Contacts Table
};
export default meta;
type Story = StoryObj<typeof Demo>;

export const Default: Story = {};
