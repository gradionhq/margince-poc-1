import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StrengthCell } from "./StrengthCell.js";

describe("StrengthCell", () => {
  it("renders an honest no-signal state, never a fabricated 0 score", () => {
    render(<StrengthCell noSignalYet />);
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("renders the integer score and PO-N-BUCKETS color bucket", () => {
    render(
      <StrengthCell
        score={72}
        bucket="strong"
        recency={0.9}
        frequency={0.6}
        reciprocity={0.8}
      />,
    );
    expect(screen.getByText("72")).toBeInTheDocument();
    expect(
      screen.getByText(/recency·frequency·reciprocity/),
    ).toBeInTheDocument();
  });

  it("hover popover shows the three labeled component bars and the non-black-box line", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    render(
      <StrengthCell
        score={72}
        bucket="strong"
        recency={0.9}
        frequency={0.6}
        reciprocity={0.8}
      />,
    );
    await user.hover(screen.getByTestId("strength-cell"));
    expect(
      await screen.findByText(/Relationship-strength 72/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Recency/)).toBeInTheDocument();
    expect(screen.getByText(/Frequency/)).toBeInTheDocument();
    expect(screen.getByText(/Reciprocity/)).toBeInTheDocument();
    expect(
      screen.getByText(
        /Computed from the captured timeline — never a black-box badge\./,
      ),
    ).toBeInTheDocument();
  });
});
