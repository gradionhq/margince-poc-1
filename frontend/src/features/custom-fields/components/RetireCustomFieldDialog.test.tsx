import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../shared/ui/forge.js", () => ({
  ConfirmDialog: ({ open, onClose, onConfirm, title, description, confirmLabel, isLoading }: any) => {
    if (!open) return null;
    return (
      <div data-testid="confirm-dialog">
        <h2>{title}</h2>
        <p>{description}</p>
        <button onClick={onClose} aria-label="Close">
          Cancel
        </button>
        <button onClick={onConfirm} disabled={isLoading} aria-label={confirmLabel}>
          {confirmLabel}
        </button>
      </div>
    );
  },
}));

import { RetireCustomFieldDialog } from "./RetireCustomFieldDialog.js";

describe("RetireCustomFieldDialog", () => {
  describe("title and description rendering", () => {
    it("renders title 'Retire this field?'", () => {
      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />
      );

      expect(screen.getByText("Retire this field?")).toBeInTheDocument();
    });

    it("renders description with fieldLabel and objectDisplayName substituted", () => {
      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />
      );

      expect(
        screen.getByText(
          "Renewal Date will be hidden from new Deal records. Every existing value stays in place and the field remains in the audit trail."
        )
      ).toBeInTheDocument();
    });

    it("substitutes objectDisplayName correctly in description", () => {
      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Contact Name"
          objectDisplayName="Company"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />
      );

      expect(
        screen.getByText(
          "Contact Name will be hidden from new Company records. Every existing value stays in place and the field remains in the audit trail."
        )
      ).toBeInTheDocument();
    });
  });

  describe("button behavior", () => {
    it("calls onConfirm when confirm button is clicked", async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();

      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={onConfirm}
          onCancel={vi.fn()}
        />
      );

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      await user.click(confirmBtn);

      expect(onConfirm).toHaveBeenCalled();
    });

    it("calls onCancel when cancel button is clicked", async () => {
      const user = userEvent.setup();
      const onCancel = vi.fn();

      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={onCancel}
        />
      );

      const cancelBtn = screen.getByRole("button", { name: /close/i });
      await user.click(cancelBtn);

      expect(onCancel).toHaveBeenCalled();
    });
  });

  describe("isLoading prop", () => {
    it("disables confirm button when isLoading is true", () => {
      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
          isLoading={true}
        />
      );

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).toBeDisabled();
    });
  });

  describe("modal lifecycle", () => {
    it("does not render when open is false", () => {
      render(
        <RetireCustomFieldDialog
          open={false}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />
      );

      expect(screen.queryByText("Retire this field?")).not.toBeInTheDocument();
    });

    it("renders when open is true", () => {
      render(
        <RetireCustomFieldDialog
          open={true}
          fieldLabel="Renewal Date"
          objectDisplayName="Deal"
          onConfirm={vi.fn()}
          onCancel={vi.fn()}
        />
      );

      expect(screen.getByText("Retire this field?")).toBeInTheDocument();
    });
  });
});
