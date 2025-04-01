// Package errmgr provides common error definitions and categories for use across applications.
// These predefined errors are designed for consistency in error handling and can be used
// directly as immutable instances or copied for customization using Copy().
package errmgr

import "github.com/olekukonko/errors"

// Common error categories used for organizing errors across different domains.
const (
	CategoryAuth       errors.ErrorCategory = "auth"       // Authentication-related errors (e.g., login failures)
	CategoryDatabase   errors.ErrorCategory = "database"   // Database-related errors (e.g., connection issues)
	CategoryNetwork    errors.ErrorCategory = "network"    // Network-related errors (e.g., timeouts, unreachable hosts)
	CategoryIO         errors.ErrorCategory = "io"         // Input/Output-related errors (e.g., file operations)
	CategoryValidation errors.ErrorCategory = "validation" // Validation-related errors (e.g., invalid input)
	CategoryBusiness   errors.ErrorCategory = "business"   // Business logic errors (e.g., rule violations)
	CategorySystem     errors.ErrorCategory = "system"     // System-level errors (e.g., resource exhaustion)
)

// Common HTTP status codes used for error responses, aligned with REST API conventions.
const (
	CodeBadRequest         = 400 // HTTP 400 Bad Request (client error, invalid input)
	CodeUnauthorized       = 401 // HTTP 401 Unauthorized (authentication required)
	CodeForbidden          = 403 // HTTP 403 Forbidden (access denied)
	CodeNotFound           = 404 // HTTP 404 Not Found (resource not found)
	CodeInternalError      = 500 // HTTP 500 Internal Server Error (server failure)
	CodeConflict           = 409 // HTTP 409 Conflict (resource conflict)
	CodeTooManyRequests    = 429 // HTTP 429 Too Many Requests (rate limiting)
	CodeNotImplemented     = 501 // HTTP 501 Not Implemented (feature not supported)
	CodeServiceUnavailable = 503 // HTTP 503 Service Unavailable (temporary unavailability)
)

// Generic Predefined Errors
var (
	ErrInvalidArg = errors.New("invalid argument")
	ErrNotFound   = errors.New("not found")
	ErrPermission = errors.New("permission denied")
	ErrTimeout    = errors.New("operation timed out").WithTimeout()
	ErrUnknown    = errors.New("unknown error")
)

// Authentication Errors
var (
	ErrAuthFailed   = Coded("ErrAuthFailed", CodeUnauthorized, "authentication failed for %s: %s")
	ErrInvalidToken = errors.New("invalid authentication token").WithCode(CodeUnauthorized)
	ErrTokenExpired = errors.New("authentication token expired").WithCode(CodeUnauthorized)
	ErrMissingCreds = errors.New("missing credentials").WithCode(CodeBadRequest)
)

// Database Errors
var (
	ErrDBConnection = Coded("ErrDBConnection", CodeInternalError, "database connection failed: %s")
	ErrDBQuery      = Coded("ErrDBQuery", CodeInternalError, "database query failed: %s")
	ErrDBTimeout    = errors.New("database operation timed out").WithCode(CodeInternalError).WithRetryable()
	ErrDBConstraint = Coded("ErrDBConstraint", CodeBadRequest, "database constraint violation: %s")
)

// Network Errors
var (
	ErrNetworkUnreachable = Coded("ErrNetworkUnreachable", CodeInternalError, "network unreachable: %s")
	ErrNetworkTimeout     = errors.New("network timeout").WithCode(CodeInternalError).WithRetryable()
	ErrNetworkConnRefused = errors.New("connection refused").WithCode(CodeInternalError)
)

// IO Errors
var (
	ErrIORead       = Coded("ErrIORead", CodeInternalError, "I/O read error: %s")
	ErrIOWrite      = Coded("ErrIOWrite", CodeInternalError, "I/O write error: %s")
	ErrFileNotFound = errors.New("file not found").WithCode(CodeNotFound)
)

// Validation Errors
var (
	ErrValidationFailed = Coded("ErrValidationFailed", CodeBadRequest, "validation failed: %s")
	ErrInvalidFormat    = Coded("ErrInvalidFormat", CodeBadRequest, "invalid format: %s")
)

// Business Logic Errors
var (
	ErrBusinessRule      = Coded("ErrBusinessRule", CodeBadRequest, "business rule violation: %s")
	ErrInsufficientFunds = errors.New("insufficient funds").WithCode(CodeBadRequest)
)

// System Errors
var (
	ErrSystemFailure     = Coded("ErrSystemFailure", CodeInternalError, "system failure: %s")
	ErrResourceExhausted = errors.New("resource exhausted").WithCode(CodeInternalError)
)

// Additional HTTP-related Errors
var (
	ErrConflict           = Coded("ErrConflict", CodeConflict, "conflict occurred: %s")
	ErrRateLimitExceeded  = Coded("ErrRateLimitExceeded", CodeTooManyRequests, "rate limit exceeded: %s")
	ErrNotImplemented     = errors.New("not implemented").WithCode(CodeNotImplemented)
	ErrServiceUnavailable = errors.New("service unavailable").WithCode(CodeServiceUnavailable)
)

// Additional Domain-Specific Errors
var (
	ErrSerialization        = Coded("ErrSerialization", CodeBadRequest, "serialization error: %s")
	ErrDeserialization      = Coded("ErrDeserialization", CodeBadRequest, "deserialization error: %s")
	ErrExternalService      = errors.New("external service error").WithCode(CodeInternalError).WithRetryable()
	ErrUnsupportedOperation = errors.New("unsupported operation").WithCode(CodeBadRequest)
)

// Predefined Templates with Categories
var (
	AuthFailed      = Categorized(CategoryAuth, "AuthFailed", "authentication failed for %s: %s")
	DBError         = Categorized(CategoryDatabase, "DBError", "database error: %s")
	NetworkError    = Categorized(CategoryNetwork, "NetworkError", "network failure: %s")
	IOError         = Categorized(CategoryIO, "IOError", "I/O error: %s")
	ValidationError = Categorized(CategoryValidation, "ValidationError", "validation error: %s")
	BusinessError   = Categorized(CategoryBusiness, "BusinessError", "business error: %s")
	SystemError     = Categorized(CategorySystem, "SystemError", "system error: %s")
)
