import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type {
  Activity,
  components,
} from "../../../lib/api-client/generated/index.js";
import { DetailsDrawer } from "./DetailsDrawer.js";

type Attachment = components["schemas"]["Attachment"];
type Activity = components["schemas"]["Activity"];

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
    access: "restricted",
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

function makeActivity(overrides: Partial<Activity> = {}): Activity {
  return {
    id: "act-1",
    workspace_id: "ws1",
    kind: "note",
    occurred_at: "2026-07-09T08:05:00Z",
    is_done: false,
    source: "ui",
    captured_by: "human:u1",
    created_at: "2026-07-09T08:05:00Z",
    updated_at: "2026-07-09T08:05:00Z",
    ...overrides,
  };
}

describe("DetailsDrawer", () => {
  it("renders attachment facts, derives the closest timeline entry, and closes on backdrop click or Escape", () => {
    const onClose = vi.fn();
    const attachment = makeAttachment();
    const activities: Activity[] = [
      makeActivity({
        id: "act-older",
        occurred_at: "2026-07-09T07:00:00Z",
        created_at: "2026-07-09T07:00:00Z",
        updated_at: "2026-07-09T07:00:00Z",
      }),
      makeActivity({
        id: "act-closest",
        occurred_at: "2026-07-09T08:05:00Z",
        created_at: "2026-07-09T08:05:00Z",
        updated_at: "2026-07-09T08:05:00Z",
      }),
      makeActivity({
        id: "act-later",
        occurred_at: "2026-07-09T11:00:00Z",
        created_at: "2026-07-09T11:00:00Z",
        updated_at: "2026-07-09T11:00:00Z",
      }),
    ];

    render(
      <DetailsDrawer
        attachment={attachment}
        open
        onClose={onClose}
        activities={activities}
      />,
    );

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("contract.pdf")).toBeInTheDocument();
    expect(within(dialog).getByText("application/pdf")).toBeInTheDocument();
    expect(within(dialog).getByText("1,536 bytes")).toBeInTheDocument();
    expect(within(dialog).getByText("SHA-256")).toBeInTheDocument();
    expect(within(dialog).getByText("sha256:abc")).toBeInTheDocument();
    expect(within(dialog).getByText(/uploaded by u1/i)).toBeInTheDocument();
    expect(within(dialog).getByText(/7\/9\/2026/i)).toBeInTheDocument();
    expect(within(dialog).getByText("Clean")).toBeInTheDocument();
    expect(within(dialog).getByText("restricted")).toBeInTheDocument();
    expect(
      within(dialog).getByText("Closest matching timeline entry"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("act-closest")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("details-drawer-backdrop"));
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("prefers a stored timeline activity id when the attachment provides one", () => {
    const onClose = vi.fn();
    const attachment = {
      ...makeAttachment({
        captured_by: "agent:attachment-extractor",
      }),
      activity_id: "stored-act",
    };

    render(
      <DetailsDrawer attachment={attachment} open onClose={onClose} />,
    );

    expect(screen.getByText("Timeline activity id")).toBeInTheDocument();
    expect(screen.getByText("stored-act")).toBeInTheDocument();
    expect(screen.getByText(/captured by agent:attachment-extractor/i)).toBeInTheDocument();
  });
});
