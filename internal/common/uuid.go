package common

import "github.com/google/uuid"

func ValidateUUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return err
	}
	return nil
}

// NormaliseUUID returns id in canonical form — lower case, hyphenated, no braces or urn
// prefix — the form Postgres returns, so an id from a request compares equal to the same
// id read back from a query.
func NormaliseUUID(id string) (string, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}
