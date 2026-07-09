import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { AttachmentRow } from "./AttachmentRow.js";

const mutateAsync = vi.fn();

vi.mock("../api/attachments.js", () => ({
  useRequestAccess: () => ({
    mutateAsync,
  }),
}));

type Attachment = components["schemas"]["Attachment"];

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

describe("AttachmentRow", () => {
  afterEach(() => {
    mutateAsync.mockReset();
  });

  it("renders scan status, size, provenance, and actions", () => {
    render(
      <AttachmentRow
        attachment={makeAttachment({
          source: "human",
          captured_by: "human:u1",
        })}
        onDownload={vi.fn()}
        onDetails={vi.fn()}
      />,
    );

    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
    expect(screen.getByText("Clean")).toBeInTheDocument();
    expect(screen.getByText("1,536 bytes")).toBeInTheDocument();
    expect(screen.getByText(/uploaded by/i)).toBeInTheDocument();
    expect(screen.getByText(/2026/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /download/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /details/i }),
    ).toBeInTheDocument();
  });

  it("makes the filename clickable when the source is human or agent-captured", async () => {
    const user = userEvent.setup();
    const onFilenameClick = vi.fn();

    render(
      <AttachmentRow
        attachment={makeAttachment({
          source: "human",
          captured_by: "human:u1",
        })}
        onFilenameClick={onFilenameClick}
      />,
    );

    await user.click(screen.getByRole("button", { name: "contract.pdf" }));
    expect(onFilenameClick).toHaveBeenCalledOnce();
  });

  it("shows the agent provenance label for captured rows", () => {
    render(
      <AttachmentRow
        attachment={makeAttachment({
          source: "email",
          captured_by: "agent:attachment-extractor",
        })}
      />,
    );

    expect(
      screen.getByText(/captured by agent:attachment-extractor/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "contract.pdf" }),
    ).toBeInTheDocument();
  });

  it("locks restricted rows down to request-access and keeps scan status visible", async () => {
    const user = userEvent.setup();

    render(
      <AttachmentRow
        attachment={makeAttachment({
          access: "restricted",
          scan_status: "clean",
        })}
      />,
    );

    expect(screen.getByText("Clean")).toBeInTheDocument();
    expect(screen.getByText("Restricted")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /request access/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /download/i }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /request access/i }));

    expect(mutateAsync).toHaveBeenCalledOnce();
    expect(
      await screen.findByText(/access request sent and logged/i),
    ).toBeInTheDocument();
  });

  it("blocks downloads and exposes the reason action for quarantined files", async () => {
    const user = userEvent.setup();

    render(
      <AttachmentRow
        attachment={makeAttachment({
          scan_status: "blocked",
          download_url: null,
        })}
      />,
    );

    expect(screen.getByText("Blocked")).toBeInTheDocument();
    expect(
      screen.getByText(/quarantined - not downloadable/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /download/i }),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: /why was this blocked/i }),
    );

    expect(
      await screen.findByText(/blocked because the file was quarantined/i),
    ).toBeInTheDocument();
  });
});
