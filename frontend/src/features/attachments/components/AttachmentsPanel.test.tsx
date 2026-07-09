import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetAuth, setAuth } from "../../identity/store/authStore.js";
import { AttachmentsPanel } from "../index.js";

function makeFileList(files: File[]): FileList {
  const fileList = {
    length: files.length,
    item: (index: number) => files[index] ?? null,
  } as FileList & { [index: number]: File };
  files.forEach((file, index) => {
    fileList[index] = file;
  });
  return fileList;
}

const dealData = {
  id: "d1",
  name: "Acme deal",
  isLoading: false,
  isError: false,
  data: {
    id: "d1",
    name: "Acme deal",
  },
};

const attachments = [
  {
    id: "a1",
    workspace_id: "ws1",
    entity_type: "deal",
    entity_id: "d1",
    filename: "QA-Validation-Requirements.docx",
    content_type:
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    byte_size: 1536,
    storage_key: "attachments/a1",
    checksum: "sha256:abc",
    access: "visible",
    scan_status: "clean",
    source: "email",
    captured_by: "agent:attachment-extractor",
    upload_url: null,
    download_url: "https://blob.example/download/a1",
    created_at: "2026-07-09T08:00:00Z",
    archived_at: null,
  },
  {
    id: "a2",
    workspace_id: "ws1",
    entity_type: "deal",
    entity_id: "d1",
    filename: "Brochure.pdf",
    content_type: "application/pdf",
    byte_size: 2048,
    storage_key: "attachments/a2",
    checksum: "sha256:def",
    access: "visible",
    scan_status: "clean",
    source: "human",
    captured_by: "human:u1",
    upload_url: null,
    download_url: "https://blob.example/download/a2",
    created_at: "2026-07-09T09:00:00Z",
    archived_at: null,
  },
] as const;

const extraction = {
  fields: [
    {
      field: "name",
      value: "Acme Deal",
      source_quote: "Acme Deal",
      page_or_section: "Page 1",
      confidence: "high",
    },
    {
      field: "amount_minor",
      value: "1000000",
      source_quote: "$10,000.00",
      page_or_section: "Section 2",
      confidence: "medium",
    },
  ],
  omitted: [
    {
      field: "expected_close_date",
      reason: "not_stated_in_file",
    },
  ],
};

const dealActivities = vi.fn();
const attachmentsMutate = vi.fn();
const attachmentExtraction = vi.fn();
const acceptExtraction = vi.fn();
const requestAccess = vi.fn();
const refetchAttachments = vi.fn();
const useAttachmentsMock = vi.fn();

vi.mock("../../deals/api/deals.js", () => ({
  useDeal: () => dealData,
  useDealActivities: () => ({
    data: dealActivities(),
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("../api/attachments.js", () => ({
  useAttachments: (...args: unknown[]) => useAttachmentsMock(...args),
  useCreateAttachment: () => ({
    mutateAsync: attachmentsMutate,
  }),
  useAttachmentExtraction: () => ({
    data: attachmentExtraction(),
    isLoading: false,
    isError: false,
  }),
  useAcceptExtraction: () => ({
    mutate: acceptExtraction,
    isPending: false,
  }),
  useRequestAccess: () => ({
    mutateAsync: requestAccess,
  }),
}));

describe("AttachmentsPanel", () => {
  beforeEach(() => {
    resetAuth();
    setAuth(
      { id: "u1", email: "sara@example.com", full_name: "Sara" } as never,
      "Sales",
      ["Sales"],
    );
    dealActivities.mockReset();
    attachmentsMutate.mockReset();
    attachmentExtraction.mockReset();
    acceptExtraction.mockReset();
    requestAccess.mockReset();
    refetchAttachments.mockReset();
    useAttachmentsMock.mockReset();
    attachmentExtraction.mockReturnValue(extraction as never);
    dealActivities.mockReturnValue([]);
    useAttachmentsMock.mockReturnValue({
      data: attachments,
      isLoading: false,
      isError: false,
      refetch: refetchAttachments,
    });
  });

  afterEach(() => {
    resetAuth();
    vi.clearAllMocks();
  });

  it("composes the deal header, extraction panel, and download toast", async () => {
    const user = userEvent.setup();
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    render(
      <MemoryRouter>
        <AttachmentsPanel entityType="deal" entityId="d1" dealId="d1" />
      </MemoryRouter>,
    );

    expect(screen.getByTestId("attachments-panel")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Acme deal" })).toHaveAttribute(
      "href",
      "/deals/d1",
    );
    expect(
      screen.getByText(/Your role: Sales · sees deal-room files/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText("QA-Validation-Requirements.docx"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", {
        name: /AI read this file — 2 fields it can ground, staged for your record \(accept to persist\)/i,
      }),
    ).toBeInTheDocument();

    const downloadButtons = screen.getAllByRole("button", {
      name: /^download$/i,
    });
    expect(downloadButtons.length).toBeGreaterThan(0);
    await user.click(downloadButtons[0]);

    expect(clickSpy).toHaveBeenCalledOnce();
    expect(
      await screen.findByText("Downloaded — access logged"),
    ).toBeInTheDocument();

    clickSpy.mockRestore();
  });

  it("opens and closes the details drawer from a row", async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AttachmentsPanel entityType="deal" entityId="d1" dealId="d1" />
      </MemoryRouter>,
    );

    const detailsButtons = screen.getAllByRole("button", {
      name: /^details$/i,
    });
    expect(detailsButtons.length).toBeGreaterThan(0);
    await user.click(detailsButtons[0]);

    const dialog = screen.getByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "QA-Validation-Requirements.docx" }),
    ).toBeInTheDocument();

    await user.keyboard("{Escape}");

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("sends the real signed-in user's id as captured_by, not a placeholder", async () => {
    resetAuth();
    setAuth(
      { id: "u42", email: "priya@example.com", full_name: "Priya" } as never,
      "Sales",
      ["Sales"],
    );
    attachmentsMutate.mockResolvedValue({
      ...attachments[1],
      id: "a3",
      filename: "diagram.png",
      scan_status: "scanning",
    });

    render(
      <MemoryRouter>
        <AttachmentsPanel entityType="deal" entityId="d1" dealId="d1" />
      </MemoryRouter>,
    );

    const file = new File(["hello"], "diagram.png", { type: "image/png" });
    fireEvent.drop(screen.getByTestId("dropzone"), {
      dataTransfer: { files: makeFileList([file]) },
    });

    await waitFor(() => {
      expect(attachmentsMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          request: expect.objectContaining({ captured_by: "human:u42" }),
        }),
      );
    });
    expect(attachmentsMutate).not.toHaveBeenCalledWith(
      expect.objectContaining({
        request: expect.objectContaining({ captured_by: "human:you" }),
      }),
    );
  });

  it("toasts virus-scan-in-progress on upload, then confirms once the scan completes", async () => {
    const uploaded = {
      ...attachments[1],
      id: "a3",
      filename: "diagram.png",
      scan_status: "scanning" as const,
    };
    attachmentsMutate.mockResolvedValue(uploaded);

    const { rerender } = render(
      <MemoryRouter>
        <AttachmentsPanel entityType="deal" entityId="d1" dealId="d1" />
      </MemoryRouter>,
    );

    const file = new File(["hello"], "diagram.png", { type: "image/png" });
    fireEvent.drop(screen.getByTestId("dropzone"), {
      dataTransfer: { files: makeFileList([file]) },
    });

    expect(
      await screen.findByText("Virus scan in progress"),
    ).toBeInTheDocument();

    // Simulate the existing useAttachments poll (attachments.ts) observing
    // the scan flip from "scanning" to "clean" for the just-uploaded file.
    useAttachmentsMock.mockReturnValue({
      data: [...attachments, { ...uploaded, scan_status: "clean" as const }],
      isLoading: false,
      isError: false,
      refetch: refetchAttachments,
    });

    rerender(
      <MemoryRouter>
        <AttachmentsPanel entityType="deal" entityId="d1" dealId="d1" />
      </MemoryRouter>,
    );

    expect(
      await screen.findByText(
        "diagram.png attached and written to the timeline with provenance",
      ),
    ).toBeInTheDocument();
  });
});
