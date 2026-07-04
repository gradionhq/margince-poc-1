import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/person.js", () => ({
  usePersonDeals: vi.fn(() => ({ data: [], isLoading: false, isError: false })),
}));

import { PersonTabs } from "./PersonTabs.js";

function renderTabs(activities: unknown[] = []) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <PersonTabs personId="p1" activities={activities as never} />
    </QueryClientProvider>,
  );
}

describe("PersonTabs", () => {
  it("defaults to the Activity pane and renders only the active pane (AC-person-7)", () => {
    renderTabs([]);
    expect(screen.getByText(/no activity captured yet/i)).toBeInTheDocument();
    expect(screen.queryByText(/no deals for this person yet/i)).not.toBeInTheDocument();
  });

  it("switches to Deals, then Notes, on click", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    renderTabs([]);
    await user.click(screen.getByRole("tab", { name: /deals/i }));
    expect(screen.getByText(/no deals for this person yet/i)).toBeInTheDocument();
    await user.click(screen.getByRole("tab", { name: /notes/i }));
    expect(screen.getByText(/no notes yet/i)).toBeInTheDocument();
  });
});
