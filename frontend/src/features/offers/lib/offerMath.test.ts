import { describe, expect, it } from "vitest";
import {
  computeLineNet,
  computeLineTax,
  computeOfferTotals,
} from "./offerMath.js";

describe("offerMath", () => {
  it("matches the backend round-then-sum line math for fractional inputs", () => {
    expect(computeLineNet(2, 1999, 12.5)).toBe(3498);
    expect(computeLineTax(3498, 19)).toBe(665);
  });

  it("rounds half away from zero like Go math.Round", () => {
    expect(computeLineNet(1, 5, 0)).toBe(5);
    expect(computeLineTax(5, 10)).toBe(1);
  });

  it("sums line totals across a mixed offer and leaves zero-price lines at zero", () => {
    const totals = computeOfferTotals([
      { quantity: 2, unitPriceMinor: 1999, discountPct: 12.5, taxRate: 19 },
      { quantity: 3, unitPriceMinor: 500, discountPct: 0, taxRate: 7.5 },
      { quantity: 4, unitPriceMinor: 0, discountPct: 33.3, taxRate: 17.5 },
    ]);

    expect(totals).toEqual({
      netMinor: 3498 + 1500,
      taxMinor: 665 + 113,
      grossMinor: 5776,
    });
  });
});
