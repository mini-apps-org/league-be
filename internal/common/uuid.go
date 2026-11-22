package common

import "github.com/google/uuid"

func ValidateUUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return err
	}
	return nil
}
