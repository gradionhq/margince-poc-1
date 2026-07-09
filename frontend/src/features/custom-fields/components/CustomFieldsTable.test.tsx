// biome-ignore-all lint/a11y/useValidAriaRole: `role` is a domain prop of CustomFieldsTable, not the ARIA role attribute.
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type {
  CustomField,
  Member,
} from "../../../lib/api-client/generated/index.js";
import { CustomFieldsTable } from "./CustomFieldsTable.js";

// Mirrors the row shape CustomFieldsTable.tsx builds for the real DataTable.
type MockRow = {
  id: string;
  field?: CustomField;
  staged?: { label: string; type: string };
};

type MockColumn = {
  key: string;
  header: string;
  render: (row: MockRow) => ReactNode;
};

type MockMenuItem = { id: string; label: string; onSelect: () => void };

// Mock all external components and utilities
vi.mock("../lib/customFieldRules.js", () => ({
  OBJECT_CHIPS: [
    { value: "deal", label: "Deal" },
    { value: "organization", label: "Company" },
    { value: "person", label: "Contact" },
    { value: "lead", label: "Lead" },
    { value: "activity", label: "Activity" },
  ],
  buildApiKey: vi.fn((object: string, slug: string) => {
    if (!slug) return "";
    return `${object}.cf_${slug}`;
  }),
  resolveMemberName: vi.fn((members: Member[], userId: string) => {
    const member = members.find((m) => m.user_id === userId);
    return member ? member.display_name : "Unknown";
  }),
}));

vi.mock("../../../shared/ui/DataTable.js", () => ({
  DataTable: ({
    rows,
    columns,
    getRowKey,
    getRowProps,
  }: {
    rows: MockRow[];
    columns: MockColumn[];
    getRowKey: (row: MockRow) => string;
    getRowProps?: (row: MockRow) => Record<string, string>;
  }) => (
    <table data-testid="data-table">
      <thead>
        <tr>
          {columns.map((col) => (
            <th key={col.key}>{col.header}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => {
          const rowKey = getRowKey(row);
          const rowProps = getRowProps?.(row) || {};
          return (
            <tr
              key={rowKey}
              data-testid={`table-row-${rowKey}`}
              className={rowProps.className}
              {...(rowProps["data-staged"] && {
                "data-staged": rowProps["data-staged"],
              })}
            >
              {columns.map((col) => (
                <td key={col.key} data-testid={`cell-${rowKey}-${col.key}`}>
                  {col.render(row)}
                </td>
              ))}
            </tr>
          );
        })}
      </tbody>
    </table>
  ),
}));

vi.mock("../../../shared/ui/forge.js", () => ({
  Chip: ({
    children,
    className,
  }: {
    children: ReactNode;
    className?: string;
  }) => (
    <span data-testid="chip" className={className}>
      {children}
    </span>
  ),
  EmptyState: () => <div data-testid="empty-state">No custom fields</div>,
  StatusBadge: ({ children }: { children: ReactNode }) => (
    <span data-testid="status-badge">{children}</span>
  ),
  Icon: () => <span data-testid="icon" />,
  IconButton: ({
    onClick,
    ...props
  }: { onClick?: () => void } & Record<string, unknown>) => (
    <button
      type="button"
      onClick={onClick}
      data-testid="icon-button"
      {...props}
    />
  ),
}));

vi.mock("../../../shared/ui/FieldGuard.tsx", () => ({
  FieldGuard: ({
    mode,
    children,
  }: {
    mode: "visible" | "masked" | "readonly";
    children: ReactNode;
  }) => <span data-testid={`field-guard-${mode}`}>{children}</span>,
}));

vi.mock("../../../shared/ui/ContextMenu.js", () => ({
  ContextMenu: ({ items }: { items: MockMenuItem[] }) => (
    <div data-testid="context-menu">
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          data-testid={`context-menu-item-${item.id}`}
          onClick={item.onSelect}
        >
          {item.label}
        </button>
      ))}
    </div>
  ),
}));

const mockMembers: Member[] = [
  {
    user_id: "user-1",
    email: "alice@example.com",
    display_name: "Alice Smith",
    status: "active",
    is_agent: false,
    roles: ["admin"],
  },
  {
    user_id: "user-2",
    email: "bob@example.com",
    display_name: "Bob Johnson",
    status: "active",
    is_agent: false,
    roles: ["rep"],
  },
];

const mockFields: CustomField[] = [
  {
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
  },
  {
    id: "field-2",
    workspace_id: "ws-1",
    object: "deal",
    label: "Priority",
    slug: "priority",
    type: "text",
    status: "active",
    column_name: "cf_priority",
    created_by: "user-2",
    created_at: "2026-07-02T10:00:00Z",
    updated_at: "2026-07-02T10:00:00Z",
  },
  {
    id: "field-3",
    workspace_id: "ws-1",
    object: "deal",
    label: "Old Field",
    slug: "old_field",
    type: "text",
    status: "retired",
    column_name: "cf_old_field",
    created_by: "user-1",
    created_at: "2026-06-01T10:00:00Z",
    updated_at: "2026-07-05T14:30:00Z",
  },
];

describe("CustomFieldsTable", () => {
  describe("1. Object chips rendering", () => {
    it("renders all 5 chips from OBJECT_CHIPS", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const chips = screen.getAllByTestId("chip");
      expect(chips).toHaveLength(5);
    });

    it("marks the selected chip with data-selected=true", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const selectedChip = screen.getByTestId("chip-deal");
      expect(selectedChip).toHaveAttribute("data-selected", "true");
    });

    it("renders non-selected chips without a count badge", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 2)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const nonSelectedChips = ["organization", "person", "lead", "activity"];
      for (const objKey of nonSelectedChips) {
        const chip = screen.getByTestId(`chip-${objKey}`);
        const badge = chip.querySelector("[data-testid='object-count']");
        expect(badge).not.toBeInTheDocument();
      }
      // Only the selected chip should have object-count testid
      const countBadges = screen.queryAllByTestId("object-count");
      expect(countBadges).toHaveLength(1); // only one for the selected chip
    });
  });

  describe("2. Selected chip count badge", () => {
    it("shows count badge for selected chip with fields.length (no staged row)", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 3)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={null}
        />,
      );
      const countBadge = screen.getByTestId("object-count");
      expect(countBadge).toHaveTextContent("3");
    });

    it("increments count badge when stagedRow is present", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 3)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "Test Field", type: "text" }}
        />,
      );
      const countBadge = screen.getByTestId("object-count");
      expect(countBadge).toHaveTextContent("4");
    });
  });

  describe("3. Table columns", () => {
    it("renders Label column with plain text", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const cell = screen.getByTestId("cell-field-1-label");
      expect(cell).toHaveTextContent("Renewal Date");
    });

    it("renders API Key column in font-mono via buildApiKey()", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const cell = screen.getByTestId("cell-field-1-apiKey");
      expect(cell).toHaveTextContent("deal.cf_renewal_date");
      const span = cell.querySelector("span.font-mono");
      expect(span).toBeInTheDocument();
    });

    it("renders Type column as Chip component", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const typeCell = screen.getByTestId("cell-field-1-type");
      expect(typeCell).toHaveTextContent("date");
    });

    it("renders Added by column with FieldGuard mode=visible for admin role", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      screen.getByTestId("cell-field-1-addedBy");
      const fieldGuard = screen.getByTestId("field-guard-visible");
      expect(fieldGuard).toHaveTextContent("Alice Smith");
    });

    it("renders Added by column with FieldGuard mode=masked for non-admin role", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="rep"
        />,
      );
      const fieldGuard = screen.getByTestId("field-guard-masked");
      expect(fieldGuard).toBeInTheDocument();
    });
  });

  describe("4. Row actions for admins", () => {
    it("renders ContextMenu for active fields when role=admin", () => {
      const onEdit = vi.fn();
      const onRetire = vi.fn();
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          onEdit={onEdit}
          onRetire={onRetire}
        />,
      );
      const contextMenu = screen.getByTestId("context-menu");
      expect(contextMenu).toBeInTheDocument();
    });

    it("omits row actions entirely for non-admin role (not disabled)", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="rep"
        />,
      );
      // For non-admin, context menu should not be rendered in active fields
      // (we check this by ensuring no Edit/Archive actions appear)
      const contextMenuItems = screen.queryAllByTestId(/context-menu-item/);
      expect(contextMenuItems.length).toBe(0);
    });

    it("calls onEdit when Edit action is selected", async () => {
      const onEdit = vi.fn();
      const userEventModule = await import("@testing-library/user-event");
      const user = userEventModule.default.setup();

      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          onEdit={onEdit}
        />,
      );
      const editButton = screen.getByTestId("context-menu-item-edit");
      await user.click(editButton);
      expect(onEdit).toHaveBeenCalledWith(mockFields[0]);
    });

    it("calls onRetire when Archive action is selected", async () => {
      const onRetire = vi.fn();
      const userEventModule = await import("@testing-library/user-event");
      const user = userEventModule.default.setup();

      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          onRetire={onRetire}
        />,
      );
      const archiveButton = screen.getByTestId("context-menu-item-archive");
      await user.click(archiveButton);
      expect(onRetire).toHaveBeenCalledWith(mockFields[0]);
    });
  });

  describe("5. Retired rows", () => {
    it("renders retired fields with reduced opacity", () => {
      render(
        <CustomFieldsTable
          fields={[mockFields[2]]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const row = screen.getByTestId("table-row-field-3");
      expect(row).toHaveClass("opacity-60");
    });

    it("shows Retired StatusBadge for retired fields", () => {
      render(
        <CustomFieldsTable
          fields={[mockFields[2]]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const statusBadge = screen.getByTestId("status-badge");
      expect(statusBadge).toHaveTextContent("Retired");
    });

    it("omits row actions for retired fields", () => {
      render(
        <CustomFieldsTable
          fields={[mockFields[2]]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      // Retired row should not have context menu
      const retiredRow = screen.getByTestId("table-row-field-3");
      const contextMenuInRow = retiredRow.querySelector(
        "[data-testid='context-menu']",
      );
      expect(contextMenuInRow).not.toBeInTheDocument();
    });
  });

  describe("6. Explanatory note", () => {
    it("always renders explanatory note about core fields", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 2)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      expect(
        screen.getByText(/Core fields are not shown/i),
      ).toBeInTheDocument();
    });

    it("renders note even with empty fields", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      expect(
        screen.getByText(/Core fields are not shown/i),
      ).toBeInTheDocument();
    });
  });

  describe("7. Staged row", () => {
    it("renders staged row above real rows with data-staged=true", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 2)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "text" }}
        />,
      );
      const stagedRow = screen.getByTestId("table-row-staged");
      expect(stagedRow).toHaveAttribute("data-staged", "true");
      // Staged row should appear first in DOM order
      const rows = screen.getAllByRole("row");
      expect(rows[1]).toHaveAttribute("data-testid", "table-row-staged");
    });

    it("renders staged row label as writing…", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "text" }}
        />,
      );
      const stagedCell = screen.getByTestId("cell-staged-label");
      expect(stagedCell).toHaveTextContent("writing…");
    });

    it("renders staged row API key as placeholder —", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "text" }}
        />,
      );
      const stagedApiKeyCell = screen.getByTestId("cell-staged-apiKey");
      expect(stagedApiKeyCell).toHaveTextContent("—");
    });

    it("renders staged row with type from stagedRow prop", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "currency" }}
        />,
      );
      const stagedTypeCell = screen.getByTestId("cell-staged-type");
      expect(stagedTypeCell).toHaveTextContent("currency");
    });

    it("omits row actions for staged row", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "text" }}
        />,
      );
      const stagedRow = screen.getByTestId("table-row-staged");
      const contextMenuInRow = stagedRow.querySelector(
        "[data-testid='context-menu']",
      );
      expect(contextMenuInRow).not.toBeInTheDocument();
    });
  });

  describe("8. Empty state", () => {
    it("renders EmptyState when fields.length === 0 and no staged row", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={null}
        />,
      );
      const emptyState = screen.getByTestId("empty-state");
      expect(emptyState).toBeInTheDocument();
    });

    it("does not render EmptyState when fields has items", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={null}
        />,
      );
      const emptyState = screen.queryByTestId("empty-state");
      expect(emptyState).not.toBeInTheDocument();
    });

    it("does not render EmptyState when stagedRow is present", () => {
      render(
        <CustomFieldsTable
          fields={[]}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "New Field", type: "text" }}
        />,
      );
      const emptyState = screen.queryByTestId("empty-state");
      expect(emptyState).not.toBeInTheDocument();
    });
  });

  describe("integration", () => {
    it("renders mixed active and retired fields with correct styling", () => {
      render(
        <CustomFieldsTable
          fields={mockFields}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
        />,
      );
      const activeRow1 = screen.getByTestId("table-row-field-1");
      const activeRow2 = screen.getByTestId("table-row-field-2");
      const retiredRow = screen.getByTestId("table-row-field-3");

      expect(activeRow1).not.toHaveClass("opacity-60");
      expect(activeRow2).not.toHaveClass("opacity-60");
      expect(retiredRow).toHaveClass("opacity-60");
    });

    it("renders staged row plus multiple fields with correct count badge", () => {
      render(
        <CustomFieldsTable
          fields={mockFields.slice(0, 2)}
          members={mockMembers}
          selectedObject="deal"
          role="admin"
          stagedRow={{ label: "Future Field", type: "numeric" }}
        />,
      );
      const countBadge = screen.getByTestId("object-count");
      expect(countBadge).toHaveTextContent("3"); // 2 active + 1 staged
    });
  });
});
