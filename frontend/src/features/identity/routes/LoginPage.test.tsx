import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/auth.js", () => ({
  login: vi.fn(),
  fetchMe: vi.fn(),
}));
vi.mock("../store/authStore.js", () => ({
  setAuth: vi.fn(),
  useAuthStore: vi.fn(() => ({ user: null, role: null, loading: false })),
}));

import * as authApi from "../api/auth.js";
import * as authStore from "../store/authStore.js";
import { LoginPage } from "./LoginPage.js";

const mockLogin = vi.mocked(authApi.login);
const mockFetchMe = vi.mocked(authApi.fetchMe);
const mockSetAuth = vi.mocked(authStore.setAuth);

function renderLoginPage() {
  return render(
    <MemoryRouter initialEntries={["/login"]}>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/people" element={<div>People Page</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("LoginPage", () => {
  it("renders the login form", () => {
    renderLoginPage();
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it("calls login and navigates to /people on success", async () => {
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
    mockLogin.mockResolvedValueOnce(undefined);
    mockFetchMe.mockResolvedValueOnce({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
    });

    renderLoginPage();
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: "admin@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "changeme" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith("admin@example.com", "changeme");
    });
    await waitFor(() => {
      expect(mockSetAuth).toHaveBeenCalledWith(fakeUser, "admin", ["admin"]);
    });
    await waitFor(() => {
      expect(screen.getByText("People Page")).toBeInTheDocument();
    });
  });

  it("shows error message on login failure", async () => {
    mockLogin.mockRejectedValueOnce(new Error("Invalid email or password"));

    renderLoginPage();
    fireEvent.change(screen.getByLabelText(/email/i), {
      target: { value: "bad@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: "wrong" },
    });
    fireEvent.click(screen.getByRole("button", { name: /sign in/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/Invalid email or password/i),
      ).toBeInTheDocument();
    });
  });
});
