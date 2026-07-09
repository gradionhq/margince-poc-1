import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { VisibilityRail } from "./VisibilityRail.js";

describe("VisibilityRail", () => {
  it("explains record-level visibility without per-file ACLs", () => {
    render(<VisibilityRail />);

    expect(
      screen.getByTestId("attachments-visibility-rail"),
    ).toBeInTheDocument();
    expect(screen.getByText("Visibility")).toBeInTheDocument();
    expect(
      screen.getByText(
        /attachment visibility follows the parent record's rbac/i,
      ),
    ).toBeInTheDocument();
  });
});
