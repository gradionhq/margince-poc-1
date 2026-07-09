import { useState } from "react";

export type ToastVariant = "success" | "error" | "info";

export type Toast = {
  id: string;
  variant: ToastVariant;
  message: string;
};

/**
 * Local, per-component toast queue. Each caller gets its own independent
 * state — this is not a shared/app-wide toast bus.
 */
export function useToasts() {
  const [toasts, setToasts] = useState<Toast[]>([]);

  function pushToast(variant: ToastVariant, message: string) {
    setToasts((current) => [
      ...current,
      { id: crypto.randomUUID(), variant, message },
    ]);
  }

  function dismissToast(id: string) {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }

  return { toasts, pushToast, dismissToast };
}
