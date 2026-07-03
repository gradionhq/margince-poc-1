import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RailIcon } from "./RailIcon.js";

describe("RailIcon", () => {
  it("renders a real glyph (not the icon-fallback) for a Forge-registered name", () => {
    render(<RailIcon name="Home" />);
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
  });

  it("renders a real glyph for a Lucide name Forge does NOT register (Building2)", () => {
    const { container } = render(<RailIcon name="Building2" />);
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("renders Target and CheckSquare without falling back", () => {
    const { container, rerender } = render(<RailIcon name="Target" />);
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
    rerender(<RailIcon name="CheckSquare" />);
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });
});
