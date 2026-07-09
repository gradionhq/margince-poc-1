import { describe, expect, it } from "vitest";
import { formatMoneyForLocale } from "./money.js";

describe("formatMoneyForLocale", () => {
  it("formats German amounts with de-DE conventions", () => {
    expect(formatMoneyForLocale(123456, "EUR", "de")).toBe("1.234,56 €");
  });

  it("formats English amounts with en-US conventions", () => {
    expect(formatMoneyForLocale(123456, "USD", "en")).toBe("$1,234.56");
  });

  it("uses the locale-specific formatter rather than a shared default", () => {
    expect(formatMoneyForLocale(1000, "EUR", "de")).not.toBe(
      formatMoneyForLocale(1000, "EUR", "en"),
    );
  });
});
