package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Simple error (no stack trace, fast)
	err := errors.New("connection failed")
	fmt.Println(err) // "connection failed"

	// Formatted error
	err = errors.Newf("user %s not found", "bob")
	fmt.Println(err) // "user bob not found"

	// Error with stack trace
	err = errors.Trace("critical issue")
	fmt.Println(err)         // "critical issue"
	fmt.Println(err.Stack()) // e.g., ["main.go:15", "caller.go:42"]

	// Named error
	err = errors.Named("InputError")
	fmt.Println(err.Name()) // "InputError"
}
