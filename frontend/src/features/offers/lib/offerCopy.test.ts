import { describe, expect, it } from "vitest";
import { getOfferCopy } from "./offerCopy.js";

describe("offerCopy", () => {
  it("defaults to German copy", () => {
    const copy = getOfferCopy();

    expect(copy.title).toBe("Angebot");
    expect(copy.meta.offerNumber).toBe("Angebotsnummer");
    expect(copy.legal).toContain("Dieses Angebot");
  });

  it("returns English copy when requested", () => {
    const copy = getOfferCopy("en");

    expect(copy.title).toBe("Offer");
    expect(copy.meta.validUntil).toBe("Valid until");
    expect(copy.totals.gross).toBe("Gross");
  });

  it("falls back to German for unknown locales", () => {
    expect(getOfferCopy("fr")).toBe(getOfferCopy("de"));
  });
});
