export function formatMoneyForLocale(
  amountMinor: number,
  currency: string,
  locale: string,
) {
  const resolvedLocale = locale === "de" ? "de-DE" : "en-US";
  return new Intl.NumberFormat(resolvedLocale, {
    style: "currency",
    currency,
  }).format(amountMinor / 100);
}
