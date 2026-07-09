import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { FormulaFieldsPanel } from "../index.js";

type ComputedField = components["schemas"]["ComputedField"];
type Organization = components["schemas"]["Organization"];

const computedFields = [
  {
    key: "open_pipeline",
    label: "Open pipeline",
    kind: "currency_minor",
    value_minor: 212000,
    formula_sql: "COALESCE(organization_open_pipeline_rollup.open_pipeline_minor_base, 0)",
    dependencies: [
      "organization_open_pipeline_rollup.open_pipeline_minor_base",
      "deal.amount_minor_base",
    ],
    computable: true,
  },
  {
    key: "weighted_pipeline",
    label: "Weighted pipeline",
    kind: "currency_minor",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "customer_age",
    label: "Customer age",
    kind: "duration_months",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "net_revenue_retention",
    label: "Net revenue retention",
    kind: "percent",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "blended_gross_margin",
    label: "Blended gross margin",
    kind: "percent",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
] satisfies ComputedField[];

function makeOrg(overrides: Partial<Organization> = {}): Organization {
  return {
    id: "org-1",
    workspace_id: "ws-1",
    display_name: "Acme Corp",
    source: "manual",
    captured_by: "human:u1",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
    ...overrides,
  };
}

describe("FormulaFieldsPanel", () => {
  it("returns null when computed_fields is absent", () => {
    const { container } = render(
      <FormulaFieldsPanel org={makeOrg()} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("shows the empty state when computed_fields is an empty array", () => {
    render(<FormulaFieldsPanel org={makeOrg({ computed_fields: [] })} />);

    expect(screen.getByTestId("formula-fields-panel")).toBeInTheDocument();
    expect(screen.getByText("No computed fields yet.")).toBeInTheDocument();
  });

  it("renders the computed table, right rail, and definition rail for populated data", () => {
    render(
      <FormulaFieldsPanel org={makeOrg({ computed_fields: computedFields })} />,
    );

    expect(screen.getByText(/formula fields/i)).toBeInTheDocument();
    expect(screen.getByTestId("formula-fields-panel")).toHaveTextContent(
      /read-only computed/i,
    );
    expect(screen.getByText(/recomputes on every write/i)).toBeInTheDocument();
    expect(screen.getByTestId("formula-field-row-open_pipeline")).toBeInTheDocument();
    expect(screen.getByTestId("formula-field-row-weighted_pipeline")).toBeInTheDocument();
    expect(screen.getByText("See it recompute")).toBeInTheDocument();
    expect(screen.getByText("AI-proposed")).toBeInTheDocument();
    expect(screen.getByText("computed:server")).toBeInTheDocument();
  });
});
