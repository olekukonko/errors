# Enhanced Error Handling for Go with Stack Traces, Monitoring, and More

[![Go Reference](https://pkg.go.dev/badge/github.com/olekukonko/errors.svg)](https://pkg.go.dev/github.com/olekukonko/errors)
[![Go Report Card](https://goreportcard.com/badge/github.com/olekukonko/errors)](https://goreportcard.com/report/github.com/olekukonko/errors)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Benchmarks](https://img.shields.io/badge/benchmarks-included-success)](README.md#performance)

A dead simple, production-grade error handling library for Go with zero-cost abstractions, stack traces, automatic monitoring, and more.

## Features

✔ **Runtime Efficiency**  
• Memory pooling (optional)  
• Lazy stack trace collection  
• Small context optimization (≤2 items)  
• Lock-free configuration

✔ **Debugging Capabilities**  
• Full stack traces with filtering  
• Error chaining/wrapping  
• Structured context attachment  
• JSON serialization

✔ **Production Monitoring**  
• Error occurrence counting  
• Threshold-based alerts  
• Categorized metrics  
• Template-based definitions

✔ **Advanced Patterns**  
• Automatic retry mechanism  
• Multi-error aggregation  
• HTTP status code support  
• Callback triggers


## Installation

```bash
go get github.com/olekukonko/errors@latest
```

## Quick Start

### Basic Error Creation

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Simple error (no stack trace by default)
    err := errors.New("database connection failed")
    fmt.Println(err) // "database connection failed"

    // Error with stack trace
    err = errors.Trace("database connection failed")
    fmt.Println(errors.Stack(err)) // Detailed stack trace

    // Formatted error
    err = errors.Newf("invalid value %q at position %d", "nil", 3)
    fmt.Println(err) // "invalid value \"nil\" at position 3"

    // Named error with stack trace
    err = errors.Named("AuthError")
    fmt.Println(err) // "AuthError"
}
```

### Contextual Errors

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Add structured context (thread-safe)
    err := errors.New("file operation failed").
        With("filename", "data.json").
        With("attempt", 3).
        With("retryable", true)

    // Extract context
    if ctx := errors.Context(err); ctx != nil && ctx["retryable"] == true {
        fmt.Println("Retrying due to:", err)
    }
}
```

### Error Management

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
    "github.com/olekukonko/errors/errmgr"
)

func main() {
    // Configure global settings
    errors.Configure(errors.Config{
        StackDepth:     64,     // Max stack frames
        DisablePooling: false,  // Enable pooling by default
        FilterInternal: true,   // Hide internal frames
    })

    // Template with status code
    var ErrNotFound = errmgr.Coded("NotFound", "resource %s not found", 404)
    err := ErrNotFound("user profile")
    fmt.Println(err)        // "resource user profile not found"
    fmt.Println(err.Code()) // 404
}
```

## Core Concepts

### Stack Traces

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    err := errors.New("example").WithStack()

    // Detailed trace
    for _, frame := range err.Stack() {
        fmt.Println(frame) // e.g., "main.main\n\tmain.go:10"
    }

    // Lightweight trace
    fmt.Println(err.FastStack()) // e.g., ["main.go:10", "caller.go:42"]
}
```

### Error Pooling (Automatic)

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Enabled by default in Go 1.24+ with autofree
    err := errors.New("temporary")
    fmt.Println(err) // "temporary"
    // Automatically returns to pool when unreachable (GC-triggered)

    // Manual control (recommended for deterministic cleanup)
    err = errors.New("manual")
    fmt.Println(err) // "manual"
    err.Free()       // Explicitly return to pool
}
```

### Error Monitoring

```go
package main

import (
    "fmt"
    "time"
    "github.com/olekukonko/errors"
    "github.com/olekukonko/errors/errmgr"
)

func main() {
    // Define a templated error
    dbError := errmgr.Define("DBError", "database error: %s")

    // Set up monitoring for "DBError"
    monitor := errmgr.NewMonitor("DBError")
    errmgr.SetThreshold("DBError", 5) // Alert after 5 occurrences
    defer monitor.Close()

    go func() {
        for alert := range monitor.Alerts() {
            fmt.Printf("ALERT: %s occurred %d times\n", 
                alert.Name(), alert.Count())
        }
    }()

    // Simulate errors with the same name
    for i := 0; i < 10; i++ {
        err := dbError(fmt.Sprintf("simulated failure %d", i))
        err.Free()
    }

    // Give the goroutine time to process alerts
    time.Sleep(100 * time.Millisecond)
}
```

## Advanced Usage

### Retry Mechanism

```go
package main

import (
    "fmt"
    "time"
    "github.com/olekukonko/errors"
)

func callFlakyService() error {
    return errors.New("service unavailable").WithRetryable()
}

func main() {
    retry := errors.NewRetry(
        errors.WithDelay(100*time.Millisecond),
        errors.WithJitter(true),
        errors.WithMaxAttempts(3),
        errors.WithOnRetry(func(attempt int, err error) {
            fmt.Printf("Attempt %d failed: %v\n", attempt, err)
        }),
    )

    err := retry.Execute(func() error {
        return callFlakyService()
    })

    if err != nil {
        fmt.Println("Retry exhausted with error:", err)
    } else {
        fmt.Println("Service call succeeded")
    }
}
```

### Multi-Error Aggregation

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func validateInput(input string) error {
    return errors.New("invalid input")
}

func checkPermissions(user string) error {
    return nil
}

func updateDatabase(record string) error {
    return errors.New("db update failed")
}

func main() {
    multi := errors.NewMultiError()

    multi.Add(validateInput("input"))
    multi.Add(checkPermissions("user"))
    multi.Add(updateDatabase("record"))

    if multi.Has() {
        fmt.Printf("Failed with %d errors:\n%v\n", 
            len(multi.Errors()), multi)
    }
}
```

### HTTP Error Handling

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/olekukonko/errors"
)

func fetchUser(id string) (string, error) {
    return "", errors.New("user not found")
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := fetchUser(r.URL.Query().Get("id"))
    if err != nil {
        err = errors.Wrap(err, "failed to fetch user").
            With("path", r.URL.Path).
            WithCode(404)

        w.WriteHeader(err.Code())
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error":   err.Error(),
            "context": err.Context(),
            "code":    err.Code(),
        })
        return
    }
    fmt.Fprintf(w, "User: %s", user)
}

func main() {
    http.HandleFunc("/user", getUserHandler)
    fmt.Println("Server running on :8080")
    http.ListenAndServe(":8080", nil)
}
```

### Error Callback

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Error with callback for logging or side effects
    count := 0
    err := errors.New("operation failed").
        Callback(func() {
            count++
            fmt.Printf("Error accessed %d times\n", count)
        })

    // Trigger callback each time Error() is called
    fmt.Println(err) // "operation failed" + "Error accessed 1 times"
    fmt.Println(err) // "operation failed" + "Error accessed 2 times"
    err.Free()
}
```

## Performance Optimization

### Configuration Tuning

```go
package main

import (
    "github.com/olekukonko/errors"
)

func main() {
    // For high-throughput services
    errors.Configure(errors.Config{
        DisablePooling: false, // Enable pooling for zero allocations
        ContextSize:    1,     // Optimize for minimal context
    })

    // Stack traces are opt-in
    err := errors.New("fast error") // No stack trace
    err = errors.Trace("debug error") // With stack trace
    err.Free()
}
```

### Memory Efficiency

```go
package main

import "github.com/olekukonko/errors"

// Memory layout (optimized)
type Error struct {
    // Hot path (frequently accessed)
    msg      string
    stack    []uintptr
    
    // Cold path
    smallContext [2]errors.contextItem // No allocation for ≤2 items
    context      map[string]interface{} // Lazy-allocated for >2 items
    // ...
}

func main() {
    err := errors.New("example").
        With("key1", "value1").
        With("key2", "value2") // Uses smallContext, no allocation
    err.Free()
}
```

## Migration Guide

### From Standard Errors

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Old: fmt.Errorf
    oldErr := fmt.Errorf("error: %s", "something went wrong")
    
    // New: errors.Newf
    newErr := errors.Newf("error: %s", "something went wrong").
        With("details", "extra info")
    
    fmt.Println(oldErr) // "error: something went wrong"
    fmt.Println(newErr) // "error: something went wrong"
    fmt.Println(newErr.Context()) // map[details:extra info]
}
```

### From pkg/errors

```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    // Old: pkg/errors.Wrap
    // err := pkgerrors.Wrap(errors.New("base"), "wrapped")
    
    // New: errors.Wrapf
    err := errors.Wrapf(errors.New("base"), "wrapped with %s", "details").
        WithStack()
    
    fmt.Println(err)        // "wrapped with details: base"
    fmt.Println(err.Stack()) // Stack trace
    err.Free()
}
```

## FAQ

**Q: When should I call Free()?**
```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    err := errors.New("example")
    fmt.Println(err) // "example"
    err.Free()       // Call Free() for immediate pool return
    // In Go 1.24+, autofree handles it when unreachable
}
```

**Q: How do I avoid stack traces for performance?**
```go
package main

import (
    "fmt"
    "github.com/olekukonko/errors"
)

func main() {
    err := errors.New("fast error") // No stack trace
    fmt.Println(err) // "fast error"
    
    err = errors.Trace("debug error") // With stack trace
    fmt.Println(err.Stack()) // Stack trace
    err.Free()
}
```

**Q: Can I use this with middleware?**
```go
package main

import (
    "fmt"
    "net/http"
    "github.com/olekukonko/errors"
)

func ErrorMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                e := errors.New("panic recovered").
                    With("path", r.URL.Path).
                    WithStack()
                fmt.Println(e) // Log or handle
                e.Free()
            }
        }()
        next.ServeHTTP(w, r)
    })
}

func main() {
    http.Handle("/", ErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        panic("test panic")
    })))
    http.ListenAndServe(":8080", nil)
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) for details.
