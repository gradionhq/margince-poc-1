import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { AttachmentList } from "./AttachmentList.js";

vi.mock("../api/attachments.js", () => ({
  useAttachments: vi.fn(),
  useRequestAccess: () => ({
    mutateAsync: vi.fn(),
  }),
}));

import { useAttachments } from "../api/attachments.js";

type Attachment = components["schemas"]["Attachment"];

function renderList(props: Parameters<typeof AttachmentList>[0]) {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <AttachmentList {...props} />
    </QueryClientProvider>,
  );
}

function makeAttachment(overrides: Partial<Attachment> = {}): Attachment {
  return {
    id: "a1",
    workspace_id: "ws1",
    entity_type: "deal",
    entity_id: "d1",
    filename: "contract.pdf",
    content_type: "application/pdf",
    byte_size: 1536,
    storage_key: "attachments/a1",
    checksum: "sha256:abc",
    access: "visible",
    scan_status: "clean",
    source: "ui",
    captured_by: "human:u1",
    upload_url: null,
    download_url: "https://blob.example/download/a1",
    created_at: "2026-07-09T08:00:00Z",
    archived_at: null,
    ...overrides,
  };
}

afterEach(() => {
  vi.clearAllMocks();
});

describe("AttachmentList", () => {
  it("shows an honest empty state", () => {
    vi.mocked(useAttachments).mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as never);

    renderList({ entityType: "deal", entityId: "d1" });

    expect(screen.getByText("No files attached yet")).toBeInTheDocument();
    expect(screen.queryByTestId("attachment-row-a1")).not.toBeInTheDocument();
  });

  it("shows a loading skeleton while the list is loading", () => {
    vi.mocked(useAttachments).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      refetch: vi.fn(),
    } as never);

    renderList({ entityType: "deal", entityId: "d1" });

    expect(screen.getByTestId("attachment-list-skeleton")).toBeInTheDocument();
  });

  it("shows an error card with retry", async () => {
    const refetch = vi.fn();
    vi.mocked(useAttachments).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch,
    } as never);

    const user = userEvent.setup();
    renderList({ entityType: "deal", entityId: "d1" });

    expect(screen.getByTestId("attachment-list-error")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(refetch).toHaveBeenCalledOnce();
  });

  it("renders attachment rows when data is present", () => {
    vi.mocked(useAttachments).mockReturnValue({
      data: [makeAttachment()],
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as never);

    renderList({ entityType: "deal", entityId: "d1" });

    expect(screen.getByTestId("attachment-row-a1")).toBeInTheDocument();
    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
  });

  it("renders the visibility rail and filters restricted rows on the client", async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    vi.mocked(useAttachments).mockReturnValue({
      data: [
        makeAttachment({ id: "a1", filename: "visible.pdf" }),
        makeAttachment({
          id: "a2",
          filename: "restricted.pdf",
          access: "restricted",
        }),
      ],
      isLoading: false,
      isError: false,
      refetch,
    } as never);

    renderList({ entityType: "deal", entityId: "d1" });

    expect(
      screen.getByTestId("attachments-visibility-rail"),
    ).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "All" })).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Visible to me" }),
    ).toBeInTheDocument();
    expect(screen.getByText("visible.pdf")).toBeInTheDocument();
    expect(screen.getByText("restricted.pdf")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Visible to me" }));

    expect(screen.getByText("visible.pdf")).toBeInTheDocument();
    expect(screen.queryByText("restricted.pdf")).not.toBeInTheDocument();
    expect(refetch).not.toHaveBeenCalled();
  });
});
