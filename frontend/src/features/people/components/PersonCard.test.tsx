import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PersonCard } from "./PersonCard.js";

describe("PersonCard", () => {
  it("renders the person name", () => {
    render(<PersonCard name="Alice Müller" />);
    expect(screen.getByText(/Alice Müller/)).toBeInTheDocument();
  });

  it("renders email when provided", () => {
    render(<PersonCard name="Bob" email="bob@example.com" />);
    expect(screen.getByText(/bob@example\.com/)).toBeInTheDocument();
  });

  it("renders fallback text when email is absent", () => {
    render(<PersonCard name="Carol" />);
    expect(screen.getByText(/no email/)).toBeInTheDocument();
  });
});
