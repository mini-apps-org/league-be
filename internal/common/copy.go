package common

import (
	"fmt"

	"github.com/jinzhu/copier"
)

// CopyFieldsByName returns a T filled from source's fields of the same name and type. Fields
// only on one side are left alone, so a column added to a db struct reaches a response
// only once the response declares it.
//
// copier fails only on unusable arguments (nil, non-struct), a programming error, so it
// panics like regexp.MustCompile. A field renamed on one side is not an error: it is
// silently left zero, which each caller's test must catch.
func CopyFieldsByName[T any](source any) T {
	var target T
	if err := copier.Copy(&target, source); err != nil {
		panic(fmt.Sprintf("common: copy %T into %T: %v", source, target, err))
	}
	return target
}
