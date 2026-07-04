import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SourceChip } from "./SourceChip.js";

describe("SourceChip", () => {
  it("shows a connector chip for a connector-captured source", () => {
    render(
      <SourceChip source="email:CAF=abc@mail" capturedBy="agent:capture" />,
    );
    expect(screen.getByText(/connector/i)).toBeInTheDocument();
  });
  it("shows a typed-by-you chip for a human-captured source", () => {
    render(<SourceChip source="ui" capturedBy="human:018f3a1b" />);
    expect(screen.getByText(/typed by you/i)).toBeInTheDocument();
  });
});
