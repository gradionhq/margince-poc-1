import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { ActorFilter } from "../hooks/useFieldHistoryView.js";
import { FieldHistoryControls } from "./FieldHistoryControls.js";

const FIELD_OPTIONS = [
  { field: "amount_minor", label: "Amount" },
  { field: "stage_id", label: "Stage" },
];

function Controlled({
  initialActor = "all",
  initialField = null,
}: {
  initialActor?: ActorFilter;
  initialField?: string | null;
}) {
  const [actor, setActor] = useState<ActorFilter>(initialActor);
  const [field, setField] = useState<string | null>(initialField);
  const [search, setSearch] = useState("");
  const hasActiveFilters = actor !== "all" || field !== null || search !== "";

  return (
    <FieldHistoryControls
      actor={actor}
      onActorChange={setActor}
      field={field}
      onFieldChange={setField}
      fieldOptions={FIELD_OPTIONS}
      search={search}
      onSearchChange={setSearch}
      hasActiveFilters={hasActiveFilters}
      onClearFilters={() => {
        setActor("all");
        setField(null);
        setSearch("");
      }}
    />
  );
}

const meta: Meta<typeof Controlled> = {
  component: Controlled,
  title: "CRM/Records/FieldHistoryControls",
};
export default meta;
type Story = StoryObj<typeof Controlled>;

export const NoActiveFilters: Story = {};

export const ActiveFilter: Story = {
  args: { initialActor: "agent", initialField: "stage_id" },
};
