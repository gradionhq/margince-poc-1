import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    GET: vi.fn(),
    POST: vi.fn(),
    DELETE: vi.fn(),
  },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import {
  useAcceptExtraction,
  useArchiveAttachment,
  useAttachment,
  useAttachmentExtraction,
  useAttachments,
  useCreateAttachment,
  useRequestAccess,
} from "./attachments.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("attachment API hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("useAttachments lists attachments and polls while any row is scanning", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({
        data: {
          data: [
            {
              id: "a1",
              scan_status: "scanning",
              entity_type: "deal",
              entity_id: "d1",
            },
          ],
          page: {},
        },
        error: undefined,
      })
      .mockResolvedValueOnce({
        data: {
          data: [
            {
              id: "a1",
              scan_status: "clean",
              entity_type: "deal",
              entity_id: "d1",
            },
          ],
          page: {},
        },
        error: undefined,
      });

    const { result } = renderHook(
      () => useAttachments({ entityType: "deal", entityId: "d1" }),
      { wrapper },
    );

    await waitFor(() =>
      expect(result.current.data?.[0]?.scan_status).toBe("scanning"),
    );
    expect(apiClient.GET).toHaveBeenCalledWith(
      "/attachments",
      expect.objectContaining({
        params: {
          query: {
            entity_type: "deal",
            entity_id: "d1",
          },
        },
      }),
    );
    expect(result.current.data?.[0].scan_status).toBe("scanning");

    await new Promise((resolve) => setTimeout(resolve, 3100));
    expect(apiClient.GET).toHaveBeenCalledTimes(2);
    expect(result.current.data?.[0].scan_status).toBe("clean");
  });

  it("useAttachment fetches a single attachment by id", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "a1",
        scan_status: "clean",
        entity_type: "deal",
        entity_id: "d1",
      },
      error: undefined,
    });

    const { result } = renderHook(() => useAttachment("a1"), { wrapper });
    let attachment:
      | {
          id: string;
          scan_status: string;
          entity_type: string;
          entity_id: string;
        }
      | undefined;
    await waitFor(() => {
      attachment = result.current.data;
      expect(attachment).toBeDefined();
      expect(attachment?.id).toBe("a1");
    });

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/attachments/a1",
      expect.objectContaining({ params: { path: { id: "a1" } } }),
    );
    expect(attachment?.id).toBe("a1");
  });

  it("useAttachment stays disabled when id is undefined", () => {
    const { result } = renderHook(() => useAttachment(undefined), {
      wrapper,
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(apiClient.GET).not.toHaveBeenCalled();
  });

  it("useCreateAttachment registers metadata, uploads bytes, and returns the attachment", async () => {
    const file = new File(["hello"], "contract.pdf", {
      type: "application/pdf",
    });
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchMock);

    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "a1",
        entity_type: "deal",
        entity_id: "d1",
        scan_status: "scanning",
        upload_url: "https://blob.example/upload/a1",
      },
      error: undefined,
    });

    const { result } = renderHook(() => useCreateAttachment(), {
      wrapper,
    });

    const created = await result.current.mutateAsync({
      request: {
        entity_type: "deal",
        entity_id: "d1",
        filename: "contract.pdf",
        content_type: "application/pdf",
        byte_size: file.size,
        source: "ui",
        captured_by: "human:u1",
      },
      file,
    });

    expect(apiClient.POST).toHaveBeenCalledWith(
      "/attachments",
      expect.objectContaining({
        body: expect.objectContaining({
          entity_id: "d1",
          filename: "contract.pdf",
        }),
      }),
    );
    expect(fetchMock).toHaveBeenCalledWith(
      "https://blob.example/upload/a1",
      expect.objectContaining({
        method: "PUT",
        body: file,
        headers: { "Content-Type": "application/pdf" },
      }),
    );
    expect(created.id).toBe("a1");
    vi.unstubAllGlobals();
  });

  it("useArchiveAttachment archives an attachment and returns the archived row", async () => {
    (apiClient.DELETE as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        id: "a1",
        entity_type: "deal",
        entity_id: "d1",
        scan_status: "clean",
        archived_at: "2026-07-05T00:00:00Z",
      },
      error: undefined,
    });

    const { result } = renderHook(() => useArchiveAttachment("a1"), {
      wrapper,
    });
    const archived = await result.current.mutateAsync();

    expect(apiClient.DELETE).toHaveBeenCalledWith(
      "/attachments/a1",
      expect.objectContaining({ params: { path: { id: "a1" } } }),
    );
    expect(archived.archived_at).toBe("2026-07-05T00:00:00Z");
  });

  it("useAttachmentExtraction reads staged extraction", async () => {
    (apiClient.GET as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        fields: [
          {
            field: "name",
            value: "Acme Deal",
            source_quote: "Acme Deal",
            page_or_section: "1",
            confidence: "high",
          },
        ],
        omitted: [],
      },
      error: undefined,
    });

    const { result } = renderHook(() => useAttachmentExtraction("a1"), {
      wrapper,
    });
    let extraction:
      | {
          fields: Array<{ field: string }>;
          omitted: Array<{ field: string }>;
        }
      | undefined;
    await waitFor(() => {
      extraction = result.current.data;
      expect(extraction).toBeDefined();
      expect(extraction?.fields).toHaveLength(1);
    });

    expect(apiClient.GET).toHaveBeenCalledWith(
      "/attachments/a1/extraction",
      expect.objectContaining({ params: { path: { id: "a1" } } }),
    );
    expect(extraction?.fields).toHaveLength(1);
  });

  it("useAcceptExtraction posts accepted field keys", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: {
        deal_id: "d1",
        accepted: [
          {
            field: "name",
            value: "Acme Deal",
            provenance: "human",
          },
        ],
      },
      error: undefined,
    });

    const { result } = renderHook(() => useAcceptExtraction("a1"), {
      wrapper,
    });
    const accepted = await result.current.mutateAsync({
      field_keys: ["name"],
      edits: { name: "Acme Deal" },
    });

    expect(apiClient.POST).toHaveBeenCalledWith(
      "/attachments/a1/extraction:accept",
      expect.objectContaining({
        params: { path: { id: "a1" } },
        body: expect.objectContaining({
          field_keys: ["name"],
        }),
      }),
    );
    expect(accepted.deal_id).toBe("d1");
  });

  it("useRequestAccess posts the request-access action", async () => {
    (apiClient.POST as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { requested: true },
      error: undefined,
    });

    const { result } = renderHook(() => useRequestAccess("a1"), {
      wrapper,
    });
    const response = await result.current.mutateAsync();

    expect(apiClient.POST).toHaveBeenCalledWith(
      "/attachments/a1/request-access",
      expect.objectContaining({ params: { path: { id: "a1" } } }),
    );
    expect(response.requested).toBe(true);
  });
});
