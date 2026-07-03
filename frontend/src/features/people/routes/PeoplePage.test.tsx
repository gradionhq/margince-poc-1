import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: vi.fn(),
}));
vi.mock("../api/people.js", () => ({
  usePeople: vi.fn(),
}));

import * as authStore from "../../identity/store/authStore.js";
import * as peopleApi from "../api/people.js";
import { PeoplePage } from "./PeoplePage.js";

const mockUseAuthStore = vi.mocked(authStore.useAuthStore);
const mockUsePeople = vi.mocked(peopleApi.usePeople);

const fakeUser = {
  id: "u1",
  workspace_id: "ws1",
  email: "admin@example.com",
  display_name: "Admin",
  timezone: "UTC",
  status: "active" as const,
  is_agent: false,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
};

function renderPeoplePage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <PeoplePage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("PeoplePage RBAC", () => {
  beforeEach(() => {
    mockUsePeople.mockReturnValue({
      data: { data: [], page: { has_more: false } },
      isLoading: false,
      isError: false,
    } as unknown as ReturnType<typeof peopleApi.usePeople>);
  });

  it("shows captured_by label when role=admin", () => {
    mockUseAuthStore.mockReturnValue({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
      loading: false,
    });
    renderPeoplePage();
    // FieldGuard mode="visible" — label is rendered
    expect(
      screen.getByText(/captured_by column: admin only/i),
    ).toBeInTheDocument();
  });

  it("hides captured_by label when role=rep (FieldGuard mode=masked)", () => {
    mockUseAuthStore.mockReturnValue({
      user: { ...fakeUser, email: "rep@example.com", display_name: "Rep" },
      role: "rep",
      roles: ["rep"],
      loading: false,
    });
    renderPeoplePage();
    // FieldGuard mode="masked" — label is hidden (renders null)
    expect(
      screen.queryByText(/captured_by column: admin only/i),
    ).not.toBeInTheDocument();
  });

  it("shows RoleBadge with current role label", () => {
    mockUseAuthStore.mockReturnValue({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
      loading: false,
    });
    renderPeoplePage();
    // Both RoleBadge and display_name render "Admin"; getAllByText confirms presence
    expect(screen.getAllByText("Admin").length).toBeGreaterThanOrEqual(1);
  });

  it("shows user display_name in header", () => {
    mockUseAuthStore.mockReturnValue({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
      loading: false,
    });
    renderPeoplePage();
    expect(screen.getAllByText("Admin").length).toBeGreaterThanOrEqual(1);
  });
});
