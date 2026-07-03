import { describe, expect, it } from "vitest";
import type { components } from "./generated/crm.js";
import type {
  Deal,
  PageInfo,
  Person,
  PersonListResponse,
} from "./generated/index.js";

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

describe("Person.strength contract compliance (PO-EXT-1)", () => {
  it("carries score/bucket/recency/frequency/reciprocity, optional and nullable", () => {
    const withStrength: Person = {
      id: "00000000-0000-0000-0000-000000000001",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      full_name: "Alice Müller",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      strength: {
        score: 72,
        bucket: "strong",
        recency: 0.9,
        frequency: 0.6,
        reciprocity: 0.8,
      },
    };
    expect(withStrength.strength?.score).toBe(72);
    expect(withStrength.strength?.bucket).toBe("strong");

    const withoutStrength: Person = {
      id: "00000000-0000-0000-0000-000000000003",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      full_name: "Bob Null",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
    };
    expect(withoutStrength.strength).toBeUndefined();
  });
});

describe("PersonStrengthBreakdown contract compliance (PO-EXT-2)", () => {
  it("carries factor values plus contributing activity refs", () => {
    const breakdown: components["schemas"]["PersonStrengthBreakdown"] = {
      person_id: "00000000-0000-0000-0000-000000000001",
      score: 72,
      bucket: "strong",
      recency: 0.9,
      frequency: 0.6,
      reciprocity: 0.8,
      contributing_activities: [
        {
          id: "00000000-0000-0000-0000-000000000040",
          kind: "email",
          subject: "Re: proposal",
          occurred_at: "2025-06-01T09:00:00Z",
        },
      ],
    };
    expect(breakdown.bucket).toBe("strong");
    expect(breakdown.contributing_activities).toHaveLength(1);
    expect(breakdown.contributing_activities[0].kind).toBe("email");
  });
});

describe("Person/Organization 360 composite reads (PO-EXT-3)", () => {
  it("Person carries relationships/deals/activities in one round trip via getPerson's Person shape", () => {
    const person360: Person = {
      id: "00000000-0000-0000-0000-000000000001",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      full_name: "Alice Müller",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      relationships: [
        {
          id: "00000000-0000-0000-0000-000000000030",
          workspace_id: "00000000-0000-0000-0000-000000000002",
          kind: "employment",
          source: "test",
          captured_by: "human:test",
          created_at: "2025-01-01T00:00:00Z",
          updated_at: "2025-01-01T00:00:00Z",
        },
      ],
      deals: [],
      activities: [
        {
          id: "00000000-0000-0000-0000-000000000040",
          kind: "email",
          subject: "Re: proposal",
          occurred_at: "2025-06-01T09:00:00Z",
        },
      ],
    };
    expect(person360.relationships).toHaveLength(1);
    expect(person360.relationships?.[0].kind).toBe("employment");
    expect(person360.activities?.[0].kind).toBe("email");
  });

  it("Organization carries the same three composite arrays", () => {
    const org360: components["schemas"]["Organization"] = {
      id: "00000000-0000-0000-0000-000000000050",
      workspace_id: "00000000-0000-0000-0000-000000000002",
      display_name: "Acme Inc",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      relationships: [],
      deals: [],
      activities: [],
    };
    expect(org360.relationships).toEqual([]);
    expect(org360.deals).toEqual([]);
    expect(org360.activities).toEqual([]);
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
  it("flattens the deal's own fields (backward-compatible with the old Deal-typed getDeal response) alongside stakeholders and timeline in one composite read", () => {
    const detail: components["schemas"]["DealDetail"] = {
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
    expect(detail.name).toBe("Acme — expansion");
    expect(detail.stakeholders).toHaveLength(1);
    expect(detail.stakeholders[0].role).toBe("champion");
    expect(detail.timeline).toHaveLength(1);
    expect(detail.timeline[0].kind).toBe("email");
  });
});

describe("PipelineRollup contract compliance (DEAL-EXT-1)", () => {
  it("matches DEAL-FORM-2's shape plus per-stage and per-deal breakdowns", () => {
    const rollup: components["schemas"]["PipelineRollup"] = {
      pipeline_id: "00000000-0000-0000-0000-000000000020",
      unweighted_minor: 14_600_000,
      weighted_minor: 9_680_000,
      base_currency: "EUR",
      as_of_date: "2026-06-04",
      by_stage: [
        {
          stage_id: "00000000-0000-0000-0000-000000000021",
          unweighted_minor: 10_000_000,
          weighted_minor: 6_000_000,
          deal_count: 1,
        },
      ],
      breakdown: [
        {
          deal_id: "00000000-0000-0000-0000-000000000010",
          base_value: 10_000_000,
          win_probability: 60,
          weighted_value: 6_000_000,
        },
      ],
    };
    expect(rollup.unweighted_minor).toBe(14_600_000);
    expect(rollup.weighted_minor).toBe(9_680_000);
    expect(rollup.breakdown[0].weighted_value).toBe(6_000_000);
  });

  it("fx_rate_unavailable Problem carries currency + as_of in details", () => {
    const problem: components["schemas"]["Problem"] = {
      status: 422,
      code: "fx_rate_unavailable",
      details: { currency: "USD", as_of: "2026-06-04" },
    };
    expect(problem.code).toBe("fx_rate_unavailable");
    expect(problem.details?.currency).toBe("USD");
    expect(problem.details?.as_of).toBe("2026-06-04");
  });
});
