import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { FieldHistoryExplainBox } from "./FieldHistoryExplainBox.js";

describe("FieldHistoryExplainBox", () => {
  it("shows a 'computed server-side'-style provenance chip", () => {
    render(<FieldHistoryExplainBox grossMinor={17707200} currency="EUR" />);
    expect(screen.getByText(/never free-typed/i)).toBeInTheDocument();
  });

  it("AC-field-history-7: toggling reveals net + 19% MwSt. = gross, exact worked numbers", async () => {
    render(<FieldHistoryExplainBox grossMinor={17707200} currency="EUR" />);
    expect(
      screen.queryByTestId("field-history-explain-box-content"),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByText(/explain this number/i));
    const box = screen.getByTestId("field-history-explain-box-content");
    expect(box.textContent).toMatch(/148\.800,00/);
    expect(box.textContent).toMatch(/19%/);
    expect(box.textContent).toMatch(/28\.272,00/);
    expect(box.textContent).toMatch(/177\.072,00/);
  });
});
