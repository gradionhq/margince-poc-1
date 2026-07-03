import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../features/identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: "u1", display_name: "Admin" },
    role: "admin",
    roles: ["admin"],
    loading: false,
  }),
}));
vi.mock("../features/people/api/people.js", () => ({
  usePeople: () => ({
    data: { data: [] },
    isLoading: false,
    isError: false,
  }),
}));

import App from "./App.js";

function renderApp(initialEntry: string) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("App routes", () => {
  it("mounts PeoplePage at /people", () => {
    renderApp("/people");
    expect(screen.getAllByText("People").length).toBeGreaterThan(0);
  });

  it("mounts ShellPlaceholderPage for rail routes without a real feature", () => {
    renderApp("/reports");
    expect(screen.getByText(/reports — coming soon/i)).toBeInTheDocument();
  });
});
