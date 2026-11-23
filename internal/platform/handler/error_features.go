package handler

import "errors"

// ErrorMapping is how one feature sentinel is served. An empty Message serves the
// code's default. Each feature declares its sentinels and their ErrorMappings in
// its own errors.go, so platform never needs to know a feature.
type ErrorMapping struct {
	Sentinel error
	Code     ErrorCode
	Message  string
}

// MapFeatureError serves the first sentinel of errorMappings found anywhere in
// err's chain with its code, keeping err as the logged cause. errorMappings is
// searched in order, so an error joining two sentinels always resolves to the
// same one. Any other error is returned unchanged and served as 500001.
func MapFeatureError(err error, errorMappings []ErrorMapping) error {
	if err == nil {
		return nil
	}

	for _, mapping := range errorMappings {
		if !errors.Is(err, mapping.Sentinel) {
			continue
		}
		if mapping.Message == "" {
			return Error(mapping.Code, err)
		}
		return ErrorWithMessage(mapping.Code, mapping.Message, err)
	}

	return err
}
