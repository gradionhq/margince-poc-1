import { describe, expect, it } from "vitest";
import type {
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
    id: "00000000-0000-0000-0000-000000000001" as any,
    workspace_id: "00000000-0000-0000-0000-000000000002" as any,
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
          id: "00000000-0000-0000-0000-000000000003" as any,
          email: "alice@example.com" as any,
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
