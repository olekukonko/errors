package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func processData(id string, attempt int) error {
	return errors.New("processing failed").
		With("id", id).
		With("attempt", attempt).
		With("retryable", true)
}

func main() {
	// 1. Basic context example
	err := processData("123", 3)
	fmt.Println("Error:", err)
	fmt.Println("Full context:", errors.Context(err)) // map[id:123 attempt:3 retryable:true]

	// 2. Accessing context through conversion
	rawErr := fmt.Errorf("wrapped: %w", err)
	fmt.Println("\nAfter wrapping with fmt.Errorf:")
	fmt.Println("Direct context access:", errors.Context(rawErr)) // nil (needs conversion)

	e := errors.Convert(rawErr)
	fmt.Println("After conversion - context:", e.Context()) // Original context preserved

	// 3. Standard library errors
	stdErr := fmt.Errorf("standard error")
	if errors.Context(stdErr) == nil {
		fmt.Println("\nStandard library errors have no context")
	}

	// 4. Adding context to standard errors
	converted := errors.Convert(stdErr).
		With("source", "legacy").
		With("severity", "high")
	fmt.Println("\nConverted standard error:")
	fmt.Println("Message:", converted.Error())   // "standard error"
	fmt.Println("Context:", converted.Context()) // map[source:legacy severity:high]

	// 5. Complex context example
	complexErr := errors.New("database operation failed").
		With("query", "SELECT * FROM users").
		With("params", map[string]interface{}{
			"limit":  100,
			"offset": 0,
		}).
		With("duration_ms", 45.2)

	fmt.Println("\nComplex error context:")
	for k, v := range errors.Context(complexErr) {
		fmt.Printf("%s: %v (%T)\n", k, v, v)
	}
}
