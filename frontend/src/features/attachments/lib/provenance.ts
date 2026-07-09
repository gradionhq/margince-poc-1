import type { components } from "../../../lib/api-client/generated/index.js";

type Attachment = components["schemas"]["Attachment"];

/**
 * Renders a human-readable provenance label for an attachment.
 *
 * `currentUserId` is the signed-in viewer's id (undefined while auth is
 * still loading, or for viewers who aren't signed in). When the attachment
 * was captured by that same human, we show "uploaded by you" instead of
 * their raw id. Every other case falls back to the existing behavior:
 * the raw `human:<id>` suffix, or the `agent:<name>` marker unchanged.
 */
export function provenanceLabel(
  attachment: Attachment,
  currentUserId?: string,
) {
  if (attachment.captured_by.startsWith("agent:")) {
    return `captured by ${attachment.captured_by}`;
  }
  if (currentUserId && attachment.captured_by === `human:${currentUserId}`) {
    return "uploaded by you";
  }
  return `uploaded by ${attachment.captured_by.replace(/^human:/, "")}`;
}
