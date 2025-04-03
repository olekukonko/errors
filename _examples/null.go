package main

import (
	"database/sql"
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Case 1: Nil error
	var err error = nil
	if errors.IsNull(err) {
		fmt.Println("Nil error is null")
	}

	// Case 2: Empty error
	err = errors.New("")
	if errors.IsNull(err) {
		fmt.Println("Empty error is null")
	} else {
		fmt.Println("Empty error is not null")
	}

	// Case 3: Error with null context
	nullString := sql.NullString{Valid: false}
	err = errors.New("").With("data", nullString)
	if errors.IsNull(err) {
		fmt.Println("Error with null context is null")
	}

	// Case 4: Error with non-null context
	validString := sql.NullString{String: "test", Valid: true}
	err = errors.New("").With("data", validString)
	if errors.IsNull(err) {
		fmt.Println("Error with valid context is null")
	} else {
		fmt.Println("Error with valid context is not null")
	}

	// Case 5: Empty MultiError
	multi := errors.NewMultiError()
	if multi.IsNull() {
		fmt.Println("Empty MultiError is null")
	}

	// Case 6: MultiError with null error
	multi.Add(errors.New("").With("data", nullString))
	if multi.IsNull() {
		fmt.Println("MultiError with null error is null")
	}

	// Case 7: MultiError with non-null error
	multi.Add(errors.New("real error"))
	if multi.IsNull() {
		fmt.Println("MultiError with mixed errors is null")
	} else {
		fmt.Println("MultiError with mixed errors is not null")
	}
}
