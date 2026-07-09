import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
    PATCH: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  customFieldsKeys,
  useCreateCustomField,
  useCustomFields,
  useRenameCustomField,
  useRetireCustomField,
  useUpdateCustomFieldOptions,
} from "./customFields.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("customFields read API", () => {
  it("useCustomFields fetches all custom fields for an object (no status filter)", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        data: [
          { id: "cf1", label: "Field 1", object: "deal" },
          { id: "cf2", label: "Field 2", object: "deal" },
        ],
        page: {},
      },
      error: undefined,
    });
    const { result } = renderHook(() => useCustomFields("deal"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/custom-fields",
      expect.objectContaining({
        params: { query: { object: "deal" } },
      }),
    );
    expect(result.current.data?.data).toHaveLength(2);
  });
});

describe("useCreateCustomField", () => {
  it("posts a CreateCustomFieldRequest and returns the created custom field", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "cf9", label: "New Field", object: "deal" },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useCreateCustomField(), {
      wrapper: localWrapper,
    });
    const created = await result.current.mutateAsync({
      label: "New Field",
      object: "deal",
      field_type: "text",
    });
    expect(created.id).toBe("cf9");
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/custom-fields",
      expect.objectContaining({
        body: expect.objectContaining({
          label: "New Field",
          object: "deal",
        }),
      }),
    );
  });

  it("invalidates the custom fields list for the field's object after creation", async () => {
    const qc = new QueryClient();
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "cf9", label: "New Field", object: "deal" },
      error: undefined,
    });
    function localWrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    const { result } = renderHook(() => useCreateCustomField(), {
      wrapper: localWrapper,
    });
    await act(async () => {
      await result.current.mutateAsync({
        label: "New Field",
        object: "deal",
        field_type: "text",
      });
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: customFieldsKeys.list("deal"),
    });
  });
});

describe("useRenameCustomField", () => {
  it("patches a custom field and returns the renamed field", async () => {
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "cf1", label: "Renamed Field", object: "deal" },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useRenameCustomField(), {
      wrapper: localWrapper,
    });
    const renamed = await result.current.mutateAsync({
      id: "cf1",
      label: "Renamed Field",
    });
    expect(renamed.label).toBe("Renamed Field");
    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/custom-fields/{id}",
      expect.objectContaining({
        params: { path: { id: "cf1" } },
        body: { label: "Renamed Field" },
      }),
    );
  });

  it("invalidates the custom fields list for the field's object after rename", async () => {
    const qc = new QueryClient();
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { id: "cf1", label: "Renamed Field", object: "deal" },
      error: undefined,
    });
    function localWrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    const { result } = renderHook(() => useRenameCustomField(), {
      wrapper: localWrapper,
    });
    await act(async () => {
      await result.current.mutateAsync({
        id: "cf1",
        label: "Renamed Field",
      });
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: customFieldsKeys.list("deal"),
    });
  });
});

describe("useRetireCustomField", () => {
  it("posts retire endpoint and returns the retired field", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "cf1",
        label: "Field 1",
        object: "deal",
        retired_at: "2026-07-09T00:00:00Z",
      },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useRetireCustomField(), {
      wrapper: localWrapper,
    });
    const retired = await result.current.mutateAsync("cf1");
    expect(retired.id).toBe("cf1");
    expect(apiClient.POST).toHaveBeenCalledWith(
      "/custom-fields/{id}/retire",
      expect.objectContaining({
        params: { path: { id: "cf1" } },
      }),
    );
  });

  it("invalidates the custom fields list for the field's object after retire", async () => {
    const qc = new QueryClient();
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "cf1",
        label: "Field 1",
        object: "deal",
        retired_at: "2026-07-09T00:00:00Z",
      },
      error: undefined,
    });
    function localWrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    const { result } = renderHook(() => useRetireCustomField(), {
      wrapper: localWrapper,
    });
    await act(async () => {
      await result.current.mutateAsync("cf1");
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: customFieldsKeys.list("deal"),
    });
  });
});

describe("useUpdateCustomFieldOptions", () => {
  it("patches custom field options and returns the updated field", async () => {
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "cf1",
        label: "Status",
        object: "deal",
        options: ["Active", "Inactive"],
      },
      error: undefined,
    });
    const qc = new QueryClient();
    const localWrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useUpdateCustomFieldOptions(), {
      wrapper: localWrapper,
    });
    const updated = await result.current.mutateAsync({
      id: "cf1",
      options: ["Active", "Inactive"],
    });
    expect(updated.id).toBe("cf1");
    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/custom-fields/{id}/options",
      expect.objectContaining({
        params: { path: { id: "cf1" } },
        body: { options: ["Active", "Inactive"] },
      }),
    );
  });

  it("invalidates the custom fields list for the field's object after updating options", async () => {
    const qc = new QueryClient();
    const invalidateSpy = vi.spyOn(qc, "invalidateQueries");
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "cf1",
        label: "Status",
        object: "deal",
        options: ["Active", "Inactive"],
      },
      error: undefined,
    });
    function localWrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
    }
    const { result } = renderHook(() => useUpdateCustomFieldOptions(), {
      wrapper: localWrapper,
    });
    await act(async () => {
      await result.current.mutateAsync({
        id: "cf1",
        options: ["Active", "Inactive"],
      });
    });
    expect(invalidateSpy).toHaveBeenCalledWith({
      queryKey: customFieldsKeys.list("deal"),
    });
  });
});
