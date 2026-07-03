import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

// We test the store functions directly — no component needed.
// Mock fetchMe so tests don't hit the network.
vi.mock("../api/auth.js", () => ({
  fetchMe: vi.fn(),
}));

import * as authApi from "../api/auth.js";
import {
  clearAuth,
  initAuth,
  resetAuth,
  setAuth,
  useAuthStore,
} from "./authStore.js";

const mockFetchMe = vi.mocked(authApi.fetchMe);

describe("useAuthStore", () => {
  afterEach(() => {
    // reset store to initial state (loading=true, user=null, role=null) between tests
    act(() => resetAuth());
    vi.clearAllMocks();
  });

  it("starts with loading=true, user=null", () => {
    const { result } = renderHook(() => useAuthStore());
    // loading starts true before initAuth is called
    expect(result.current.loading).toBe(true);
    expect(result.current.user).toBeNull();
    expect(result.current.role).toBeNull();
  });

  it("initAuth sets user and role on 200", async () => {
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
    mockFetchMe.mockResolvedValueOnce({
      user: fakeUser,
      role: "admin",
      roles: ["admin"],
    });

    const { result } = renderHook(() => useAuthStore());
    await act(() => initAuth());

    expect(result.current.loading).toBe(false);
    expect(result.current.user).toEqual(fakeUser);
    expect(result.current.role).toBe("admin");
  });

  it("initAuth sets user=null on 401 (fetchMe returns null)", async () => {
    mockFetchMe.mockResolvedValueOnce(null);

    const { result } = renderHook(() => useAuthStore());
    await act(() => initAuth());

    expect(result.current.loading).toBe(false);
    expect(result.current.user).toBeNull();
    expect(result.current.role).toBeNull();
  });

  it("clearAuth resets user and role", () => {
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
    act(() => setAuth(fakeUser, "admin"));
    act(() => clearAuth());

    const { result } = renderHook(() => useAuthStore());
    expect(result.current.user).toBeNull();
    expect(result.current.role).toBeNull();
  });
});
