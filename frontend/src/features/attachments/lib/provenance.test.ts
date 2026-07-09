import { describe, expect, it } from "vitest";
import type { components } from "../../../lib/api-client/generated/index.js";
import { provenanceLabel } from "./provenance.js";

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

describe("provenanceLabel", () => {
  it("renders 'uploaded by you' when the current user is the uploader", () => {
    const attachment = makeAttachment({ captured_by: "human:u1" });
    expect(provenanceLabel(attachment, "u1")).toBe("uploaded by you");
  });

  it("renders the raw id when the current user is a different human", () => {
    const attachment = makeAttachment({ captured_by: "human:u1" });
    expect(provenanceLabel(attachment, "u2")).toBe("uploaded by u1");
  });

  it("renders the raw id when the current user id is unavailable", () => {
    const attachment = makeAttachment({ captured_by: "human:u1" });
    expect(provenanceLabel(attachment)).toBe("uploaded by u1");
  });

  it("never leaks 'you' for a non-matching id, even if the ids merely share a substring", () => {
    const attachment = makeAttachment({ captured_by: "human:u12" });
    expect(provenanceLabel(attachment, "u1")).toBe("uploaded by u12");
  });

  it("renders the agent marker unchanged regardless of currentUserId", () => {
    const attachment = makeAttachment({
      captured_by: "agent:attachment-extractor",
    });
    expect(provenanceLabel(attachment, "u1")).toBe(
      "captured by agent:attachment-extractor",
    );
  });
});
