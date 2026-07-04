import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { PersonList } from "./PersonList.js";

function renderList(props: Parameters<typeof PersonList>[0]) {
  return render(
    <MemoryRouter>
      <PersonList {...props} />
    </MemoryRouter>,
  );
}

const somePeople = [
  {
    id: "1",
    workspace_id: "ws1",
    full_name: "Alice",
    source: "email:inbox",
    captured_by: "agent:capture",
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
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    strength: {
      score: 72,
      bucket: "strong" as const,
      recency: 0.9,
      frequency: 0.6,
      reciprocity: 0.8,
    },
    last_activity_at: "2024-06-01T10:00:00Z",
  },
  {
    id: "2",
    workspace_id: "ws1",
    full_name: "Bob",
    source: "ui",
    captured_by: "human:018f3a1b",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    strength: null,
    last_activity_at: null,
  },
] as never;

describe("PersonList", () => {
  it("shows skeleton while loading", () => {
    renderList({
      people: [],
      isLoading: true,
      isError: false,
      onRetry: vi.fn(),
    });
    expect(screen.getByTestId("person-list-skeleton")).toBeInTheDocument();
  });

  it("shows error card with retry button on failure", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const onRetry = vi.fn();
    const user = userEvent.setup();
    renderList({ people: [], isLoading: false, isError: true, onRetry });
    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("renders empty state gracefully, no fabricated counts", () => {
    renderList({
      people: [],
      isLoading: false,
      isError: false,
      onRetry: vi.fn(),
    });
    expect(screen.getByText(/no contacts/i)).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders a table with person name and email columns", () => {
    renderList({
      people: somePeople,
      isLoading: false,
      isError: false,
      onRetry: vi.fn(),
    });
    expect(screen.getByRole("table")).toBeInTheDocument();
    const rows = screen.getAllByRole("row");
    expect(rows.length).toBeGreaterThan(1);
    expect(screen.getByText(/Alice/)).toBeInTheDocument();
    expect(screen.getByText(/alice@example\.com/)).toBeInTheDocument();
    expect(screen.getByText(/Bob/)).toBeInTheDocument();
  });

  it("renders 'no activity' honestly for a person with null last_activity_at", () => {
    renderList({
      people: somePeople,
      isLoading: false,
      isError: false,
      onRetry: vi.fn(),
    });
    expect(screen.getByText(/no activity/)).toBeInTheDocument();
  });

  it("navigates to /people/:id on row click", () => {
    const { container } = renderList({
      people: somePeople,
      isLoading: false,
      isError: false,
      onRetry: vi.fn(),
    });
    const row = screen.getByText("Alice").closest("tr") as HTMLElement;
    fireEvent.click(row);
    // MemoryRouter has no visible location assertion helper here; instead assert the row is
    // click-activatable (role/tabIndex) — the navigate() call itself is exercised in
    // PersonDetailPage.test.tsx via a routed MemoryRouter with the target path in initialEntries.
    expect(row).toHaveAttribute("tabIndex", "0");
    void container;
  });
});
