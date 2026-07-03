import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

// Control the store state per test
vi.mock("../features/identity/store/authStore.js", () => ({
  useAuthStore: vi.fn(),
}));

import * as authStore from "../features/identity/store/authStore.js";
import { ProtectedRoute } from "./ProtectedRoute.js";

const mockUseAuthStore = vi.mocked(authStore.useAuthStore);

function renderInRouter(initialPath: string) {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/login" element={<div>Login Page</div>} />
        <Route
          path="/people"
          element={
            <ProtectedRoute>
              <div>Protected Content</div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProtectedRoute", () => {
  it("shows loading indicator while auth is loading", () => {
    mockUseAuthStore.mockReturnValue({
      user: null,
      role: null,
      roles: [],
      loading: true,
    });
    renderInRouter("/people");
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });

  it("redirects to /login when user is null and not loading", () => {
    mockUseAuthStore.mockReturnValue({
      user: null,
      role: null,
      roles: [],
      loading: false,
    });
    renderInRouter("/people");
    expect(screen.getByText("Login Page")).toBeInTheDocument();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });

  it("renders children when user is set", () => {
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
    mockUseAuthStore.mockReturnValue({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
      loading: false,
    });
    renderInRouter("/people");
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });
});
