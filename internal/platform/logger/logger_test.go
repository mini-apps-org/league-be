package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

func TestRequestIDHandlerStampsRecords(t *testing.T) {
	var output bytes.Buffer
	handler, err := newHandler(&output, "production", "info")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(handler).With("component", "test")

	ctx := context.WithValue(context.Background(), config.ContextKeyRequestID, "req-123")
	logger.InfoContext(ctx, "with request")

	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("decode %q: %v", output.String(), err)
	}
	if record["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want req-123 (derived logger must keep the stamp)", record["request_id"])
	}

	output.Reset()
	logger.Info("no request")
	if bytes.Contains(output.Bytes(), []byte("request_id")) {
		t.Fatalf("record without a request context carries request_id: %s", output.String())
	}
}
