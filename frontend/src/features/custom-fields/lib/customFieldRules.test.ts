import { describe, expect, it } from "vitest";
import type {
  CustomField,
  Member,
} from "../../../lib/api-client/generated/index.js";
import {
  buildApiKey,
  buildDdlPreview,
  deriveAuditEntries,
  detectStructuralWord,
  OBJECT_CHIPS,
  resolveMemberName,
  slugify,
} from "./customFieldRules.js";

describe("customFieldRules", () => {
  describe("OBJECT_CHIPS", () => {
    it("exports exactly 5 entries with correct value/label pairs", () => {
      expect(OBJECT_CHIPS).toHaveLength(5);
      expect(OBJECT_CHIPS).toEqual([
        { value: "deal", label: "Deal" },
        { value: "organization", label: "Company" },
        { value: "person", label: "Contact" },
        { value: "lead", label: "Lead" },
        { value: "activity", label: "Activity" },
      ]);
    });
  });

  describe("slugify", () => {
    it('converts "Renewal date" to "renewal_date"', () => {
      expect(slugify("Renewal date")).toBe("renewal_date");
    });

    it('converts "  Budget   ceiling!" to "budget_ceiling"', () => {
      expect(slugify("  Budget   ceiling!")).toBe("budget_ceiling");
    });

    it('converts "" to ""', () => {
      expect(slugify("")).toBe("");
    });

    it("handles multiple spaces and special chars", () => {
      expect(slugify("Hello  @@ World")).toBe("hello_world");
    });

    it("trims leading and trailing underscores", () => {
      expect(slugify("!!!hello!!!")).toBe("hello");
    });
  });

  describe("buildApiKey", () => {
    it("returns empty string when slug is empty", () => {
      expect(buildApiKey("deal", "")).toBe("");
    });

    it('returns "deal.cf_renewal_date" for deal and "renewal_date" slug', () => {
      expect(buildApiKey("deal", "renewal_date")).toBe("deal.cf_renewal_date");
    });

    it('returns "organization.cf_budget_code" for organization and "budget_code" slug', () => {
      expect(buildApiKey("organization", "budget_code")).toBe(
        "organization.cf_budget_code",
      );
    });
  });

  describe("buildDdlPreview", () => {
    it("returns correct DDL format for deal and text type", () => {
      expect(buildDdlPreview("deal", "renewal_date", "text")).toBe(
        "ALTER deal ADD COLUMN cf_renewal_date (text) · backfilled NULL · reversible",
      );
    });

    it("returns correct DDL format for organization and currency type", () => {
      expect(buildDdlPreview("organization", "budget_code", "currency")).toBe(
        "ALTER organization ADD COLUMN cf_budget_code (currency) · backfilled NULL · reversible",
      );
    });
  });

  describe("detectStructuralWord", () => {
    it('returns "object" when label contains "object"', () => {
      expect(detectStructuralWord("This is an object field")).toBe("object");
    });

    it('returns "relationship" when label contains "relationship"', () => {
      expect(detectStructuralWord("Main relationship")).toBe("relationship");
    });

    it('returns "link to" when label contains "link to"', () => {
      expect(detectStructuralWord("Link To account")).toBe("link to");
    });

    it('returns "lookup to" when label contains "lookup to"', () => {
      expect(detectStructuralWord("Lookup to record")).toBe("lookup to");
    });

    it('returns null for clean label "Renewal date"', () => {
      expect(detectStructuralWord("Renewal date")).toBeNull();
    });

    it("performs case-insensitive matching", () => {
      expect(detectStructuralWord("OBJECT FIELD")).toBe("object");
      expect(detectStructuralWord("Link To account")).toBe("link to");
    });
  });

  describe("resolveMemberName", () => {
    const members: Member[] = [
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

    it("returns display_name when user_id matches", () => {
      expect(resolveMemberName(members, "user-1")).toBe("Alice Smith");
      expect(resolveMemberName(members, "user-2")).toBe("Bob Johnson");
    });

    it('returns "Unknown" when no member matches', () => {
      expect(resolveMemberName(members, "user-999")).toBe("Unknown");
    });

    it("returns Unknown for empty array", () => {
      expect(resolveMemberName([], "user-1")).toBe("Unknown");
    });
  });

  describe("deriveAuditEntries", () => {
    const activeField: CustomField = {
      id: "field-001",
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

    const retiredField: CustomField = {
      id: "field-002",
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
    };

    it("yields one 'added' entry for active field", () => {
      const entries = deriveAuditEntries([activeField]);
      expect(entries).toHaveLength(1);
      expect(entries[0]).toEqual({
        id: "field-001",
        actorId: "user-1",
        label: "Renewal Date",
        type: "date",
        object: "deal",
        occurredAt: "2026-07-01T10:00:00Z",
        auditRef: "audit#field00-created",
        action: "added",
      });
    });

    it("yields two entries (added + retired) for retired field", () => {
      const entries = deriveAuditEntries([retiredField]);
      expect(entries).toHaveLength(2);
      expect(entries[0].action).toBe("retired");
      expect(entries[0].occurredAt).toBe("2026-07-05T14:30:00Z");
      expect(entries[0].auditRef).toBe("audit#field00-retired");
      expect(entries[1].action).toBe("added");
      expect(entries[1].occurredAt).toBe("2026-06-01T10:00:00Z");
      expect(entries[1].auditRef).toBe("audit#field00-created");
    });

    it("returns empty array for empty fields", () => {
      expect(deriveAuditEntries([])).toEqual([]);
    });

    it("sorts mixed entries by occurredAt descending (newest first)", () => {
      const field1: CustomField = {
        ...activeField,
        id: "field-1",
        created_at: "2026-07-01T08:00:00Z",
        updated_at: "2026-07-01T08:00:00Z",
      };
      const field2: CustomField = {
        ...retiredField,
        id: "field-2",
        created_at: "2026-07-02T10:00:00Z",
        updated_at: "2026-07-03T14:00:00Z",
      };

      const entries = deriveAuditEntries([field1, field2]);
      // field2 retired at 2026-07-03T14:00:00Z should be first
      expect(entries[0].occurredAt).toBe("2026-07-03T14:00:00Z");
      expect(entries[0].action).toBe("retired");
      // field2 added at 2026-07-02T10:00:00Z should be second
      expect(entries[1].occurredAt).toBe("2026-07-02T10:00:00Z");
      expect(entries[1].action).toBe("added");
      // field1 added at 2026-07-01T08:00:00Z should be last
      expect(entries[2].occurredAt).toBe("2026-07-01T08:00:00Z");
    });
  });
});
