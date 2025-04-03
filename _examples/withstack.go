package main

import (
	"fmt"
	"github.com/olekukonko/errors"
	"math/rand"
	"time"
)

func basicFunc() error {
	return fmt.Errorf("basic error")
}

func enhancedFunc() *errors.Error {
	return errors.New("enhanced error")
}

func main() {
	// 1. Package-level WithStack - works with ANY error type
	err1 := basicFunc()
	enhanced1 := errors.WithStack(err1) // Handles basic errors
	fmt.Println("Package-level WithStack:")
	fmt.Println(enhanced1.Stack())

	// 2. Method-style WithStack - only for *errors.Error
	err2 := enhancedFunc()
	enhanced2 := err2.WithStack() // More natural chaining
	fmt.Println("\nMethod-style WithStack:")
	fmt.Println(enhanced2.Stack())

	// 3. Combined usage in real-world scenario
	result := processData()
	if result != nil {
		// Use package-level when type is unknown
		stackErr := errors.WithStack(result)

		// Then use method-style for chaining
		finalErr := stackErr.
			With("timestamp", time.Now()).
			WithCode(500)

		fmt.Println("\nCombined Usage:")
		fmt.Println("Message:", finalErr.Error())
		fmt.Println("Context:", finalErr.Context())
		fmt.Println("Stack:")
		for _, frame := range finalErr.Stack() {
			fmt.Println(frame)
		}
	}
}

func processData() error {
	// Could return either basic or enhanced error
	if rand.Intn(2) == 0 {
		return fmt.Errorf("database error")
	}
	return errors.New("validation error").With("field", "email")
}
