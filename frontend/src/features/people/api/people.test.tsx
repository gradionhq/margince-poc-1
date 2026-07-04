import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn() },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import { usePerson } from "./people.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("usePerson", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches a person by id", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "p1", full_name: "Dana Lee", title: "VP Sales" },
    });
    const { result } = renderHook(() => usePerson("p1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/people/{id}",
      expect.objectContaining({ params: { path: { id: "p1" } } }),
    );
    expect(result.current.data?.full_name).toBe("Dana Lee");
  });

  it("stays disabled when id is undefined", () => {
    const { result } = renderHook(() => usePerson(undefined), { wrapper });
    expect(result.current.fetchStatus).toBe("idle");
    expect(apiClient.GET).not.toHaveBeenCalled();
  });
});
