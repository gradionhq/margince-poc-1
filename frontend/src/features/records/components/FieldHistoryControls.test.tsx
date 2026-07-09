import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FieldHistoryControls } from "./FieldHistoryControls.js";

const FIELD_OPTIONS = [
  { field: "amount_minor", label: "Amount" },
  { field: "stage_id", label: "Stage" },
];

describe("FieldHistoryControls", () => {
  it("AC-3: the actor segmented control calls onActorChange with human/agent", async () => {
    const onActorChange = vi.fn();
    render(
      <FieldHistoryControls
        actor="all" onActorChange={onActorChange} field={null} onFieldChange={vi.fn()}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={vi.fn()}
        hasActiveFilters={false} onClearFilters={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /agent/i }));
    expect(onActorChange).toHaveBeenCalledWith("agent");
  });

  it("AC-4: clicking a field chip calls onFieldChange with that field's key", async () => {
    const onFieldChange = vi.fn();
    render(
      <FieldHistoryControls
        actor="all" onActorChange={vi.fn()} field={null} onFieldChange={onFieldChange}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={vi.fn()}
        hasActiveFilters={false} onClearFilters={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Stage" }));
    expect(onFieldChange).toHaveBeenCalledWith("stage_id");
  });

  it("AC-4: clicking 'All fields' calls onFieldChange with null", async () => {
    const onFieldChange = vi.fn();
    render(
      <FieldHistoryControls
        actor="all" onActorChange={vi.fn()} field="stage_id" onFieldChange={onFieldChange}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={vi.fn()}
        hasActiveFilters onClearFilters={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "All fields" }));
    expect(onFieldChange).toHaveBeenCalledWith(null);
  });

  it("AC-5: typing in the search box calls onSearchChange", async () => {
    const onSearchChange = vi.fn();
    render(
      <FieldHistoryControls
        actor="all" onActorChange={vi.fn()} field={null} onFieldChange={vi.fn()}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={onSearchChange}
        hasActiveFilters={false} onClearFilters={vi.fn()}
      />,
    );
    await userEvent.type(screen.getByPlaceholderText(/search fields/i), "x");
    expect(onSearchChange).toHaveBeenCalled();
  });

  it("AC-5: Clear filters only renders when hasActiveFilters, and calls onClearFilters", async () => {
    const onClearFilters = vi.fn();
    const { rerender } = render(
      <FieldHistoryControls
        actor="all" onActorChange={vi.fn()} field={null} onFieldChange={vi.fn()}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={vi.fn()}
        hasActiveFilters={false} onClearFilters={onClearFilters}
      />,
    );
    expect(screen.queryByRole("button", { name: /clear filters/i })).not.toBeInTheDocument();
    rerender(
      <FieldHistoryControls
        actor="agent" onActorChange={vi.fn()} field={null} onFieldChange={vi.fn()}
        fieldOptions={FIELD_OPTIONS} search="" onSearchChange={vi.fn()}
        hasActiveFilters onClearFilters={onClearFilters}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /clear filters/i }));
    expect(onClearFilters).toHaveBeenCalled();
  });
});
