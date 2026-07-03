import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LoginForm } from "./LoginForm.js";

describe("LoginForm", () => {
  it("renders email and password fields", () => {
    render(<LoginForm onSubmit={vi.fn()} />);
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it("shows validation error for empty email on submit", async () => {
    render(<LoginForm onSubmit={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));
    expect(await screen.findByText(/email is required/i)).toBeInTheDocument();
  });

  it("calls onSubmit with email + password when valid", () => {
    const onSubmit = vi.fn();
    render(<LoginForm onSubmit={onSubmit} />);
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: "admin@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "changeme" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));
    expect(onSubmit).toHaveBeenCalledWith({
      email: "admin@example.com",
      password: "changeme",
    });
  });

  it("shows submitting state when isSubmitting=true", () => {
    render(<LoginForm onSubmit={vi.fn()} isSubmitting />);
    expect(screen.getByRole("button", { name: /signing in/i })).toBeDisabled();
  });
});
