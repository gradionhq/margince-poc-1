import { describe, expect, it } from "vitest";
import type {
  Offer,
  OfferLineItem,
  OfferLineItemListResponse,
  OfferListResponse,
} from "./index.js";

describe("offer type aliases", () => {
  it("exports Offer and OfferListResponse from the generated barrel", () => {
    const offer: Offer = {
      id: "00000000-0000-0000-0000-000000000101",
      workspace_id: "00000000-0000-0000-0000-000000000102",
      deal_id: "00000000-0000-0000-0000-000000000103",
      offer_number: "OFF-001",
      revision: 1,
      status: "draft",
      template_id: "00000000-0000-0000-0000-000000000104",
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      line_items: [],
      net_minor: 0,
      tax_minor: 0,
      gross_minor: 0,
    };

    const response: OfferListResponse = { data: [offer] };
    expect(response.data[0].offer_number).toBe("OFF-001");
  });

  it("exports OfferLineItem and OfferLineItemListResponse from the generated barrel", () => {
    const lineItem: OfferLineItem = {
      id: "00000000-0000-0000-0000-000000000201",
      workspace_id: "00000000-0000-0000-0000-000000000102",
      offer_id: "00000000-0000-0000-0000-000000000101",
      description: "Implementation",
      quantity: 1,
      unit_price_minor: 2500,
      discount_pct: 0,
      tax_rate: 0,
      line_net_minor: 2500,
      line_tax_minor: 0,
      line_gross_minor: 2500,
      source: "test",
      captured_by: "human:test",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-01T00:00:00Z",
      evidence: null,
      price_grounded: true,
    };

    const response: OfferLineItemListResponse = { data: [lineItem] };
    expect(response.data[0].description).toBe("Implementation");
  });
});
