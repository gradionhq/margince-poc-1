// biome-ignore-all lint/a11y/useValidAriaRole: `role` is a domain prop of RoleBadge, not the ARIA role attribute.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RoleBadge } from "./RoleBadge.js";

describe("RoleBadge", () => {
  it("renders admin label", () => {
    render(<RoleBadge role="admin" />);
    expect(screen.getByText("Admin")).toBeInTheDocument();
  });
  it("renders read_only label", () => {
    render(<RoleBadge role="read_only" />);
    expect(screen.getByText("Read Only")).toBeInTheDocument();
  });
  it("falls back to raw key for unknown role", () => {
    render(<RoleBadge role="superadmin" />);
    expect(screen.getByText("superadmin")).toBeInTheDocument();
  });
});
