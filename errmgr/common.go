// Package errmgr provides common error definitions and categories for use across applications.
// These predefined errors are designed for consistency in error handling and can be used
// directly as immutable instances or copied for customization using Copy().
package errmgr

import (
	"github.com/olekukonko/errors"
)

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

// Generic Predefined Errors (Static)
var (
	ErrInvalidArg = errors.New("invalid argument")
	ErrNotFound   = errors.New("not found")
	ErrPermission = errors.New("permission denied")
	ErrTimeout    = errors.New("operation timed out").WithTimeout()
	ErrUnknown    = errors.New("unknown error")
)

// Authentication Errors (Templated)
var (
	ErrAuthFailed   = Define("ErrAuthFailed", "authentication failed for %s: %s")
	ErrInvalidToken = Define("ErrInvalidToken", "invalid authentication token: %s")
	ErrTokenExpired = Define("ErrTokenExpired", "authentication token expired: %s")
	ErrMissingCreds = Define("ErrMissingCreds", "missing credentials: %s")
)

// Database Errors (Templated)
var (
	ErrDBConnection = Define("ErrDBConnection", "database connection failed: %s")
	ErrDBQuery      = Define("ErrDBQuery", "database query failed: %s")
	ErrDBTimeout    = Define("ErrDBTimeout", "database operation timed out: %s")
	ErrDBConstraint = Define("ErrDBConstraint", "database constraint violation: %s")
)

// Network Errors (Templated)
var (
	ErrNetworkUnreachable = Define("ErrNetworkUnreachable", "network unreachable: %s")
	ErrNetworkTimeout     = Define("ErrNetworkTimeout", "network timeout: %s")
	ErrNetworkConnRefused = Define("ErrNetworkConnRefused", "connection refused: %s")
)

// IO Errors (Templated)
var (
	ErrIORead       = Define("ErrIORead", "I/O read error: %s")
	ErrIOWrite      = Define("ErrIOWrite", "I/O write error: %s")
	ErrFileNotFound = Define("ErrFileNotFound", "file (%s) not found")
)

// Validation Errors (Templated)
var (
	ErrValidationFailed = Define("ErrValidationFailed", "validation failed: %s")
	ErrInvalidFormat    = Define("ErrInvalidFormat", "invalid format: %s")
)

// Business Logic Errors (Templated)
var (
	ErrBusinessRule      = Define("ErrBusinessRule", "business rule violation: %s")
	ErrInsufficientFunds = Define("ErrInsufficientFunds", "insufficient funds: %s")
)

// System Errors (Templated)
var (
	ErrSystemFailure     = Define("ErrSystemFailure", "system failure: %s")
	ErrResourceExhausted = Define("ErrResourceExhausted", "resource exhausted: %s")
)

// Additional HTTP-related Errors (Templated)
var (
	ErrConflict           = Define("ErrConflict", "conflict occurred: %s")
	ErrRateLimitExceeded  = Define("ErrRateLimitExceeded", "rate limit exceeded: %s")
	ErrNotImplemented     = Define("ErrNotImplemented", "%s not implemented")
	ErrServiceUnavailable = Define("ErrServiceUnavailable", "service (%s) unavailable")
)

// Additional Domain-Specific Errors (Templated)
var (
	ErrSerialization        = Define("ErrSerialization", "serialization error: %s")
	ErrDeserialization      = Define("ErrDeserialization", "deserialization error: %s")
	ErrExternalService      = Define("ErrExternalService", "external service (%s) error")
	ErrUnsupportedOperation = Define("ErrUnsupportedOperation", "unsupported operation %s")
)

// Predefined Templates with Categories (Templated)
var (
	AuthFailed      = Categorized(CategoryAuth, "AuthFailed", "authentication failed for %s: %s")
	DBError         = Categorized(CategoryDatabase, "DBError", "database error: %s")
	NetworkError    = Categorized(CategoryNetwork, "NetworkError", "network failure: %s")
	IOError         = Categorized(CategoryIO, "IOError", "I/O error: %s")
	ValidationError = Categorized(CategoryValidation, "ValidationError", "validation error: %s")
	BusinessError   = Categorized(CategoryBusiness, "BusinessError", "business error: %s")
	SystemError     = Categorized(CategorySystem, "SystemError", "system error: %s")
)
