package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func ParsePathUUID(r *http.Request, name string) (string, error) {
	id, err := uuid.Parse(mux.Vars(r)[name])
	if err != nil {
		return "", ErrorWithMessage(CodeValidationFailed, name+" must be a uuid", err)
	}
	return id.String(), nil
}

// InvalidJSONBodyError hides the decoder's message, which can echo the request body; the
// cause is still logged.
func InvalidJSONBodyError(cause error) error {
	return ErrorWithMessage(CodeValidationFailed,
		"request body is not valid JSON for this endpoint", cause)
}
