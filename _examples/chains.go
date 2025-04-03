package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func databaseQuery() error {
	return errors.New("connection timeout").
		With("timeout_sec", 5).
		With("server", "db01.prod")
}

func businessLogic(userID string) error {
	err := databaseQuery()
	if err != nil {
		// Create new error with JUST the business logic context
		return errors.New("failed to process user "+userID).
			With("user_id", userID).
			With("stage", "processing").
			Wrap(err) // Wrap original error
	}
	return nil
}

func apiHandler() error {
	err := businessLogic("12345")
	if err != nil {
		// Create new error with JUST the API-level context
		return errors.New("API request failed").
			WithCode(500).
			WithStack().
			Wrap(err) // Wrap business error
	}
	return nil
}

func main() {
	err := apiHandler()

	fmt.Println("=== Combined Message ===")
	fmt.Println(err)

	fmt.Println("\n=== Error Chain ===")
	for i, e := range errors.UnwrapAll(err) {
		fmt.Printf("%d. %T\n", i+1, e)
		if err, ok := e.(*errors.Error); ok {
			fmt.Println(err.Format())
		} else {
			fmt.Println(e)
		}
	}

	fmt.Println("\n=== Error Checks ===")
	if errors.Is(err, errors.New("connection timeout")) {
		fmt.Println("✓ Matches connection timeout error")
	}
}
