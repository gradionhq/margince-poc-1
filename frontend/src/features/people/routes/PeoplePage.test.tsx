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
      refetch: vi.fn(),
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

describe("PeoplePage toolbar and section label", () => {
  beforeEach(() => {
    mockUseAuthStore.mockReturnValue({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
      loading: false,
    });
    mockUsePeople.mockReturnValue({
      data: { data: [], page: { has_more: false } },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof peopleApi.usePeople>);
  });

  it("renders the section label 'Contacts we actually know'", () => {
    renderPeoplePage();
    expect(screen.getByText(/contacts we actually know/i)).toBeInTheDocument();
  });

  it("renders the Strength sort control", () => {
    renderPeoplePage();
    expect(
      screen.getByRole("button", { name: /sort by strength/i }),
    ).toBeInTheDocument();
  });

  it("renders the Filter control", () => {
    renderPeoplePage();
    expect(screen.getByText(/filter/i)).toBeInTheDocument();
  });

  it("renders the search input", () => {
    renderPeoplePage();
    expect(screen.getByPlaceholderText(/search contacts/i)).toBeInTheDocument();
  });

  it("renders the New contact button", () => {
    renderPeoplePage();
    expect(
      screen.getByRole("button", { name: /new contact/i }),
    ).toBeInTheDocument();
  });

  it("does not render a capture banner", () => {
    renderPeoplePage();
    expect(screen.queryByText(/capture banner/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/pending.*contact/i)).not.toBeInTheDocument();
  });
});
