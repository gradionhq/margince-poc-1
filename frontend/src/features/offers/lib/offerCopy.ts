export type OfferLocale = "de" | "en";

type OfferCopy = {
  title: string;
  meta: {
    offerNumber: string;
    deal: string;
    validUntil: string;
  };
  lineTable: {
    description: string;
    quantity: string;
    unit: string;
    unitPrice: string;
    discount: string;
    taxRate: string;
    net: string;
  };
  totals: {
    net: string;
    tax: string;
    gross: string;
  };
  legal: string;
  pdfButton: string;
  viewPdf: string;
};

const offerCopy: Record<OfferLocale, OfferCopy> = {
  de: {
    title: "Angebot",
    meta: {
      offerNumber: "Angebotsnummer",
      deal: "Deal",
      validUntil: "Gültig bis",
    },
    lineTable: {
      description: "Beschreibung",
      quantity: "Menge",
      unit: "Einheit",
      unitPrice: "Stückpreis",
      discount: "Rabatt",
      taxRate: "MwSt.",
      net: "Netto",
    },
    totals: {
      net: "Netto",
      tax: "MwSt.",
      gross: "Brutto",
    },
    legal:
      "Dieses Angebot ist freibleibend und gilt bis zum angegebenen Datum.",
    pdfButton: "PDF erzeugen",
    viewPdf: "PDF ansehen",
  },
  en: {
    title: "Offer",
    meta: {
      offerNumber: "Offer number",
      deal: "Deal",
      validUntil: "Valid until",
    },
    lineTable: {
      description: "Description",
      quantity: "Qty",
      unit: "Unit",
      unitPrice: "Unit price",
      discount: "Discount",
      taxRate: "Tax rate",
      net: "Net",
    },
    totals: {
      net: "Net",
      tax: "Tax",
      gross: "Gross",
    },
    legal:
      "This offer is non-binding and remains valid until the listed date.",
    pdfButton: "Generate PDF",
    viewPdf: "View PDF",
  },
};

export function getOfferCopy(locale: string = "de") {
  return offerCopy[locale === "en" ? "en" : "de"];
}
