import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TopBar } from "./TopBar.js";

describe("TopBar", () => {
  it("renders a 56px (h-14) elevated bar with a subtle bottom border", () => {
    render(<TopBar title="Contacts" />);
    const bar = screen.getByTestId("top-bar");
    expect(bar.className).toContain("h-14"); // 14 * 4px = 56px
    expect(bar.className).toContain("bg-gf-elevated");
    expect(bar.className).toContain("border-gf-subtle");
  });

  it("renders no contextual actions at cold start (empty action area)", () => {
    render(<TopBar title="Home" />);
    const actions = screen.getByTestId("top-bar-actions");
    expect(actions.children).toHaveLength(0);
  });

  it("renders only the actions true for the current state", () => {
    render(
      <TopBar
        title="Deals"
        actions={[
          { id: "new", render: () => <button type="button">New deal</button> },
        ]}
      />,
    );
    const actions = screen.getByTestId("top-bar-actions");
    expect(actions.children).toHaveLength(1);
    expect(screen.getByRole("button", { name: "New deal" })).not.toBeNull();
  });
});
