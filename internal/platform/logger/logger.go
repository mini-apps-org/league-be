// Package logger builds the process logger: the one place that picks the
// handler, the level and the output stream.
package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

const envDevelopment = "development"

func New(env, level string) (*slog.Logger, error) {
	handler, err := newHandler(os.Stdout, env, level)

	return slog.New(handler), err
}

// newHandler takes a writer so tests need not swap os.Stdout.
func newHandler(w io.Writer, env, level string) (slog.Handler, error) {
	resolved, err := parseLevel(level)

	// Development only: GKE's agent drops a record split across physical lines.
	if env == envDevelopment {
		w = indentJSON{w: w}
	}

	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: resolved}), err
}

// indentJSON re-indents each record. An io.Writer rather than a slog.Handler
// because implementing Handler means getting WithGroup right, and this only
// reformats bytes that are already valid JSON. JSONHandler writes one whole
// record per call.
type indentJSON struct {
	w io.Writer
}

func (i indentJSON) Write(p []byte) (int, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimRight(p, "\n"), "", "  "); err != nil {
		return i.w.Write(p) // not JSON; pass it through rather than lose the line
	}
	out.WriteByte('\n')

	if _, err := i.w.Write(out.Bytes()); err != nil {
		return 0, err
	}

	return len(p), nil // input length: a short count reads as a failed write
}

// parseLevel falls back to info so the returned level is always usable.
func parseLevel(level string) (slog.Level, error) {
	if level == "" {
		return slog.LevelInfo, nil
	}

	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		return slog.LevelInfo, fmt.Errorf("logger: %q is not a level (debug, info, warn, error)", level)
	}

	return parsed, nil
}
