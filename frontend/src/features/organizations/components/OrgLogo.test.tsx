import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OrgLogo } from "./OrgLogo.js";

describe("OrgLogo", () => {
  it("renders a deterministic monogram for the same name every time (no logo field exists yet)", () => {
    const { container: first } = render(<OrgLogo name="Acme Inc" />);
    const { container: second } = render(<OrgLogo name="Acme Inc" />);
    expect(first.innerHTML).toBe(second.innerHTML);
  });
  it("never renders a broken <img> — no src means monogram, not an empty img tag", () => {
    render(<OrgLogo name="Acme Inc" />);
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
