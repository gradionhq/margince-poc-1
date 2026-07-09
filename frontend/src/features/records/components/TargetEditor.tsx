import { useState } from "react";
import { Button, TextInput } from "../../../shared/ui/forge.js";
import type { Quota } from "../api/quotas.js";
import {
  parseGermanIntegerEuros,
  useUpdateQuotaTarget,
} from "../api/quotas.js";

export function TargetEditor({
  quota,
  onToast,
}: {
  quota: Quota;
  onToast: (variant: "success" | "error", message: string) => void;
}) {
  const [value, setValue] = useState(
    (quota.target_minor / 100).toLocaleString("de-DE", {
      maximumFractionDigits: 0,
    }),
  );
  const mutation = useUpdateQuotaTarget(quota.id);

  function handleSave() {
    const targetMinor = parseGermanIntegerEuros(value);
    if (targetMinor <= 0) {
      onToast("error", "Enter a target amount in EUR");
      return;
    }

    mutation.mutate(
      { targetMinor, version: quota.version },
      {
        onSuccess: (updated) => {
          setValue(
            (updated.target_minor / 100).toLocaleString("de-DE", {
              maximumFractionDigits: 0,
            }),
          );
          onToast(
            "success",
            "Target saved as human-typed — change logged, attainment recomputed",
          );
        },
        onError: () =>
          onToast("error", "Failed to save target. Please try again."),
      },
    );
  }

  return (
    <div>
      <h4 className="mb-gf-sm text-gf-label font-semibold uppercase tracking-wide text-gf-tertiary">
        Period target
      </h4>
      <div className="flex items-center gap-gf-md">
        <TextInput
          value={value}
          onChange={setValue}
          className="w-40 font-mono"
        />
        <span className="font-mono text-gf-caption text-gf-tertiary">EUR</span>
      </div>
      <Button
        variant="secondary"
        size="sm"
        className="mt-gf-md w-full justify-center"
        onClick={handleSave}
        loading={mutation.isPending}
      >
        Save target
      </Button>
      <p className="mt-gf-sm text-gf-caption text-gf-tertiary">
        Editing writes a human-typed value to the quota and logs the change.
        Attainment recomputes against it.
      </p>
    </div>
  );
}
