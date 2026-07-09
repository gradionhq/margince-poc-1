import { describe, expect, it } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { formatComputedFieldValue } from "./format.js";

type ComputedField = components["schemas"]["ComputedField"];

describe("formatComputedFieldValue", () => {
  it("formats computable currency, count, duration, and percent values", () => {
    const currencyField = {
      key: "open_pipeline",
      label: "Open pipeline",
      kind: "currency_minor",
      value_minor: 123456,
      formula_sql: "sum(...)",
      dependencies: [],
      computable: true,
    } satisfies ComputedField;
    const countField = {
      key: "customer_count",
      label: "Customers",
      kind: "count",
      value: 42,
      formula_sql: "count(...)",
      dependencies: [],
      computable: true,
    } satisfies ComputedField;
    const durationField = {
      key: "customer_age",
      label: "Customer age",
      kind: "duration_months",
      value: 18,
      formula_sql: "age(...)",
      dependencies: [],
      computable: true,
    } satisfies ComputedField;
    const percentField = {
      key: "gross_margin",
      label: "Blended gross margin",
      kind: "percent",
      value: 37.5,
      formula_sql: "margin(...)",
      dependencies: [],
      computable: true,
    } satisfies ComputedField;

    expect(formatComputedFieldValue(currencyField)).toBe("1.234,56 €");
    expect(formatComputedFieldValue(countField)).toBe("42");
    expect(formatComputedFieldValue(durationField)).toBe("18 mo");
    expect(formatComputedFieldValue(percentField)).toBe("37.5%");
  });

  it("returns the honest not-computable label even when the field carries a value", () => {
    const field = {
      key: "weighted_pipeline",
      label: "Weighted pipeline",
      kind: "currency_minor",
      value_minor: 999999,
      formula_sql: "",
      dependencies: [],
      computable: false,
      reason: "not_yet_built",
    } satisfies ComputedField;

    expect(formatComputedFieldValue(field)).toBe("Not computable yet");
  });
});
