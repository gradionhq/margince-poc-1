import type { ReactElement } from "react";

export type AttachmentsPanelProps =
  | {
      entityType: "deal";
      entityId: string;
      dealId: string;
    }
  | {
      entityType: "person" | "organization" | "lead" | "activity";
      entityId: string;
      dealId?: never;
    };

export function AttachmentsPanel(_props: AttachmentsPanelProps): ReactElement | null {
  return null;
}
