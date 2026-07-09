import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ScanStatusChip } from "./ScanStatusChip.js";

describe("ScanStatusChip", () => {
  it("renders scanning as a neutral chip with a running dot", () => {
    render(<ScanStatusChip scanStatus="scanning" />);

    expect(screen.getByText("Scanning…")).toBeInTheDocument();
    expect(screen.getByTestId("status-dot")).toHaveAttribute(
      "data-state",
      "running",
    );
    expect(screen.getByTestId("chip")).toHaveAttribute("data-variant", "neutral");
  });

  it("renders clean as a success chip", () => {
    render(<ScanStatusChip scanStatus="clean" />);

    expect(screen.getByText("Clean")).toBeInTheDocument();
    expect(screen.getByTestId("status-dot")).toHaveAttribute(
      "data-state",
      "success",
    );
    expect(screen.getByTestId("chip")).toHaveAttribute("data-variant", "success");
  });

  it("renders blocked as a danger chip", () => {
    render(<ScanStatusChip scanStatus="blocked" />);

    expect(screen.getByText("Blocked")).toBeInTheDocument();
    expect(screen.getByTestId("status-dot")).toHaveAttribute(
      "data-state",
      "error",
    );
    expect(screen.getByTestId("chip")).toHaveAttribute("data-variant", "danger");
  });
});
