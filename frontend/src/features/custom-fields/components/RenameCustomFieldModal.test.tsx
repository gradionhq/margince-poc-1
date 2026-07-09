import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { CustomField } from "../../../lib/api-client/generated/index.js";
import { RenameCustomFieldModal } from "./RenameCustomFieldModal.js";

const mockField: CustomField = {
  id: "field-1",
  workspace_id: "ws-1",
  object: "deal",
  label: "Renewal Date",
  slug: "renewal_date",
  type: "date",
  status: "active",
  column_name: "cf_renewal_date",
  created_by: "user-1",
  created_at: "2026-07-01T10:00:00Z",
  updated_at: "2026-07-01T10:00:00Z",
};

describe("RenameCustomFieldModal", () => {
  describe("initial state and TextInput", () => {
    it("renders a single TextInput seeded with field.label", () => {
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      expect(textInput).toHaveValue("Renewal Date");
    });
  });

  describe("Save button disabled states", () => {
    it("disables Save when trimmed value is empty", async () => {
      const user = userEvent.setup();
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      await user.clear(textInput);

      const saveBtn = screen.getByRole("button", { name: /save/i });
      expect(saveBtn).toBeDisabled();
    });

    it("disables Save when trimmed value equals current field.label (unchanged)", async () => {
      const user = userEvent.setup();
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      // Value is already "Renewal Date", so no change needed
      // Try to trigger change by clearing and retyping the same value
      await user.clear(textInput);
      await user.type(textInput, "Renewal Date");

      const saveBtn = screen.getByRole("button", { name: /save/i });
      expect(saveBtn).toBeDisabled();
    });

    it("disables Save when only whitespace is added", async () => {
      const user = userEvent.setup();
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      await user.clear(textInput);
      await user.type(textInput, "   ");

      const saveBtn = screen.getByRole("button", { name: /save/i });
      expect(saveBtn).toBeDisabled();
    });
  });

  describe("Save button enabled states", () => {
    it("enables Save when typing a new non-empty trimmed value", async () => {
      const user = userEvent.setup();
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      await user.clear(textInput);
      await user.type(textInput, "New Label");

      const saveBtn = screen.getByRole("button", { name: /save/i });
      expect(saveBtn).not.toBeDisabled();
    });
  });

  describe("Save button callback", () => {
    it("calls onSave with trimmed new label when Save is clicked", async () => {
      const user = userEvent.setup();
      const onSave = vi.fn();

      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={onSave}
        />,
      );

      const textInput = screen.getByRole("textbox");
      await user.clear(textInput);
      await user.type(textInput, "  Updated Field  ");

      const saveBtn = screen.getByRole("button", { name: /save/i });
      await user.click(saveBtn);

      expect(onSave).toHaveBeenCalledWith("Updated Field");
    });
  });

  describe("Cancel button", () => {
    it("calls onClose when Cancel is clicked", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();

      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={onClose}
          onSave={vi.fn()}
        />,
      );

      const cancelBtn = screen.getByRole("button", { name: /cancel/i });
      await user.click(cancelBtn);

      expect(onClose).toHaveBeenCalled();
    });
  });

  describe("isLoading prop", () => {
    it("disables Save button when isLoading is true", () => {
      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
          isLoading={true}
        />,
      );

      const saveBtn = screen.getByRole("button", { name: /save/i });
      expect(saveBtn).toBeDisabled();
    });
  });

  describe("interaction flow", () => {
    it("initially shows initial value; typing enables Save; clearing disables; typing same value disables", async () => {
      const user = userEvent.setup();

      render(
        <RenameCustomFieldModal
          open={true}
          field={mockField}
          onClose={vi.fn()}
          onSave={vi.fn()}
        />,
      );

      const textInput = screen.getByRole("textbox");
      const saveBtn = screen.getByRole("button", { name: /save/i });

      // Step 1: initial value is field.label
      expect(textInput).toHaveValue("Renewal Date");
      expect(saveBtn).toBeDisabled();

      // Step 2: typing new value enables Save
      await user.clear(textInput);
      await user.type(textInput, "New Value");
      expect(saveBtn).not.toBeDisabled();

      // Step 3: clearing it disables Save
      await user.clear(textInput);
      expect(saveBtn).toBeDisabled();

      // Step 4: typing same value as original disables Save
      await user.type(textInput, "Renewal Date");
      expect(saveBtn).toBeDisabled();
    });
  });
});
