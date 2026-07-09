import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { ExplainBox } from "./ExplainBox.js";

type ComputedField = components["schemas"]["ComputedField"];

describe("ExplainBox", () => {
  it("opens and closes the computable formula explanation", async () => {
    const field = {
      key: "open_pipeline",
      label: "Open pipeline",
      kind: "currency_minor",
      value_minor: 212000,
      formula_sql: "sum(deal.amount_minor_base)",
      dependencies: ["deal.amount_minor", "deal.fx_rate_to_base"],
      computable: true,
    } satisfies ComputedField;

    render(<ExplainBox field={field} />);

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /explain this number/i }));
    expect(
      screen.getByTestId("formula-field-explain-open_pipeline"),
    ).toBeInTheDocument();
    expect(screen.getByText("Open pipeline")).toBeInTheDocument();
    expect(
      screen.getByText("sum(deal.amount_minor_base)"),
    ).toBeInTheDocument();
    expect(screen.getByText("deal.amount_minor")).toBeInTheDocument();
    expect(screen.getByText("2.120,00 €")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /explain this number/i }));
    expect(
      screen.queryByTestId("formula-field-explain-open_pipeline"),
    ).not.toBeInTheDocument();
  });

  it("shows the not-computable reason instead of a formula result", async () => {
    const field = {
      key: "weighted_pipeline",
      label: "Weighted pipeline",
      kind: "currency_minor",
      formula_sql: "",
      dependencies: [],
      computable: false,
      reason: "not_yet_built",
    } satisfies ComputedField;

    render(<ExplainBox field={field} />);

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /explain this number/i }));
    expect(
      screen.getByTestId("formula-field-explain-weighted_pipeline"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Not computable yet — not_yet_built"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/formula/i)).not.toBeInTheDocument();
  });
});
