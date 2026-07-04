import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/person.js", () => ({
  useOrganizationName: vi.fn(() => ({ data: undefined, isLoading: false })),
  useUpdatePerson: vi.fn(() => ({
    mutate: vi.fn(),
    isPending: false,
    error: null,
  })),
}));

import { PersonHeader } from "./PersonHeader.js";

const basePerson = {
  id: "p1",
  full_name: "Alice Johnson",
  title: "VP Sales",
  source: "manual",
  captured_by: "human:u1",
  emails: [
    {
      id: "e1",
      email: "alice@example.com",
      email_type: "work" as const,
      is_primary: true,
      position: 0,
      source: "email:inbox",
      captured_by: "agent:capture",
      created_at: "2026-05-02T00:00:00Z",
    },
  ],
  phones: [],
  relationships: [
    {
      id: "r1",
      kind: "employment" as const,
      organization_id: "o1",
      is_current_primary: true,
      source: "manual",
      captured_by: "human:u1",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  ],
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

function renderHeader(person = basePerson) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PersonHeader person={person as never} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PersonHeader", () => {
  it("shows full_name, title, and a provenance chip+date per contact field (AC-person-1)", () => {
    renderHeader();
    expect(screen.getByText("Alice Johnson")).toBeInTheDocument();
    expect(screen.getByText(/VP Sales/)).toBeInTheDocument();
    expect(screen.getByText("alice@example.com")).toBeInTheDocument();
    expect(screen.getByText(/connector/i)).toBeInTheDocument();
    expect(screen.getByText(/2026-05-02/)).toBeInTheDocument();
  });

  it("links the company to /companies/{organization_id} (known gap: route doesn't exist yet)", () => {
    renderHeader();
    const link = screen.getByTestId("company-link");
    expect(link).toHaveAttribute("href", "/companies/o1");
  });

  it("renders the email-draft affordance disabled-honest, never a live send button (PILOT-EXCLUDED)", () => {
    renderHeader();
    const draftBtn = screen.getByRole("button", { name: /draft email/i });
    expect(draftBtn).toBeDisabled();
    expect(screen.getByText(/later chapter/i)).toBeInTheDocument();
  });

  it("Edit toggles inline inputs for full_name/title", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    renderHeader();
    await user.click(screen.getByRole("button", { name: /^edit$/i }));
    expect(screen.getByDisplayValue("Alice Johnson")).toBeInTheDocument();
    expect(screen.getByDisplayValue("VP Sales")).toBeInTheDocument();
  });
});
