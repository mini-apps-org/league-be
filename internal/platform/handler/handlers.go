package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/nguyentanvinh/league-be/internal/platform/config"
)

type Handler struct {
	Config *config.Config
}

func (h *Handler) ReturnWithJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if statusCode == http.StatusNoContent {
		return
	}

	if resp, ok := v.(config.SuccessResponse); ok && resp.Count == nil {
		one := int64(1)
		resp.Count = &one
		v = resp
	}

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status is already on the wire, so logging is all that is left.
		slog.Error("handler: encode response failed", "type", fmt.Sprintf("%T", v), "err", err)
	}
}

// maxBodyBytes caps a request body. These endpoints carry a handful of short
// strings, so anything larger is a mistake or an attempt to make us allocate.
const maxBodyBytes = 1 << 20

// DecodeJSON reads exactly one JSON object and rejects unknown fields, so a
// mistyped field name fails loudly instead of being ignored.
func (h *Handler) DecodeJSON(w http.ResponseWriter, r *http.Request, dest any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, config.ContextKeyUserID, userID)
}

func (h *Handler) ExtractUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(config.ContextKeyUserID).(string)
	return userID, ok && userID != ""
}
