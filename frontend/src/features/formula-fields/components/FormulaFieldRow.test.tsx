import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { FormulaFieldRow } from "./FormulaFieldRow.js";

type ComputedField = components["schemas"]["ComputedField"];

describe("FormulaFieldRow", () => {
  it("renders a computable row with a derived badge, lock icon, and formatted value", () => {
    const field = {
      key: "open_pipeline",
      label: "Open pipeline",
      kind: "currency_minor",
      value_minor: 212000,
      formula_sql: "sum(deal.amount_minor_base)",
      dependencies: ["deal.amount_minor", "deal.fx_rate_to_base"],
      computable: true,
    } satisfies ComputedField;

    render(<FormulaFieldRow field={field} />);

    expect(screen.getByText("Open pipeline")).toBeInTheDocument();
    expect(screen.getByText("Σ Derived")).toBeInTheDocument();
    expect(
      screen.getByTitle("Read-only — computed, cannot be edited"),
    ).toBeInTheDocument();
    expect(screen.getByText("2.120,00 €")).toBeInTheDocument();
  });

  it("renders the not-computable label and omits the lock icon when the field is not built yet", () => {
    const field = {
      key: "customer_age",
      label: "Customer age",
      kind: "duration_months",
      value: null,
      formula_sql: "",
      dependencies: [],
      computable: false,
      reason: "not_yet_built",
    } satisfies ComputedField;

    render(<FormulaFieldRow field={field} />);

    expect(screen.getByText("Customer age")).toBeInTheDocument();
    expect(screen.getByText("Formula unavailable")).toBeInTheDocument();
    expect(
      screen.queryByTitle("Read-only — computed, cannot be edited"),
    ).not.toBeInTheDocument();
  });
});
