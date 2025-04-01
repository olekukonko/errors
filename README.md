# Enhanced Error Handling for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/olekukonko/errors.svg)](https://pkg.go.dev/github.com/olekukonko/errors)
[![Go Report Card](https://goreportcard.com/badge/github.com/olekukonko/errors)](https://goreportcard.com/report/github.com/olekukonko/errors)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A dead simple yet powerful drop-in replacement for Go's standard errors package with stack traces, error templates, monitoring capabilities, and performance optimizations.

## Features

- **Stack Traces**: Automatic capture with configurable skipping
- **Error Templates**: Predefined error formats with placeholders
- **Error Monitoring**: Count occurrences and set alert thresholds
- **Performance Optimized**:
    - Object pooling
    - Small context caching
    - Lazy stack capture
- **Context Support**: Attach key-value pairs to errors
- **Error Chaining**: Wrap and unwrap error causes
- **HTTP Status Codes**: Associate errors with HTTP codes
- **JSON Support**: Full error serialization
- **Compatibility**: Implements standard `error` interface and works with `errors.Is/As`

## Installation

```bash
go get github.com/olekukonko/errors
```

## Quick Start

### Basic Usage

```go
package main

import (
	"fmt"
	"github.com/olekukonko/errors"
)

func main() {
	// Simple error with stack trace
	err := errors.New("something went wrong")
	fmt.Println(err)
	fmt.Println(err.Stack())

	// Formatted error
	err = errors.Newf("invalid value: %d", 42)
	
	// Named error
	err = errors.Named("ValidationError")
	
	// Fast error (no stack trace)
	err = errors.FastNew("critical path error")
}
```

##### or 

```go

package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

var ErrNetwork = errors.Define("ErrNetwork", "network error: %s")

func main() {
    err := riskyOperation()
    if errors.IsError(err) {
        fmt.Printf("%s\nCount: %d\n", err, errors.Count(err))
        fmt.Println("Stack:", errors.Stack(err))
    } else {
        fmt.Println("Standard error:", err)
    }
}

func riskyOperation() error {
    return ErrNetwork("connection timeout").
        With("endpoint", "api.example.com").
        With("timeout", 30)
}
```


```go

err := errors.New("example").With("key", "value")

if errors.IsEnhanced(err) {
    fmt.Println("Stack:", errors.Stack(err))
    fmt.Println("Context:", errors.Context(err))
    fmt.Println("Name:", errors.Name(err))
    fmt.Println("Count:", errors.Count(err))
    fmt.Println("HTTP Code:", errors.Code(err))
}
```
### Error Templates

```go
// Define an error template
var ErrDatabase = errors.Define("ErrDatabase", "database error: %s")

// Use the template
func queryDB() error {
	return ErrDatabase("connection timeout")
}
```

### Error Context

```go
err := errors.New("file operation failed").
	With("filename", "data.json").
	With("size", 1024)
	
// Get context later
ctx := err.(*errors.Error).Context()
```

### Error Monitoring

```go
// Set threshold for alerts
errors.SetThreshold("ErrDatabase", 10) // Alert after 10 occurrences

// Monitor errors
monitor := errors.Monitor("ErrDatabase")
go func() {
	for err := range monitor {
		fmt.Printf("ALERT: Too many database errors! Latest: %v\n", err)
	}
}()

// Trigger threshold
for i := 0; i < 15; i++ {
	_ = ErrDatabase("connection failed")
}
```

### HTTP Error Codes

```go
// Create error with HTTP status code
var ErrNotFound = errors.Coded("ErrNotFound", 404, "%s not found")

// Get the code later
err := ErrNotFound("user profile")
code := err.(*errors.Error).Code() // Returns 404
```

## Performance Tuning

Configure global settings for performance:

```go
// Disable stack traces (faster)
errors.DisableStack = true

// Disable error registry (faster, no counting/monitoring)
errors.DisableRegistry = true

// Disable object pooling (for debugging)
errors.DisablePooling = true
```

## Benchmarks

The package is highly optimized. Some benchmark results:

| Operation                | Time/Op | Allocs/Op |
|--------------------------|---------|-----------|
| New (with stack)         | 221ns   | 1         |
| New (no stack)           | 11.2ns  | 0         |
| FastNew                  | 9.99ns  | 0         |
| WithContext              | 14.8ns  | 0         |
| Error.Is                 | 5.53ns  | 0         |
| Error.Count              | 9.75ns  | 0         |

## Advanced Features

### Custom Error Functions

```go
var ErrComplex = errors.Callable("ErrComplex", func(args ...interface{}) *errors.Error {
	// Custom error creation logic
	return errors.Newf("complex error: %v", args)
})

// Usage
err := ErrComplex("data", 42, true)
```

### JSON Serialization

```go
err := errors.New("example").With("key", "value")
jsonData, _ := json.Marshal(err)
// {"message":"example","context":{"key":"value"},"stack":[...]}
```

### Error Pooling

Errors are pooled by default. Remember to Free() them when done:

```go
err := errors.New("temporary")
defer err.Free() // Returns to pool
```

## Predefined Errors

Common HTTP errors are predefined:

```go
errors.ErrNotFound        // 404
errors.ErrPermission      // 403
errors.ErrInvalidArgument // 400
errors.ErrAuth            // 401
```

## Why Use This Package?

- **More informative errors** with stack traces and context
- **Better debugging** with rich error information
- **Production monitoring** with error counting and alerts
- **High performance** through careful optimization
- **Drop-in replacement** works with standard error handling

## License

MIT. See LICENSE file for details.