package crmctx

import (
	"context"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	ctx := With(context.Background(), Principal{UserID: "u1", TenantID: "t1"})
	p, ok := From(ctx)
	if !ok || p.UserID != "u1" {
		t.Fatalf("principal not round-tripped: %+v ok=%v", p, ok)
	}
}
