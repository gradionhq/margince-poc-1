import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { CustomField, Member } from "../../../lib/api-client/generated/index.js";

const mockDeriveAuditEntries = vi.fn();
const mockResolveMemberName = vi.fn();

vi.mock("../lib/customFieldRules.js", () => ({
  deriveAuditEntries: (fields: CustomField[]) => mockDeriveAuditEntries(fields),
  resolveMemberName: (members: Member[], userId: string) =>
    mockResolveMemberName(members, userId),
}));

vi.mock("../../../shared/ui/forge.js", () => ({
  Skeleton: ({ height, "data-testid": testId }: any) => (
    <div data-testid={testId} style={{ height }}>
      Loading...
    </div>
  ),
}));

vi.mock("../../../shared/ui/FieldGuard.tsx", () => ({
  FieldGuard: ({ mode, children }: any) => {
    if (mode === "masked") {
      return (
        <span data-testid={`field-guard-${mode}`} role="img" aria-label="Masked value">
          ••••
        </span>
      );
    }
    return <span data-testid={`field-guard-${mode}`}>{children}</span>;
  },
}));

import { CustomFieldAuditCard } from "./CustomFieldAuditCard.js";

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
    label: "Old Field",
    slug: "old_field",
    type: "text",
    status: "retired",
    column_name: "cf_old_field",
    created_by: "user-2",
    created_at: "2026-06-01T10:00:00Z",
    updated_at: "2026-07-05T14:30:00Z",
  },
];

describe("CustomFieldAuditCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("loading state", () => {
    it("renders Skeleton with data-testid='audit-card-skeleton' when isLoading is true", () => {
      mockDeriveAuditEntries.mockReturnValue([]);

      render(
        <CustomFieldAuditCard
          fields={[]}
          members={mockMembers}
          role="admin"
          isLoading={true}
          isError={false}
        />
      );

      expect(screen.getByTestId("audit-card-skeleton")).toBeInTheDocument();
    });
  });

  describe("error state", () => {
    it("shows error text when isError is true", () => {
      mockDeriveAuditEntries.mockReturnValue([]);

      render(
        <CustomFieldAuditCard
          fields={[]}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={true}
        />
      );

      expect(screen.getByText(/something went wrong/i)).toBeInTheDocument();
    });
  });

  describe("empty entries state", () => {
    it("shows 'No changes yet.' when entries are empty", () => {
      mockDeriveAuditEntries.mockReturnValue([]);

      render(
        <CustomFieldAuditCard
          fields={[]}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(screen.getByText("No changes yet.")).toBeInTheDocument();
    });
  });

  describe("audit entries rendering", () => {
    it("renders 'added' entry with correct format", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-1",
          actorId: "user-1",
          label: "Renewal Date",
          type: "date",
          object: "deal",
          occurredAt: "2026-07-01T10:00:00Z",
          auditRef: "audit#field1id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Alice Smith");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(
        screen.getByText((content, element) => {
          return element &&
            element.textContent === "Alice Smith added Renewal Date (date) to deal" &&
            element.tagName === "P";
        })
      ).toBeInTheDocument();
    });

    it("renders 'retired' entry with correct format", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-2",
          actorId: "user-2",
          label: "Old Field",
          type: "text",
          object: "deal",
          occurredAt: "2026-07-05T14:30:00Z",
          auditRef: "audit#field2id-retired",
          action: "retired",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Bob Johnson");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(1, 2)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(
        screen.getByText((content, element) => {
          return element &&
            element.textContent === "Bob Johnson retired Old Field" &&
            element.tagName === "P";
        })
      ).toBeInTheDocument();
    });

    it("renders both added and retired entries for a retired field", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-2",
          actorId: "user-2",
          label: "Old Field",
          type: "text",
          object: "deal",
          occurredAt: "2026-07-05T14:30:00Z",
          auditRef: "audit#field2id-retired",
          action: "retired",
        },
        {
          id: "field-2",
          actorId: "user-2",
          label: "Old Field",
          type: "text",
          object: "deal",
          occurredAt: "2026-06-01T10:00:00Z",
          auditRef: "audit#field2id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Bob Johnson");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(1, 2)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(
        screen.getByText((content, element) => {
          return element &&
            element.textContent === "Bob Johnson retired Old Field" &&
            element.tagName === "P";
        })
      ).toBeInTheDocument();
      expect(
        screen.getByText((content, element) => {
          return element &&
            element.textContent === "Bob Johnson added Old Field (text) to deal" &&
            element.tagName === "P";
        })
      ).toBeInTheDocument();
    });
  });

  describe("date formatting", () => {
    it("includes formatted date and auditRef as caption", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-1",
          actorId: "user-1",
          label: "Renewal Date",
          type: "date",
          object: "deal",
          occurredAt: "2026-07-01T10:00:00Z",
          auditRef: "audit#field1id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Alice Smith");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      // Date should be formatted - check caption paragraph
      const captionText = screen.getByText((content, element) => {
        return element &&
          element.textContent?.includes("7/1/2026") &&
          element.textContent?.includes("audit#field1id-created") &&
          element.tagName === "P";
      });
      expect(captionText).toBeInTheDocument();
    });
  });

  describe("FieldGuard for actor name - admin role", () => {
    it("wraps actorName in FieldGuard with mode='visible' when role === 'admin'", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-1",
          actorId: "user-1",
          label: "Renewal Date",
          type: "date",
          object: "deal",
          occurredAt: "2026-07-01T10:00:00Z",
          auditRef: "audit#field1id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Alice Smith");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(screen.getByTestId("field-guard-visible")).toBeInTheDocument();
      expect(screen.getByText("Alice Smith")).toBeInTheDocument();
    });
  });

  describe("FieldGuard for actor name - non-admin role", () => {
    it("wraps actorName in FieldGuard with mode='masked' when role !== 'admin'", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-1",
          actorId: "user-1",
          label: "Renewal Date",
          type: "date",
          object: "deal",
          occurredAt: "2026-07-01T10:00:00Z",
          auditRef: "audit#field1id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Alice Smith");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          role="rep"
          isLoading={false}
          isError={false}
        />
      );

      expect(screen.getByTestId("field-guard-masked")).toBeInTheDocument();
      // Name should not be visible, masked glyph should be shown instead
      const fieldGuardMasked = screen.getByTestId("field-guard-masked");
      expect(fieldGuardMasked.textContent).toBe("••••");
    });
  });

  describe("complete integration", () => {
    it("calls deriveAuditEntries with fields and renders entries in ul", () => {
      mockDeriveAuditEntries.mockReturnValue([
        {
          id: "field-1",
          actorId: "user-1",
          label: "Renewal Date",
          type: "date",
          object: "deal",
          occurredAt: "2026-07-01T10:00:00Z",
          auditRef: "audit#field1id-created",
          action: "added",
        },
      ]);

      mockResolveMemberName.mockReturnValue("Alice Smith");

      render(
        <CustomFieldAuditCard
          fields={mockFields.slice(0, 1)}
          members={mockMembers}
          role="admin"
          isLoading={false}
          isError={false}
        />
      );

      expect(mockDeriveAuditEntries).toHaveBeenCalledWith(mockFields.slice(0, 1));

      const ul = screen.getByRole("list");
      expect(ul).toBeInTheDocument();
      expect(ul.querySelectorAll("li")).toHaveLength(1);
    });
  });
});
