package errors

// Common HTTP error codes
const (
	CodeBadRequest    = 400
	CodeUnauthorized  = 401
	CodeForbidden     = 403
	CodeNotFound      = 404
	CodeInternalError = 500
)

// Predefined errors
var (
	ErrInvalidArg = New("invalid argument")
	ErrNotFound   = New("not found")
	ErrPermission = New("permission denied")
	ErrTimeout    = New("operation timed out")
	ErrUnknown    = New("unknown error")
)

// Predefined errors with codes
var (
	ErrAuth = Coded("ErrAuth", CodeUnauthorized, "authentication failed for %s: %s")
)

// Predefined templates
var (
	ErrDatabase       = Define("ErrDatabase", "database error: %s")
	ErrNetwork        = Define("ErrNetwork", "network failure: %s")
	ErrConfig         = Define("ErrConfig", "invalid configuration: %s")
	ErrValidation     = Define("ErrValidation", "validation error: %s")
	ErrResource       = Define("ErrResource", "resource exhausted: %s")
	ErrConflict       = Define("ErrConflict", "conflict detected: %s")
	ErrNotImplemented = Define("ErrNotImplemented", "feature not implemented: %s")
	ErrArity          = Define("ErrArity", "wrong number of arguments for '%s'")
	ErrExpiry         = Define("ErrExpiry", "invalid expire time in '%s'")
	ErrParse          = Define("ErrParse", "parse error: %s")
	ErrIO             = Define("ErrIO", "I/O error: %s")
	ErrOverflow       = Define("ErrOverflow", "overflow error: %s")
	ErrUnderflow      = Define("ErrUnderflow", "underflow error: %s")
	ErrRace           = Define("ErrRace", "race condition detected: %s")
)
