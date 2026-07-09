import type { components } from "../../../lib/api-client/generated/index.js";

type ComputedField = components["schemas"]["ComputedField"];

const currencyFormatter = new Intl.NumberFormat("de-DE", {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatComputedFieldValue(field: ComputedField): string {
  if (!field.computable) return "Not computable yet";

  switch (field.kind) {
    case "currency_minor":
      return `${currencyFormatter.format((field.value_minor ?? 0) / 100)} €`;
    case "duration_months":
      return `${String(field.value ?? 0)} mo`;
    case "percent":
      return `${String(field.value ?? 0)}%`;
    case "count":
      return String(field.value ?? 0);
  }
}
