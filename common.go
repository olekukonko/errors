// Package errors provides common error definitions and categories for use across applications.
// These predefined errors are designed for consistency in error handling and can be used
// directly as immutable instances or copied for customization using Copy().
package errors

// ErrorCategory represents a category for classifying errors, aiding in organization and filtering.
type ErrorCategory string

// Common error categories used for organizing errors across different domains.
const (
	CategoryAuth       ErrorCategory = "auth"       // Authentication-related errors (e.g., login failures)
	CategoryDatabase   ErrorCategory = "database"   // Database-related errors (e.g., connection issues)
	CategoryNetwork    ErrorCategory = "network"    // Network-related errors (e.g., timeouts, unreachable hosts)
	CategoryIO         ErrorCategory = "io"         // Input/Output-related errors (e.g., file operations)
	CategoryValidation ErrorCategory = "validation" // Validation-related errors (e.g., invalid input)
	CategoryBusiness   ErrorCategory = "business"   // Business logic errors (e.g., rule violations)
	CategorySystem     ErrorCategory = "system"     // System-level errors (e.g., resource exhaustion)
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
// These are single, immutable instances for general use. Use Copy() to customize.
var (
	// ErrInvalidArg represents a generic invalid argument error.
	ErrInvalidArg = New("invalid argument")
	// ErrNotFound indicates a resource or entity was not found.
	ErrNotFound = New("not found")
	// ErrPermission denotes a lack of permission to perform an action.
	ErrPermission = New("permission denied")
	// ErrTimeout signals an operation exceeded its allotted time, marked as a timeout.
	ErrTimeout = New("operation timed out").WithTimeout()
	// ErrUnknown represents an unspecified or unexpected error.
	ErrUnknown = New("unknown error")
)

// Authentication Errors
// These errors are related to authentication failures and are coded with HTTP status codes.
var (
	// ErrAuthFailed indicates authentication failed for a user with a reason.
	ErrAuthFailed = Coded("ErrAuthFailed", CodeUnauthorized, "authentication failed for %s: %s")
	// ErrInvalidToken denotes an invalid authentication token.
	ErrInvalidToken = New("invalid authentication token").WithCode(CodeUnauthorized)
	// ErrTokenExpired signals an expired authentication token.
	ErrTokenExpired = New("authentication token expired").WithCode(CodeUnauthorized)
	// ErrMissingCreds indicates missing authentication credentials.
	ErrMissingCreds = New("missing credentials").WithCode(CodeBadRequest)
)

// Database Errors
// These errors pertain to database operations, often retryable or indicative of server issues.
var (
	// ErrDBConnection signals a failure to connect to the database.
	ErrDBConnection = Coded("ErrDBConnection", CodeInternalError, "database connection failed: %s")
	// ErrDBQuery indicates a database query execution failure.
	ErrDBQuery = Coded("ErrDBQuery", CodeInternalError, "database query failed: %s")
	// ErrDBTimeout denotes a database operation timing out, marked as retryable.
	ErrDBTimeout = New("database operation timed out").WithCode(CodeInternalError).WithRetryable()
	// ErrDBConstraint signals a database constraint violation (e.g., unique key).
	ErrDBConstraint = Coded("ErrDBConstraint", CodeBadRequest, "database constraint violation: %s")
)

// Network Errors
// These errors relate to network issues, often retryable due to transient conditions.
var (
	// ErrNetworkUnreachable indicates a network destination is unreachable.
	ErrNetworkUnreachable = Coded("ErrNetworkUnreachable", CodeInternalError, "network unreachable: %s")
	// ErrNetworkTimeout signals a network operation timeout, marked as retryable.
	ErrNetworkTimeout = New("network timeout").WithCode(CodeInternalError).WithRetryable()
	// ErrNetworkConnRefused denotes a refused network connection.
	ErrNetworkConnRefused = New("connection refused").WithCode(CodeInternalError)
)

// IO Errors
// These errors occur during input/output operations, such as file handling.
var (
	// ErrIORead indicates an error reading from a source.
	ErrIORead = Coded("ErrIORead", CodeInternalError, "I/O read error: %s")
	// ErrIOWrite signals an error writing to a destination.
	ErrIOWrite = Coded("ErrIOWrite", CodeInternalError, "I/O write error: %s")
	// ErrFileNotFound denotes a file not found error.
	ErrFileNotFound = New("file not found").WithCode(CodeNotFound)
)

// Validation Errors
// These errors arise from invalid input or data validation failures.
var (
	// ErrValidationFailed indicates a general validation failure.
	ErrValidationFailed = Coded("ErrValidationFailed", CodeBadRequest, "validation failed: %s")
	// ErrInvalidFormat signals data in an incorrect format.
	ErrInvalidFormat = Coded("ErrInvalidFormat", CodeBadRequest, "invalid format: %s")
)

// Business Logic Errors
// These errors stem from violations of business rules or conditions.
var (
	// ErrBusinessRule denotes a business rule violation.
	ErrBusinessRule = Coded("ErrBusinessRule", CodeBadRequest, "business rule violation: %s")
	// ErrInsufficientFunds indicates insufficient resources (e.g., money) for an operation.
	ErrInsufficientFunds = New("insufficient funds").WithCode(CodeBadRequest)
)

// System Errors
// These errors reflect system-level failures or resource issues.
var (
	// ErrSystemFailure indicates a general system failure.
	ErrSystemFailure = Coded("ErrSystemFailure", CodeInternalError, "system failure: %s")
	// ErrResourceExhausted signals exhaustion of system resources (e.g., memory, CPU).
	ErrResourceExhausted = New("resource exhausted: %s").WithCode(CodeInternalError)
)

// Additional HTTP-related Errors
// These errors map to specific HTTP status codes beyond the common ones.
var (
	// ErrConflict indicates a resource conflict (e.g., duplicate entry).
	ErrConflict = Coded("ErrConflict", CodeConflict, "conflict occurred: %s")
	// ErrRateLimitExceeded signals exceeding a rate limit.
	ErrRateLimitExceeded = Coded("ErrRateLimitExceeded", CodeTooManyRequests, "rate limit exceeded: %s")
	// ErrNotImplemented denotes an unimplemented feature.
	ErrNotImplemented = New("not implemented").WithCode(CodeNotImplemented)
	// ErrServiceUnavailable indicates temporary service unavailability.
	ErrServiceUnavailable = New("service unavailable: %s").WithCode(CodeServiceUnavailable)
)

// Additional Domain-Specific Errors
// These errors cover miscellaneous domain-specific cases.
var (
	// ErrSerialization indicates a serialization error.
	ErrSerialization = Coded("ErrSerialization", CodeBadRequest, "serialization error: %s")
	// ErrDeserialization signals a deserialization error.
	ErrDeserialization = Coded("ErrDeserialization", CodeBadRequest, "deserialization error: %s")
	// ErrExternalService denotes an error from an external service, marked as retryable.
	ErrExternalService = New("external service error: %s").WithCode(CodeInternalError).WithRetryable()
	// ErrUnsupportedOperation indicates an operation is not supported.
	ErrUnsupportedOperation = New("unsupported operation").WithCode(CodeBadRequest)
)

// Predefined Templates with Categories
// These are categorized error templates for specific domains, enhancing error classification.
var (
	// AuthFailed is a categorized authentication failure error.
	AuthFailed = Categorized(CategoryAuth, "AuthFailed", "authentication failed for %s: %s")
	// DBError is a general categorized database error.
	DBError = Categorized(CategoryDatabase, "DBError", "database error: %s")
	// NetworkError is a general categorized network error.
	NetworkError = Categorized(CategoryNetwork, "NetworkError", "network failure: %s")
	// IOError is a general categorized I/O error.
	IOError = Categorized(CategoryIO, "IOError", "I/O error: %s")
	// ValidationError is a general categorized validation error.
	ValidationError = Categorized(CategoryValidation, "ValidationError", "validation error: %s")
	// BusinessError is a general categorized business logic error.
	BusinessError = Categorized(CategoryBusiness, "BusinessError", "business error: %s")
	// SystemError is a general categorized system error.
	SystemError = Categorized(CategorySystem, "SystemError", "system error: %s")
)
