package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// ErrorCode is a 6-digit number: the first three digits are the HTTP status the
// error is served with, the last three are the case within that status. So 403002
// is "the second distinct 403". The frontend switches on these values, so a
// code may be added but never reassigned to a different meaning.
//
// This block is the whole registry for the repo. Allocate a new code by taking
// the next free number for its status; never renumber one that has shipped.
type ErrorCode int64

const (
	// 400 — the request itself is malformed.
	CodeValidationFailed ErrorCode = 400001

	// 401 — the caller is not identified.
	CodeNoSession       ErrorCode = 401001
	CodeSessionInvalid  ErrorCode = 401002
	CodeNoUserInContext ErrorCode = 401003

	// 403 — identified, but the request is refused.
	CodeCSRFCookieMissing ErrorCode = 403001
	CodeCSRFTokenMismatch ErrorCode = 403002
	// CodePermissionDenied: the caller's roles lack the route's permission.
	// Distinct from the CSRF 403s, which mean "reload the page".
	CodePermissionDenied ErrorCode = 403005

	// 404
	CodeNotFound ErrorCode = 404001

	// 409
	CodeConflict ErrorCode = 409001

	// 5xx
	CodeInternal ErrorCode = 500001
)

// errorDetail is the two texts every code carries. reason is for us: a stable
// slug that goes to the log and to the API docs, naming the exact failure.
// message is for the browser, so it must never name an internal cause.
type errorDetail struct {
	reason  string
	message string
}

var errorDetails = map[ErrorCode]errorDetail{
	CodeValidationFailed: {"VALIDATION_FAILED", "Some of the submitted data is invalid."},

	CodeNoSession:       {"SESSION_COOKIE_ABSENT", "Your session has expired. Please sign in again."},
	CodeSessionInvalid:  {"SESSION_INVALID", "Your session has expired. Please sign in again."},
	CodeNoUserInContext: {"USER_NOT_IN_CONTEXT", "Your session has expired. Please sign in again."},

	CodeCSRFCookieMissing: {"CSRF_COOKIE_ABSENT", "That request could not be completed. Reload the page and try again."},
	CodeCSRFTokenMismatch: {"CSRF_TOKEN_MISMATCH", "That request could not be completed. Reload the page and try again."},
	CodePermissionDenied:  {"PERMISSION_DENIED", "You do not have permission to do that."},

	CodeNotFound: {"NOT_FOUND", "That item no longer exists."},

	CodeConflict: {"CONFLICT", "That change conflicts with something that already exists."},

	CodeInternal: {"INTERNAL", "Something went wrong. Please try again."},
}

// detailFor never returns a zero value: an unregistered code would otherwise
// serve an empty message to the client.
func detailFor(code ErrorCode) errorDetail {
	if detail, ok := errorDetails[code]; ok {
		return detail
	}
	return errorDetails[CodeInternal]
}

func messageFor(code ErrorCode) string {
	return detailFor(code).message
}

func reasonFor(code ErrorCode) string {
	return detailFor(code).reason
}

// Status reads the HTTP status off the code's first three digits. A code that
// is not 6 digits, or whose prefix is not a 4xx or 5xx status, is served as 500 —
// the safe direction for a code nobody registered.
func (c ErrorCode) Status() int {
	if c < 400000 || c > 599999 {
		return http.StatusInternalServerError
	}
	return int(c / 1000)
}

// APIError is an error that knows how it should be served. Err is logged, never
// serialized, hence no json tags: the wire shape is ErrorResponse's job.
type APIError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%d %s: %s: %v", e.Code, reasonFor(e.Code), e.Message, e.Err)
	}
	return fmt.Sprintf("%d %s: %s", e.Code, reasonFor(e.Code), e.Message)
}

func (e *APIError) Status() int {
	return e.Code.Status()
}

// ErrorResponse is the wire format for every failed request: {"error":{...}}.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    ErrorCode `json:"code" example:"404001"`
	Message string    `json:"message" example:"That item no longer exists."`
}

// Error builds an APIError with the code's default message, keeping cause for
// the log.
func Error(code ErrorCode, cause error) *APIError {
	return &APIError{
		Code:    code,
		Message: messageFor(code),
		Err:     cause,
	}
}

// ErrorWithMessage overrides the default text; never to surface an internal detail.
func ErrorWithMessage(code ErrorCode, message string, cause error) *APIError {
	return &APIError{Code: code, Message: message, Err: cause}
}

// ReturnWithError serves err as the ErrorResponse envelope. Anything that is not
// an *APIError becomes 500001, so an unclassified failure cannot leak its text.
func (h *Handler) ReturnWithError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)
	status := apiErr.Status()

	// Below 500 is the caller's mistake, not ours. A 404 or a missing CSRF token
	// logged at error level makes a healthy service look like it is on fire.
	level := slog.LevelError
	if status < http.StatusInternalServerError {
		level = slog.LevelWarn
	}

	slog.LogAttrs(r.Context(), level, "request failed",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", status),
		slog.Int64("code", int64(apiErr.Code)),
		slog.String("reason", reasonFor(apiErr.Code)),
		slog.Any("err", apiErr.Err),
	)

	h.ReturnWithJSON(w, status, ErrorResponse{
		Error: ErrorBody{Code: apiErr.Code, Message: apiErr.Message},
	})
}

// AsAPIError resolves any error to the APIError it should be served as.
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return Error(CodeInternal, err)
}
