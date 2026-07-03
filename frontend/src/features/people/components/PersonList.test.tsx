import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PersonList } from "./PersonList.js";

const somePeople = [
  {
    id: "1",
    full_name: "Alice",
    emails: [
      {
        id: "e1",
        email: "alice@example.com",
        email_type: "work" as const,
        is_primary: true,
        position: 0,
        source: "manual",
        captured_by: "u1",
      },
    ],
  },
  { id: "2", full_name: "Bob" },
];

describe("PersonList", () => {
  it("shows loading text while fetching", () => {
    render(<PersonList people={[]} isLoading={true} isError={false} />);
    expect(screen.getByText(/Loading/)).toBeInTheDocument();
  });

  it("shows error text on failure", () => {
    render(<PersonList people={[]} isLoading={false} isError={true} />);
    expect(screen.getByText(/Failed to load/)).toBeInTheDocument();
  });

  it("renders empty list gracefully (no crash, heading visible)", () => {
    render(<PersonList people={[]} isLoading={false} isError={false} />);
    expect(screen.getByRole("heading", { name: /People/ })).toBeInTheDocument();
    expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
  });

  it("renders each person name and email", () => {
    render(
      <PersonList people={somePeople} isLoading={false} isError={false} />,
    );
    expect(screen.getByText(/Alice/)).toBeInTheDocument();
    expect(screen.getByText(/alice@example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/Bob/)).toBeInTheDocument();
    expect(screen.getAllByText(/no email/)).toHaveLength(1);
  });
});
