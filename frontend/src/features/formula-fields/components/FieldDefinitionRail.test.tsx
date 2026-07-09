import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FieldDefinitionRail } from "./FieldDefinitionRail.js";

describe("FieldDefinitionRail", () => {
  it("renders the formula SQL, dependencies, and computed provenance", () => {
    render(
      <FieldDefinitionRail
        field={{
          key: "open_pipeline",
          label: "Open pipeline",
          kind: "currency_minor",
          value_minor: 212000,
          formula_sql:
            "COALESCE(organization_open_pipeline_rollup.open_pipeline_minor_base, 0)",
          dependencies: [
            "organization_open_pipeline_rollup.open_pipeline_minor_base",
            "deal.amount_minor_base",
          ],
          computable: true,
        }}
      />,
    );

    expect(screen.getByText("Open pipeline")).toBeInTheDocument();
    expect(screen.getByText("computed:server")).toBeInTheDocument();
    expect(screen.getByTestId("formula-sql")).toHaveTextContent(
      "COALESCE(organization_open_pipeline_rollup.open_pipeline_minor_base, 0)",
    );
    expect(screen.getByTestId("formula-dependencies")).toHaveTextContent(
      "organization_open_pipeline_rollup.open_pipeline_minor_base",
    );
    expect(screen.getByTestId("formula-dependencies")).toHaveTextContent(
      "deal.amount_minor_base",
    );
    expect(
      screen.getByText(
        /authoring new formula logic is a reviewed source change, not a runtime builder/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/this needs the development path, not this screen/i),
    ).toBeInTheDocument();
  });
});
