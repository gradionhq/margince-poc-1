import { describe, expect, it } from "vitest";
import type {
  PageInfo,
  Person,
  PersonListResponse,
  Deal,
} from "./generated/index.js";
import type { components } from "./generated/crm.js";

describe("PersonListResponse contract compliance", () => {
  it("data is an array of Person", () => {
    const resp: PersonListResponse = {
      data: [],
      page: { has_more: false },
    };
    expect(Array.isArray(resp.data)).toBe(true);
  });

  it("page has has_more boolean", () => {
    const page: PageInfo = { has_more: true };
    expect(typeof page.has_more).toBe("boolean");
  });

  it("page.next_cursor is optional string", () => {
    const withCursor: PageInfo = { has_more: true, next_cursor: "tok_abc" };
    expect(typeof withCursor.next_cursor).toBe("string");
    const withoutCursor: PageInfo = { has_more: false };
    expect(withoutCursor.next_cursor).toBeUndefined();
  });
});

describe("Person contract compliance", () => {
  const minimalPerson: Person = {
    id: "00000000-0000-0000-0000-000000000001",
    workspace_id: "00000000-0000-0000-0000-000000000002",
    full_name: "Alice Müller",
    source: "test",
    captured_by: "human:test",
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-01-01T00:00:00Z",
  };

  it("required string fields are present", () => {
    expect(typeof minimalPerson.full_name).toBe("string");
    expect(typeof minimalPerson.source).toBe("string");
    expect(typeof minimalPerson.captured_by).toBe("string");
  });

  it("created_at and updated_at are strings", () => {
    expect(typeof minimalPerson.created_at).toBe("string");
    expect(typeof minimalPerson.updated_at).toBe("string");
  });

  it("emails is an optional array", () => {
    const withEmail: Person = {
      ...minimalPerson,
      emails: [
        {
          id: "00000000-0000-0000-0000-000000000003",
          email: "alice@example.com",
          email_type: "work",
          is_primary: true,
          position: 0,
          source: "test",
          captured_by: "human:test",
        },
      ],
    };
    expect(Array.isArray(withEmail.emails)).toBe(true);
    expect(withEmail.emails?.[0].email).toBe("alice@example.com");
  });

  it("phones is optional and absent when not set", () => {
    expect(minimalPerson.phones).toBeUndefined();
  });
});

describe("restoreDeal contract compliance", () => {
  it("200 response schema is Deal with a nullable archived_at", () => {
    const restored: Deal = {
      id: "00000000-0000-0000-0000-000000000010",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      name: "Acme — restored deal",
      pipeline_id: "00000000-0000-0000-0000-000000000020",
      stage_id: "00000000-0000-0000-0000-000000000021",
      status: "open",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      archived_at: null,
    };
    expect(restored.archived_at).toBeNull();
  });
});

describe("Deal row-extension contract compliance (DEAL-EXT-2)", () => {
  it("stage_entered_at is an optional nullable date-time", () => {
    const deal: Deal = {
      id: "00000000-0000-0000-0000-000000000010",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      name: "Acme — expansion",
      pipeline_id: "00000000-0000-0000-0000-000000000020",
      stage_id: "00000000-0000-0000-0000-000000000021",
      status: "open",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      stage_entered_at: "2025-06-01T09:00:00Z",
      stakeholder_count: 3,
    };
    expect(typeof deal.stage_entered_at).toBe("string");
    expect(typeof deal.stakeholder_count).toBe("number");
  });

  it("both fields are optional (absent on a minimal Deal)", () => {
    const minimal: Deal = {
      id: "00000000-0000-0000-0000-000000000011",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      name: "Acme — bare",
      pipeline_id: "00000000-0000-0000-0000-000000000020",
      stage_id: "00000000-0000-0000-0000-000000000021",
      status: "open",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
    };
    expect(minimal.stage_entered_at).toBeUndefined();
    expect(minimal.stakeholder_count).toBeUndefined();
  });
});

describe("DealDetail contract compliance (DEAL-EXT-3)", () => {
  it("nests deal, stakeholders, and timeline in one composite read", () => {
    const detail: components["schemas"]["DealDetail"] = {
      deal: {
        id: "00000000-0000-0000-0000-000000000010",
        workspace_id: "00000000-0000-0000-0000-000000000002",
        name: "Acme — expansion",
        pipeline_id: "00000000-0000-0000-0000-000000000020",
        stage_id: "00000000-0000-0000-0000-000000000021",
        status: "open",
        source: "test",
        captured_by: "human:test",
        created_at: "2025-01-01T00:00:00Z",
        updated_at: "2025-01-01T00:00:00Z",
      },
      stakeholders: [
        {
          id: "00000000-0000-0000-0000-000000000030",
          workspace_id: "00000000-0000-0000-0000-000000000002",
          kind: "deal_stakeholder",
          role: "champion",
          source: "test",
          captured_by: "human:test",
          created_at: "2025-01-01T00:00:00Z",
          updated_at: "2025-01-01T00:00:00Z",
        },
      ],
      timeline: [
        {
          id: "00000000-0000-0000-0000-000000000040",
          kind: "email",
          subject: "Re: proposal",
          occurred_at: "2025-06-01T09:00:00Z",
        },
      ],
    };
    expect(detail.deal.name).toBe("Acme — expansion");
    expect(detail.stakeholders).toHaveLength(1);
    expect(detail.stakeholders[0].role).toBe("champion");
    expect(detail.timeline).toHaveLength(1);
    expect(detail.timeline[0].kind).toBe("email");
  });
});
