import { describe, expect, it } from "vitest";
import type {
  Deal,
  Organization,
  Relationship,
} from "../../../lib/api-client/generated/index.js";
import {
  formatLocation,
  formatWonLifetime,
  getEmploymentContactIds,
  isChampion,
  lastTouchAt,
  openAndWonDeals,
  primaryDomainUrl,
  stalledDeal,
} from "./orgSelectors.js";

const baseOrg: Organization = {
  id: "org1",
  workspace_id: "w1",
  display_name: "Acme Corp",
  source: "manual",
  captured_by: "human:u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-06-01T00:00:00Z",
};

function rel(overrides: Partial<Relationship>): Relationship {
  return {
    id: `r-${Math.random()}`,
    workspace_id: "w1",
    kind: "employment",
    is_current_primary: false,
    source: "manual",
    captured_by: "human:u1",
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

function deal(overrides: Partial<Deal>): Deal {
  return {
    id: `d-${Math.random()}`,
    workspace_id: "w1",
    name: "Deal",
    pipeline_id: "p1",
    stage_id: "s1",
    status: "open",
    source: "manual",
    captured_by: "human:u1",
    created_at: "",
    updated_at: "",
    ...overrides,
  };
}

describe("getEmploymentContactIds", () => {
  it("keeps only live employment edges for this org, deduped", () => {
    const org: Organization = {
      ...baseOrg,
      relationships: [
        rel({ kind: "employment", organization_id: "org1", person_id: "p1" }),
        rel({ kind: "employment", organization_id: "org1", person_id: "p1" }),
        rel({
          kind: "employment",
          organization_id: "org1",
          person_id: "p2",
          archived_at: "2026-01-01T00:00:00Z",
        }),
        rel({
          kind: "deal_stakeholder",
          organization_id: "org1",
          person_id: "p3",
        }),
      ],
    };
    expect(getEmploymentContactIds(org)).toEqual(["p1"]);
  });

  it("returns [] when relationships is absent (list-row shape)", () => {
    expect(getEmploymentContactIds(baseOrg)).toEqual([]);
  });
});

describe("isChampion (PO-N-CHAMPION)", () => {
  it("true when a deal_stakeholder/champion edge matches an open deal on this org", () => {
    const org: Organization = {
      ...baseOrg,
      deals: [deal({ id: "d1", status: "open" })],
      relationships: [
        rel({
          kind: "deal_stakeholder",
          person_id: "p1",
          deal_id: "d1",
          role: "champion",
        }),
      ],
    };
    expect(isChampion("p1", org)).toBe(true);
  });

  it("false when the matching deal is not open", () => {
    const org: Organization = {
      ...baseOrg,
      deals: [deal({ id: "d1", status: "won" })],
      relationships: [
        rel({
          kind: "deal_stakeholder",
          person_id: "p1",
          deal_id: "d1",
          role: "champion",
        }),
      ],
    };
    expect(isChampion("p1", org)).toBe(false);
  });

  it("false when role isn't the literal string champion", () => {
    const org: Organization = {
      ...baseOrg,
      deals: [deal({ id: "d1", status: "open" })],
      relationships: [
        rel({
          kind: "deal_stakeholder",
          person_id: "p1",
          deal_id: "d1",
          role: "economic_buyer",
        }),
      ],
    };
    expect(isChampion("p1", org)).toBe(false);
  });
});

describe("primaryDomainUrl", () => {
  it("prefers is_primary, falls back to first, honest null on none", () => {
    expect(
      primaryDomainUrl([
        { domain: "old.com", is_primary: false },
        { domain: "acme.com", is_primary: true },
      ]),
    ).toBe("https://acme.com");
    expect(primaryDomainUrl([{ domain: "only.com", is_primary: false }])).toBe(
      "https://only.com",
    );
    expect(primaryDomainUrl(undefined)).toBeNull();
    expect(primaryDomainUrl([])).toBeNull();
  });
});

describe("formatLocation", () => {
  it("composes city/country, omitting null parts", () => {
    expect(formatLocation({ city: "Berlin", country: "DE" })).toBe(
      "Berlin, DE",
    );
    expect(formatLocation({ city: null, country: "DE" })).toBe("DE");
    expect(formatLocation({ city: "Berlin", country: null })).toBe("Berlin");
    expect(formatLocation(null)).toBeNull();
    expect(formatLocation(undefined)).toBeNull();
  });
});

describe("openAndWonDeals", () => {
  it("excludes lost, keeps open and won", () => {
    const deals = [
      deal({ id: "d1", status: "open" }),
      deal({ id: "d2", status: "won" }),
      deal({ id: "d3", status: "lost" }),
    ];
    expect(openAndWonDeals(deals).map((d) => d.id)).toEqual(["d1", "d2"]);
  });
});

describe("formatWonLifetime", () => {
  it("sums per-currency, never mixes currencies into one total", () => {
    const deals = [
      deal({ status: "won", amount_minor: 100_000, currency: "EUR" }),
      deal({ status: "won", amount_minor: 50_000, currency: "EUR" }),
      deal({ status: "won", amount_minor: 20_000, currency: "USD" }),
      deal({ status: "open", amount_minor: 999_999, currency: "EUR" }),
    ];
    const out = formatWonLifetime(deals);
    expect(out).toContain("€");
    expect(out).toContain("$");
  });

  it("returns null when no won deals exist (honest, not a fabricated 0)", () => {
    expect(formatWonLifetime([deal({ status: "open" })])).toBeNull();
  });
});

describe("lastTouchAt", () => {
  it("prefers the most recent activity, falls back to updated_at", () => {
    const org: Organization = {
      ...baseOrg,
      activities: [
        { id: "a1", kind: "email", occurred_at: "2026-05-01T00:00:00Z" },
        { id: "a2", kind: "call", occurred_at: "2026-06-15T00:00:00Z" },
      ],
    };
    expect(lastTouchAt(org)).toBe("2026-06-15T00:00:00Z");
    expect(lastTouchAt(baseOrg)).toBe(baseOrg.updated_at);
  });
});

describe("stalledDeal", () => {
  it("returns the first deal actually flagged stalled, undefined otherwise", () => {
    const deals = [
      deal({ id: "d1", stalled: false }),
      deal({ id: "d2", stalled: true }),
    ];
    expect(stalledDeal(deals)?.id).toBe("d2");
    expect(stalledDeal([deal({ stalled: false })])).toBeUndefined();
  });
});
