package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/olekukonko/errors"
)

func main() {
	// 1. Basic error with stack
	fmt.Println("=== Basic Error ===")
	err := errors.New("basic error").With("key", "value").Wrap(errors.ErrNotFound)
	log.Printf("Error: %v", err)
	log.Printf("Stack: %+v", err.Stack())

	// 2. Templated error
	fmt.Println("\n=== Templated Error ===")
	err = errors.ErrDatabase("connection failed")
	log.Printf("Database error: %v", err)
	log.Printf("Count: %d", err.Count())

	// 3. Coded error
	fmt.Println("\n=== Coded Error ===")
	err = errors.ErrAuth("user123", "invalid credentials")
	log.Printf("Auth error: %v", err)
	log.Printf("Code: %d", err.Code())
	log.Printf("Count: %d", err.Count())

	// 4. Function error
	fmt.Println("\n=== Function Error ===")
	err = errors.Func(exampleFunc, "processing failed")
	log.Printf("Func error: %v", err)

	// 5. Monitoring threshold
	fmt.Println("\n=== Threshold Monitoring ===")
	errors.SetThreshold("ErrDatabase", 2)
	go func() {
		ch := errors.Monitor("ErrDatabase")
		for err := range ch {
			log.Printf("Alert! Database error count: %d, Last error: %v", err.Count(), err)
		}
	}()
	for i := 0; i < 3; i++ {
		_ = errors.ErrDatabase(fmt.Sprintf("attempt %d", i))
		time.Sleep(100 * time.Millisecond) // Simulate work
	}

	// 6. Last error
	fmt.Println("\n=== Last Error ===")
	lastErr := errors.GetLastError("ErrDatabase")
	log.Printf("Last database error: %v", lastErr)

	// 7. JSON serialization
	fmt.Println("\n=== JSON Serialization ===")
	jsonBytes, _ := json.MarshalIndent(err, "", "  ")
	log.Printf("JSON: %s", string(jsonBytes))

	time.Sleep(1 * time.Second) // Allow goroutine to print
}

func exampleFunc() {}
