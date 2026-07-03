import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FieldGuard } from "./FieldGuard.js";

describe("FieldGuard", () => {
  it("renders children when visible", () => {
    render(
      <FieldGuard mode="visible">
        <span>secret</span>
      </FieldGuard>,
    );
    expect(screen.getByText("secret")).toBeInTheDocument();
  });
  it("renders a visible mask token (not the value, not nothing) when masked", () => {
    render(
      <FieldGuard mode="masked">
        <span>secret</span>
      </FieldGuard>,
    );
    // The underlying value is withheld...
    expect(screen.queryByText("secret")).toBeNull();
    // ...but the field is visibly masked, distinguishable from "no data".
    const mask = screen.getByLabelText("Masked value");
    expect(mask).toBeInTheDocument();
    expect(mask.textContent).toBe("••••");
  });
  it("renders readonly children with aria-readonly", () => {
    render(
      <FieldGuard mode="readonly">
        <span>secret</span>
      </FieldGuard>,
    );
    const child = screen.getByText("secret");
    expect(child).toBeInTheDocument();
    // The readonly wrapper marks the field non-editable for assistive tech.
    const wrapper = child.parentElement;
    expect(wrapper).toHaveAttribute("aria-readonly", "true");
  });
});
