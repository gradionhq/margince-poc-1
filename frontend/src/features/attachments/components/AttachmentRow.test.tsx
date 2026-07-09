import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { AttachmentRow } from "./AttachmentRow.js";

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
    expect(screen.getByRole("button", { name: /download/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /details/i })).toBeInTheDocument();
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

    expect(screen.getByText(/captured by agent:attachment-extractor/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "contract.pdf" })).toBeInTheDocument();
  });
});
