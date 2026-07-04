import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FactorBar } from "./FactorBar.js";

describe("FactorBar", () => {
  it("renders the label and rounded percentage", () => {
    render(<FactorBar label="Recency" value={0.73} />);
    expect(screen.getByText("Recency")).toBeInTheDocument();
    expect(screen.getByText("73")).toBeInTheDocument();
  });
});
