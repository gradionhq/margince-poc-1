import { Toast } from "@shared/ui";
import { createPortal } from "react-dom";

type ToastItem = { id: string; message: string; variant?: string };

const VALID_VARIANTS = new Set(["success", "error", "warning", "info"]);
type ToastVariant = "success" | "error" | "warning" | "info";

function toVariant(v?: string): ToastVariant {
  return VALID_VARIANTS.has(v ?? "") ? (v as ToastVariant) : "info";
}

export function ToastContainer({
  toasts,
  onDismiss,
}: {
  toasts: ReadonlyArray<ToastItem>;
  onDismiss: (id: string) => void;
}) {
  if (toasts.length === 0) return null;
  return createPortal(
    <div className="fixed bottom-gf-md right-gf-md z-gf-toast flex flex-col gap-gf-sm">
      {toasts.map((t) => (
        <Toast
          key={t.id}
          id={t.id}
          variant={toVariant(t.variant)}
          title={t.message}
          onClose={onDismiss}
        />
      ))}
    </div>,
    document.body,
  );
}
