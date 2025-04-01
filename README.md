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
// Simple error with automatic stack trace
err := errors.New("database connection failed")
fmt.Println(errors.Stack(err)) // Detailed stack trace

// Formatted error
err = errors.Newf("invalid value %q at position %d", "nil", 3)

// Named error type
err = errors.Named("AuthError")

// High-performance error (no stack trace)
err = errors.Fast("critical path failure") 
```

### Contextual Errors

```go
// Add structured context
err := errors.New("file operation failed").
    With("filename", "data.json").
    With("attempt", 3).
    With("retryable", true)

// Extract context
if ctx := errors.Context(err); ctx["retryable"] == true {
    // Retry logic
}
```

### Error Management

```go
// Configure global settings
errors.Configure(errors.Config{
    StackDepth:   64,      // Max stack frames
    DisableStack: false,   // Enable traces
    FilterInternal: true,  // Hide internal frames
})

// Template with status code
var ErrNotFound = errors.Coded("NotFound", 404, "%s not found")
err := ErrNotFound("user profile")
fmt.Println(err.Code()) // 404
```

## Core Concepts

### Stack Traces

```go
err := errors.New("example").Trace()

// Detailed trace
for _, frame := range err.Stack() {
    fmt.Printf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
}

// Lightweight trace
fmt.Println(err.FastStack()) // ["file.go:123", "main.go:42"]
```

### Error Pooling (Automatic)

```go
// Enabled by default in Go 1.24+
err := errors.New("temporary")
// Automatically returns to pool when unreachable

// Manual control (optional)
err.Free() // Explicit return to pool
```

### Error Monitoring

```go
// Track error occurrences
monitor := errors.NewMonitor("DBError", 5) // Threshold=5
defer monitor.Close()

go func() {
    for alert := range monitor.Alerts() {
        fmt.Printf("ALERT: %s occurred %d times\n", 
            alert.Name, alert.Count)
    }
}()

// Simulate errors
for i := 0; i < 10; i++ {
    errors.Named("DBError").Increment()
}
```

## Advanced Usage

### Retry Mechanism

```go
retry := errors.NewRetry(
    errors.WithBackoff(100*time.Millisecond),
    errors.WithJitter(50*time.Millisecond),
)

err := retry.Execute(func() error {
    return callFlakyService()
})

if errors.IsRetryable(err) {
    // Handle retry exhaustion
}
```

### Multi-Error Aggregation

```go
batch := errors.NewBatch()

batch.Add(validateInput(input))
batch.Add(checkPermissions(user)))
batch.Add(updateDatabase(record)))

if batch.Failed() {
    fmt.Printf("Failed with %d errors:\n%v", 
        batch.Count(), batch)
}
```

### HTTP Error Handling

```go
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := fetchUser(r.URL.Query().Get("id"))
    if err != nil {
        err := errors.Wrap(err, "getUserHandler").
            With("path", r.URL.Path).
            WithCode(404)
            
        w.WriteHeader(err.Code())
        json.NewEncoder(w).Encode(err)
        return
    }
    // ...
}
```

## Performance Optimization

### Configuration Tuning

```go
// For high-throughput services
errors.Configure(errors.Config{
    DisableStack:   true,  // Skip traces
    DisablePooling: false, // Enable pooling
    ContextSize:    1,     // Optimize for 1-2 context items
})
```

### Memory Efficiency

```go
type contextItem struct {
    key   string
    value interface{}
}

// Memory layout (optimized)
type Error struct {
    // Hot path (frequently accessed)
    msg      string
    stack    []uintptr
    
    // Cold path
    smallContext [2]contextItem // No allocation for ≤2 items
    // ...
}
```

## Migration Guide

### From Standard Errors

1. Replace `fmt.Errorf()` with `errors.Newf()`
2. Convert `errors.New()` to `errors.New()` as normal
3. Add context with `.With()` instead of string formatting
4. Use `errors.Is()`/`errors.As()` as normal

### From pkg/errors

1. Replace `Wrap()` with `errors.Wrapf()`
2. Use `errors.Stack()` instead of `StackTrace()`
3. Enable pooling for better performance
4. Leverage structured context instead of error messages

## FAQ

**Q: When should I call Free()?**  
A: Only when you need deterministic memory reclamation. The pool automatically handles cleanup in Go 1.24+.

**Q: How to disable stack traces?**
```go
errors.Configure(errors.Config{DisableStack: true})
```

**Q: Can I use this with middleware?**
```go
func ErrorMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
defer func() {
if err := recover(); err != nil {
errors.New("panic").With("path", r.URL.Path)
// ...
}
}()
next.ServeHTTP(w, r)
})
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) for details.