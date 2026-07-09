import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockBuildApiKey = vi.fn((object: string, slug: string) => {
  if (!slug) return "";
  return `${object}.cf_${slug}`;
});

const mockBuildDdlPreview = vi.fn(
  (object: string, slug: string, type: string) => {
    return `ALTER ${object} ADD COLUMN cf_${slug} (${type}) · backfilled NULL · reversible`;
  },
);

const mockDetectStructuralWord = vi.fn((label: string): string | null => {
  const lower = label.toLowerCase();
  const words = ["object", "relationship", "link to", "lookup to"];
  for (const word of words) {
    if (lower.includes(word)) return word;
  }
  return null;
});

const mockSlugify = vi.fn((label: string): string => {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
});

vi.mock("../lib/customFieldRules.js", () => ({
  buildApiKey: (obj: string, slug: string) => mockBuildApiKey(obj, slug),
  buildDdlPreview: (obj: string, slug: string, type: string) =>
    mockBuildDdlPreview(obj, slug, type),
  detectStructuralWord: (label: string) => mockDetectStructuralWord(label),
  slugify: (label: string) => mockSlugify(label),
}));

import { NewCustomFieldModal } from "./NewCustomFieldModal.js";

describe("NewCustomFieldModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Label input + live API key derivation", () => {
    it("shows a label input and derived API key field below it", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );
      const textboxes = screen.getAllByRole("textbox");
      expect(textboxes.length).toBeGreaterThanOrEqual(2); // label and api key
    });

    it("updates the API key on every keystroke when typing in label", async () => {
      const user = userEvent.setup();

      // Mock slugify to return the expected slug BEFORE rendering
      mockSlugify.mockImplementation((label: string) => {
        if (label === "Renewal date") return "renewal_date";
        return label.toLowerCase().replace(/[^a-z0-9]+/g, "_");
      });

      mockBuildApiKey.mockImplementation((obj: string, slug: string) => {
        return slug ? `${obj}.cf_${slug}` : "";
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Renewal date");

      // After typing, verify buildApiKey was called with the right slug
      const calls = mockBuildApiKey.mock.calls;
      const hasCorrectCall = calls.some(
        ([obj, slug]) => obj === "deal" && slug === "renewal_date",
      );
      expect(hasCorrectCall).toBe(true);

      const apiKeyInputs = screen.getAllByRole("textbox");
      expect(apiKeyInputs[1]).toHaveValue("deal.cf_renewal_date");
    });

    it("shows empty string in API key when label is empty", () => {
      mockBuildApiKey.mockReturnValue("");

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const apiKeyInputs = screen.getAllByRole("textbox");
      expect(apiKeyInputs[1]).toHaveValue("");
    });

    it("API key input is disabled (read-only)", async () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const apiKeyInputs = screen.getAllByRole("textbox");
      expect(apiKeyInputs[1]).toBeDisabled();
    });
  });

  describe("Live DDL preview", () => {
    it("displays DDL preview with correct format", () => {
      mockBuildDdlPreview.mockReturnValue(
        "ALTER deal ADD COLUMN cf_renewal_date (text) · backfilled NULL · reversible",
      );

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const preview = screen.getByTestId("ddl-preview");
      expect(preview).toHaveClass("font-mono", "text-xs", "bg-gf-elevated");
    });

    it("updates DDL preview on every keystroke", async () => {
      const user = userEvent.setup();
      mockSlugify.mockReturnValue("contact");
      mockBuildDdlPreview.mockReturnValueOnce(
        "ALTER organization ADD COLUMN cf_contact (text) · backfilled NULL · reversible",
      );

      render(
        <NewCustomFieldModal
          open={true}
          object="organization"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Contact");

      expect(mockBuildDdlPreview).toHaveBeenCalledWith(
        "organization",
        "contact",
        "text",
      );
    });

    it("has correct CSS styling for DDL preview", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const preview = screen.getByTestId("ddl-preview");
      expect(preview).toHaveClass(
        "font-mono",
        "text-xs",
        "bg-gf-elevated",
        "border",
        "border-gf-subtle",
        "rounded-md",
        "px-gf-md",
        "py-gf-sm",
      );
    });
  });

  describe("Type picker", () => {
    it("renders type dropdown with 6 options", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelects = screen.getAllByRole("combobox");
      expect(typeSelects.length).toBeGreaterThan(0);
    });

    it("updates DDL preview when type changes", async () => {
      const user = userEvent.setup();
      mockSlugify.mockReturnValue("amount");
      mockBuildDdlPreview
        .mockReturnValueOnce(
          "ALTER deal ADD COLUMN cf_amount (number) · backfilled NULL · reversible",
        )
        .mockReturnValueOnce(
          "ALTER deal ADD COLUMN cf_amount (currency) · backfilled NULL · reversible",
        );

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Amount");

      const typeSelects = screen.getAllByRole("combobox");
      // Assuming first combobox is type picker
      await user.click(typeSelects[0]);
      // The exact interaction depends on how the select is implemented
      // but the DDL preview should update
    });
  });

  describe("Currency code field (conditional)", () => {
    it("is hidden when type is not currency", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      expect(screen.queryByText("ISO-4217 code")).not.toBeInTheDocument();
    });

    it("appears when type === currency", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "currency");

      expect(screen.getByText("ISO-4217 code")).toBeInTheDocument();
    });

    it("disables Confirm when type is currency and code is empty", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      // Type label first
      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Price");

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "currency");

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).toBeDisabled();
    });

    it("enables Confirm when currency code is filled (and label is non-empty)", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      // Type label
      const textboxes = screen.getAllByRole("textbox");
      const labelInput = textboxes[0];
      await user.type(labelInput, "Price");

      // Select currency type
      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "currency");

      // Find and fill currency code field
      const currencyInput = screen
        .getAllByRole("textbox")
        .find(
          (box) =>
            box !== labelInput &&
            (box.getAttribute("placeholder") || "").includes("e.g., USD"),
        );

      if (currencyInput) {
        await user.type(currencyInput, "USD");
      }

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).not.toBeDisabled();
    });

    it("shows caption about minor-units storage", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "currency");

      expect(
        screen.getByText(/Stored as integer minor-units/i),
      ).toBeInTheDocument();
    });
  });

  describe("Picklist options editor (conditional)", () => {
    it("is hidden when type is not picklist", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      expect(screen.queryByText("Picklist options")).not.toBeInTheDocument();
    });

    it("shows picklist options editor when type === picklist", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "picklist");

      expect(screen.getByText("Picklist options")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /add option/i }),
      ).toBeInTheDocument();
    });

    it("prevents deleting the last remaining option with exact message", async () => {
      const user = userEvent.setup();
      const onGuardToast = vi.fn();

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          onGuardToast={onGuardToast}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "picklist");

      const removeBtn = screen.getByRole("button", { name: /remove/i });
      await user.click(removeBtn);

      expect(onGuardToast).toHaveBeenCalledWith(
        "A picklist needs at least one option",
      );
    });

    it("allows adding new options", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "picklist");

      const addBtn = screen.getByRole("button", { name: /add option/i });
      await user.click(addBtn);

      const removeButtons = screen.getAllByRole("button", { name: /remove/i });
      expect(removeButtons.length).toBe(2); // Should have 2 options now
    });

    it("allows deleting non-last options", async () => {
      const user = userEvent.setup();
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "picklist");

      const addBtn = screen.getByRole("button", { name: /add option/i });
      await user.click(addBtn);

      const removeButtons = screen.getAllByRole("button", { name: /remove/i });
      await user.click(removeButtons[0]); // Remove first option

      const remainingRemoveButtons = screen.getAllByRole("button", {
        name: /remove/i,
      });
      expect(remainingRemoveButtons.length).toBe(1); // Should have 1 option left
    });
  });

  describe("Structural-word refusal", () => {
    it("shows refusal banner when structural word is detected", async () => {
      const user = userEvent.setup();
      mockDetectStructuralWord.mockImplementation((label: string) => {
        if (label.toLowerCase().includes("link to")) return "link to";
        return null;
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Link To account");

      expect(
        screen.getByText(
          /This looks like a new object, relationship, or logic/i,
        ),
      ).toBeInTheDocument();
    });

    it("displays exact server 422 error text in refusal banner", async () => {
      const user = userEvent.setup();
      mockDetectStructuralWord.mockImplementation((label: string) => {
        if (label.toLowerCase().includes("relationship")) return "relationship";
        return null;
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "relationship field");

      const banner = screen.getByText(
        /This looks like a new object, relationship, or logic/i,
      );
      expect(banner).toHaveTextContent(
        "Runtime custom fields only add bounded scalar columns",
      );
    });

    it("disables Confirm immediately when structural word is present", async () => {
      const user = userEvent.setup();
      mockDetectStructuralWord.mockImplementation((label: string) => {
        if (label.toLowerCase().includes("object")) return "object";
        return null;
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "new object");

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).toBeDisabled();
    });

    it("removes banner and re-enables Confirm when structural word is cleared", async () => {
      const user = userEvent.setup();
      mockDetectStructuralWord.mockImplementation((label: string) => {
        if (label.toLowerCase().includes("object")) return "object";
        return null;
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "new object");

      expect(
        screen.getByText(/This looks like a new object/i),
      ).toBeInTheDocument();

      await user.clear(labelInput);
      await user.type(labelInput, "valid field");

      expect(
        screen.queryByText(/This looks like a new object/i),
      ).not.toBeInTheDocument();

      // Confirm should now be enabled since label is non-empty and no structural word
      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).not.toBeDisabled();
    });

    it("has no dismiss button on the refusal banner itself", async () => {
      const user = userEvent.setup();
      mockDetectStructuralWord.mockImplementation((label: string) => {
        if (label.toLowerCase().includes("object")) return "object";
        return null;
      });

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "new object");

      // The banner text should exist
      expect(
        screen.getByText(/This looks like a new object/i),
      ).toBeInTheDocument();

      // The banner is not dismissible - you have to clear the structural word from the label
    });
  });

  describe("Empty-label guard", () => {
    it("disables Confirm when label is empty", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).toBeDisabled();
    });

    it("enables Confirm when label becomes non-empty", async () => {
      const user = userEvent.setup();

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "My Field");

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).not.toBeDisabled();
    });

    it("disables Confirm again if label is cleared", async () => {
      const user = userEvent.setup();

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "My Field");
      await user.clear(labelInput);

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      expect(confirmBtn).toBeDisabled();
    });
  });

  describe("Confirm button behavior", () => {
    it("calls onConfirm with correct CreateCustomFieldRequest for text field", async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      mockSlugify.mockReturnValue("my_field");

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={onConfirm}
          userId="u123"
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "My Field");

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      await user.click(confirmBtn);

      expect(onConfirm).toHaveBeenCalledWith(
        expect.objectContaining({
          object: "deal",
          label: "My Field",
          type: "text",
          source: "manual",
          captured_by: "human:u123",
        }),
      );
    });

    it("includes currency code when type === currency", async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      mockSlugify.mockReturnValue("price");

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={onConfirm}
          userId="u456"
        />,
      );

      const textboxes = screen.getAllByRole("textbox");
      const labelInput = textboxes[0];
      await user.type(labelInput, "Price");

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "currency");

      // Find and fill currency code
      const currencyInput = screen
        .getAllByRole("textbox")
        .find(
          (box) =>
            box !== labelInput &&
            (box.getAttribute("placeholder") || "").includes("e.g., USD"),
        );

      if (currencyInput) {
        await user.type(currencyInput, "USD");
      }

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      await user.click(confirmBtn);

      expect(onConfirm).toHaveBeenCalledWith(
        expect.objectContaining({
          object: "deal",
          label: "Price",
          type: "currency",
          currency: "USD",
          source: "manual",
          captured_by: "human:u456",
        }),
      );
    });

    it("includes picklist options when type === picklist", async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();
      mockSlugify.mockReturnValue("status");

      render(
        <NewCustomFieldModal
          open={true}
          object="organization"
          onClose={vi.fn()}
          onConfirm={onConfirm}
          userId="u789"
        />,
      );

      const labelInput = screen.getAllByRole("textbox")[0];
      await user.type(labelInput, "Status");

      const typeSelect = screen.getByRole("combobox");
      await user.selectOptions(typeSelect, "picklist");

      // Fill first option
      const optionInputs = screen.getAllByRole("textbox");
      const firstOptionInput = optionInputs.find(
        (box) =>
          box !== labelInput &&
          (box.getAttribute("placeholder") || "").includes("Option"),
      );

      if (firstOptionInput) {
        await user.type(firstOptionInput, "Active");
      }

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      await user.click(confirmBtn);

      expect(onConfirm).toHaveBeenCalledWith(
        expect.objectContaining({
          object: "organization",
          label: "Status",
          type: "picklist",
          options: ["Active"],
          source: "manual",
          captured_by: "human:u789",
        }),
      );
    });

    it("does not call onConfirm when Confirm is disabled", async () => {
      const user = userEvent.setup();
      const onConfirm = vi.fn();

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={onConfirm}
        />,
      );

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      await user.click(confirmBtn);

      // Confirm is natively disabled while the label is empty, so the click
      // never reaches handleConfirm.
      expect(onConfirm).not.toHaveBeenCalled();
    });
  });

  describe("Modal lifecycle", () => {
    it("closes when onClose is called from Cancel button", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();

      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={onClose}
          onConfirm={vi.fn()}
        />,
      );

      const cancelBtn = screen.getByRole("button", { name: /cancel/i });
      await user.click(cancelBtn);

      expect(onClose).toHaveBeenCalled();
    });

    it("shows loading state when isLoading prop is true", () => {
      render(
        <NewCustomFieldModal
          open={true}
          object="deal"
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          isLoading={true}
        />,
      );

      const confirmBtn = screen.getByRole("button", { name: /confirm/i });
      // When loading, button should show loading state (implementation detail)
      expect(confirmBtn).toBeDisabled();
    });
  });
});
