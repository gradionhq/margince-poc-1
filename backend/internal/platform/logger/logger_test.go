package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestTraceparentRoundTrip(t *testing.T) {
	tp := FormatTraceparent("0af7651916cd43dd8448eb211c80319c", "b7ad6b7169203331")
	tid, sid, ok := ParseTraceparent(tp)
	if !ok || tid != "0af7651916cd43dd8448eb211c80319c" || sid != "b7ad6b7169203331" {
		t.Fatalf("roundtrip failed: tp=%q tid=%q sid=%q ok=%v", tp, tid, sid, ok)
	}
	if _, _, ok := ParseTraceparent("garbage"); ok {
		t.Fatal("garbage must not parse")
	}
}

func TestLogger_CarriesBaseFields(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(NewJSONHandler(&buf)))
	ctx := WithModule(WithTrace(context.Background(), "trace-1", "span-1"), "crm-core")
	Logger(ctx).Info("did a thing")

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("log line not JSON: %v\n%s", err, buf.String())
	}
	for _, k := range []string{"trace_id", "span_id", "module"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing base field %q in %v", k, m)
		}
	}
	if m["trace_id"] != "trace-1" {
		t.Fatalf("trace_id=%v want trace-1", m["trace_id"])
	}
}
