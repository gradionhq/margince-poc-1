export function computeLineNet(
  quantity: number,
  unitPriceMinor: number,
  discountPct: number,
) {
  return Math.round(quantity * unitPriceMinor * (1 - discountPct / 100));
}

export function computeLineTax(lineNetMinor: number, taxRate: number) {
  return Math.round((lineNetMinor * taxRate) / 100);
}

export function computeOfferTotals(
  lines: Array<{
    quantity: number;
    unitPriceMinor: number;
    discountPct: number;
    taxRate: number;
  }>,
) {
  let netMinor = 0;
  let taxMinor = 0;

  for (const line of lines) {
    const lineNetMinor = computeLineNet(
      line.quantity,
      line.unitPriceMinor,
      line.discountPct,
    );
    const lineTaxMinor = computeLineTax(lineNetMinor, line.taxRate);
    netMinor += lineNetMinor;
    taxMinor += lineTaxMinor;
  }

  return { netMinor, taxMinor, grossMinor: netMinor + taxMinor };
}
