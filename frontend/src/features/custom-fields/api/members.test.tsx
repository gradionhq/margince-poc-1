import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import { useMembers } from "./members.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("members read API", () => {
  it("useMembers fetches all members without parameters", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        data: [
          { id: "m1", name: "Alice" },
          { id: "m2", name: "Bob" },
        ],
        page: {},
      },
      error: undefined,
    });
    const { result } = renderHook(() => useMembers(), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith("/members");
    expect(result.current.data?.data).toHaveLength(2);
  });
});
