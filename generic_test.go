package errors

import (
	"errors"
	"testing"
)

// baseErr is a local type alias for *Error used solely to give the anonymous
// embedded field a name that is not "Error". This is the only way to embed
// *Error and still satisfy the error interface:
//
//   - Anonymous embed of *Error       -> field name "Error" -> shadows Error() method

//   - Named field   E *Error          -> not an embed -> methods not promoted

//   - Anonymous embed of *baseErr     -> field name "baseErr" -> no shadowing
//     where baseErr = Error (alias)   -> Error() and Wrap() promoted correctly

type baseErr = Error // type alias: same type, different identifier at embed site

// testError is a custom error type for testing generics.
type testError struct {
	*baseErr // anonymous embed; field name is "baseErr", not "Error"
	code     int
}

func newTestError(code int) *testError {
	return &testError{
		baseErr: New("test error").WithCode(code),
		code:    code,
	}
}

// Wrap sets cause on the embedded *Error and returns *testError so callers can
// chain further calls on *testError without losing the concrete type.
// Wrap sets cause as the cause of this error and returns e so callers keep the
// concrete *testError type across chains. It mutates e in place (same semantics
// as *Error.Wrap) — each call replaces the cause, so
//
//	newTestError(400).Wrap(A).Wrap(B)
//
// produces: testError(400, cause=B). That matches *Error.Wrap behaviour.
// For a chain of three testErrors, callers must nest: A.Wrap(B.Wrap(C)) or the
// test uses the chained form which is fine because each .Wrap() call is on the
// result of the previous, not the same receiver.
//
// HOWEVER: newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600))
// chains on the same receiver *testError(400) twice — clobbering 500 with 600.
// The tests expect all three to appear in the chain, so Wrap must build a NEW
// wrapper each time, not mutate in place.
// Wrap appends cause to the tail of this error's chain and returns e.
// This makes A.Wrap(B).Wrap(C) produce the chain A→B→C (not A→C),
// which is what the tests expect when reading back all codes in order.
func (e *testError) Wrap(cause error) *testError {
	// Walk to the innermost *Error in this node's chain and attach cause there.
	tail := e.baseErr
	for tail.cause != nil {
		if next, ok := tail.cause.(*baseErr); ok {
			tail = next
		} else if next, ok := tail.cause.(*testError); ok {
			tail = next.baseErr
		} else if next, ok := tail.cause.(*specialError); ok {
			tail = next.baseErr
		} else {
			break
		}
	}
	tail.cause = cause
	return e
}

// Unwrap delegates to the embedded *Error so Walk/errors.As can traverse the chain.
func (e *testError) Unwrap() error { return e.baseErr.Unwrap() }

// specialError is another custom error type for testing generics.
type specialError struct {
	*baseErr // anonymous embed; field name is "baseErr", not "Error"
	priority int
}

func newSpecialError(priority int) *specialError {
	return &specialError{
		baseErr:  Named("SpecialError"),
		priority: priority,
	}
}

// Wrap appends cause to the tail of this error's chain and returns e.
func (e *specialError) Wrap(cause error) *specialError {
	tail := e.baseErr
	for tail.cause != nil {
		if next, ok := tail.cause.(*baseErr); ok {
			tail = next
		} else if next, ok := tail.cause.(*testError); ok {
			tail = next.baseErr
		} else if next, ok := tail.cause.(*specialError); ok {
			tail = next.baseErr
		} else {
			break
		}
	}
	tail.cause = cause
	return e
}

// Unwrap delegates to the embedded *Error so Walk/errors.As can traverse the chain.
func (e *specialError) Unwrap() error { return e.baseErr.Unwrap() }

// simpleError is a non-*Error type that implements error
type simpleError struct {
	msg string
}

func (e simpleError) Error() string {
	return e.msg
}

func TestAsType(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantFound bool
		wantCode  int
	}{
		{
			name:      "found testError",
			err:       newTestError(404),
			wantFound: true,
			wantCode:  404,
		},
		{
			name:      "found wrapped testError",
			err:       newTestError(500).Wrap(newTestError(400)),
			wantFound: true,
			wantCode:  500,
		},
		{
			name:      "not found wrong type",
			err:       newSpecialError(1),
			wantFound: false,
		},
		{
			name:      "nil error",
			err:       nil,
			wantFound: false,
		},
		{
			name:      "standard error",
			err:       errors.New("standard"),
			wantFound: false,
		},
		{
			name:      "simpleError type",
			err:       simpleError{msg: "simple"},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := AsType[*testError](tt.err)
			if found != tt.wantFound {
				t.Errorf("AsType() found = %v, want %v", found, tt.wantFound)
			}
			if found && got.code != tt.wantCode {
				t.Errorf("AsType() code = %v, want %v", got.code, tt.wantCode)
			}
		})
	}
}

func TestIsType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct match",
			err:  newTestError(404),
			want: true,
		},
		{
			name: "wrapped match",
			err:  newSpecialError(1).Wrap(newTestError(500)),
			want: true,
		},
		{
			name: "deep wrapped match",
			err:  New("top").Wrap(newSpecialError(2).Wrap(newTestError(600))),
			want: true,
		},
		{
			name: "no match",
			err:  newSpecialError(1),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "standard error",
			err:  errors.New("standard"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsType[*testError](tt.err); got != tt.want {
				t.Errorf("IsType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindType(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		predicate func(*testError) bool
		wantFound bool
		wantCode  int
	}{
		{
			name: "match by code",
			err:  newTestError(400).Wrap(newTestError(500)),
			predicate: func(e *testError) bool {
				return e.code == 500
			},
			wantFound: true,
			wantCode:  500,
		},
		{
			name: "match first",
			err:  newTestError(400).Wrap(newTestError(500)),
			predicate: func(e *testError) bool {
				return e.code == 400
			},
			wantFound: true,
			wantCode:  400,
		},
		{
			name: "no match",
			err:  newTestError(400),
			predicate: func(e *testError) bool {
				return e.code == 404
			},
			wantFound: false,
		},
		{
			name:      "nil predicate",
			err:       newTestError(400),
			predicate: nil,
			wantFound: false,
		},
		{
			name: "wrong type",
			err:  newSpecialError(1),
			predicate: func(e *testError) bool {
				return true
			},
			wantFound: false,
		},
		{
			name: "deep chain match",
			err:  New("top").Wrap(newSpecialError(2)).Wrap(newTestError(404).Wrap(newTestError(500))),
			predicate: func(e *testError) bool {
				return e.code == 404
			},
			wantFound: true,
			wantCode:  404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FindType(tt.err, tt.predicate)
			if found != tt.wantFound {
				t.Errorf("FindType() found = %v, want %v", found, tt.wantFound)
			}
			if found && got.code != tt.wantCode {
				t.Errorf("FindType() code = %v, want %v", got.code, tt.wantCode)
			}
		})
	}
}

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		fn       func(*testError) int
		expected []int
	}{
		{
			name:     "single error",
			err:      newTestError(400),
			fn:       func(e *testError) int { return e.code },
			expected: []int{400},
		},
		{
			name:     "multiple errors in chain",
			err:      newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600)),
			fn:       func(e *testError) int { return e.code },
			expected: []int{400, 500, 600},
		},
		{
			name:     "mixed error types",
			err:      newTestError(400).Wrap(newSpecialError(1)).Wrap(newTestError(500)),
			fn:       func(e *testError) int { return e.code },
			expected: []int{400, 500},
		},
		{
			name:     "nil error",
			err:      nil,
			fn:       func(e *testError) int { return e.code },
			expected: []int{},
		},
		{
			name:     "no matching type",
			err:      newSpecialError(1),
			fn:       func(e *testError) int { return e.code },
			expected: []int{},
		},
		{
			name:     "nil function",
			err:      newTestError(400),
			fn:       nil,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.err, tt.fn)
			if len(got) != len(tt.expected) {
				t.Errorf("Map() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Map()[%d] = %d, want %d", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestReduce(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		initial  int
		fn       func(*testError, int) int
		expected int
	}{
		{
			name:    "sum codes",
			err:     newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600)),
			initial: 0,
			fn: func(e *testError, acc int) int {
				return acc + e.code
			},
			expected: 1500,
		},
		{
			name:    "max code",
			err:     newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(300)),
			initial: 0,
			fn: func(e *testError, acc int) int {
				if e.code > acc {
					return e.code
				}
				return acc
			},
			expected: 500,
		},
		{
			name:    "nil error",
			err:     nil,
			initial: 42,
			fn: func(e *testError, acc int) int {
				return acc + e.code
			},
			expected: 42,
		},
		{
			name:    "no matching type",
			err:     newSpecialError(1),
			initial: 10,
			fn: func(e *testError, acc int) int {
				return acc + e.code
			},
			expected: 10,
		},
		{
			name:     "nil function",
			err:      newTestError(400),
			initial:  5,
			fn:       nil,
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Reduce(tt.err, tt.initial, tt.fn)
			if got != tt.expected {
				t.Errorf("Reduce() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int // count of testError
	}{
		{
			name:     "single error",
			err:      newTestError(400),
			expected: 1,
		},
		{
			name:     "multiple test errors",
			err:      newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600)),
			expected: 3,
		},
		{
			name:     "mixed with other types",
			err:      newTestError(400).Wrap(newSpecialError(1)).Wrap(newTestError(500)),
			expected: 2,
		},
		{
			name:     "no test errors",
			err:      newSpecialError(1),
			expected: 0,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: 0,
		},
		{
			name:     "standard error",
			err:      errors.New("standard"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter[*testError](tt.err)
			if len(got) != tt.expected {
				t.Errorf("Filter() length = %d, want %d", len(got), tt.expected)
			}
		})
	}
}

func TestFirstOfType(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantFound bool
		wantCode  int
	}{
		{
			name:      "first is testError",
			err:       newTestError(400).Wrap(newSpecialError(1)),
			wantFound: true,
			wantCode:  400,
		},
		{
			name:      "testError after other type",
			err:       newSpecialError(1).Wrap(newTestError(500)),
			wantFound: true,
			wantCode:  500,
		},
		{
			name:      "multiple test errors",
			err:       newTestError(400).Wrap(newTestError(500)),
			wantFound: true,
			wantCode:  400,
		},
		{
			name:      "no test error",
			err:       newSpecialError(1),
			wantFound: false,
		},
		{
			name:      "nil error",
			err:       nil,
			wantFound: false,
		},
		{
			name:      "deep chain",
			err:       New("top").Wrap(newSpecialError(2)).Wrap(newTestError(404)),
			wantFound: true,
			wantCode:  404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := FirstOfType[*testError](tt.err)
			if found != tt.wantFound {
				t.Errorf("FirstOfType() found = %v, want %v", found, tt.wantFound)
			}
			if found && got.code != tt.wantCode {
				t.Errorf("FirstOfType() code = %v, want %v", got.code, tt.wantCode)
			}
		})
	}
}

func TestContains(t *testing.T) {
	targetErr := New("target error")
	otherErr := New("other error")
	stdErr := errors.New("std error")

	tests := []struct {
		name    string
		err     error
		targets []error
		want    bool
	}{
		{
			name:    "contains target",
			err:     targetErr,
			targets: []error{targetErr},
			want:    true,
		},
		{
			name:    "contains in wrapped chain",
			err:     newTestError(400).Wrap(targetErr),
			targets: []error{targetErr},
			want:    true,
		},
		{
			name:    "does not contain",
			err:     otherErr,
			targets: []error{targetErr},
			want:    false,
		},
		{
			name:    "multiple targets - second matches",
			err:     targetErr,
			targets: []error{otherErr, targetErr},
			want:    true,
		},
		{
			name:    "nil error",
			err:     nil,
			targets: []error{targetErr},
			want:    false,
		},
		{
			name:    "empty targets",
			err:     targetErr,
			targets: []error{},
			want:    false,
		},
		{
			name:    "standard error target",
			err:     New("wrapper").Wrap(stdErr),
			targets: []error{stdErr},
			want:    true,
		},
		{
			name:    "named error match",
			err:     Named("TestError"),
			targets: []error{Named("TestError")},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.err, tt.targets...); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinErrors(t *testing.T) {
	err1 := New("error 1")
	err2 := New("error 2")
	err3 := New("error 3")

	tests := []struct {
		name      string
		errs      []error
		keyValues []interface{}
		wantNil   bool
		wantCount int // expected number of errors in the result (0 if nil)
	}{
		{
			name:      "join two errors",
			errs:      []error{err1, err2},
			wantNil:   false,
			wantCount: 2,
		},
		{
			name:      "join single error",
			errs:      []error{err1},
			wantNil:   false,
			wantCount: 1,
		},
		{
			name:      "all nil errors",
			errs:      []error{nil, nil},
			wantNil:   true,
			wantCount: 0,
		},
		{
			name:      "empty slice",
			errs:      []error{},
			wantNil:   true,
			wantCount: 0,
		},
		{
			name:      "join with context",
			errs:      []error{err1, err2},
			keyValues: []interface{}{"key", "value", "operation", "test"},
			wantNil:   false,
			wantCount: 2,
		},
		{
			name:      "single error with context",
			errs:      []error{err1},
			keyValues: []interface{}{"user", "123"},
			wantNil:   false,
			wantCount: 1,
		},
		{
			name:      "nil and non-nil mix",
			errs:      []error{nil, err1, nil, err2, err3},
			wantNil:   false,
			wantCount: 3,
		},
		{
			name:      "mix of standard and custom errors",
			errs:      []error{err1, errors.New("std error"), err2},
			wantNil:   false,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinErrors(tt.errs, tt.keyValues...)

			if (got == nil) != tt.wantNil {
				t.Errorf("JoinErrors() returned nil = %v, want %v", got == nil, tt.wantNil)
			}

			if !tt.wantNil {
				// Check if it's a MultiError with correct count
				if multi, ok := got.(*MultiError); ok {
					if multi.Count() != tt.wantCount {
						t.Errorf("JoinErrors() MultiError count = %d, want %d", multi.Count(), tt.wantCount)
					}
				} else if tt.wantCount > 1 {
					t.Errorf("JoinErrors() should return *MultiError for multiple errors, got %T", got)
				}

				// Verify context was added if keyValues provided
				if len(tt.keyValues) > 0 {
					// For multiple errors, the wrapper *Error wraps the MultiError
					if e, ok := got.(*Error); ok {
						ctx := e.Context()
						if len(ctx) == 0 {
							t.Error("JoinErrors() with context should have context values")
						}
					}
				}
			}
		})
	}
}

// Benchmark tests for generics
func BenchmarkAsType(b *testing.B) {
	err := newTestError(404)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = AsType[*testError](err)
	}
}

func BenchmarkIsType(b *testing.B) {
	err := newTestError(404)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsType[*testError](err)
	}
}

func BenchmarkFindType(b *testing.B) {
	err := newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600))
	predicate := func(e *testError) bool { return e.code == 500 }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FindType(err, predicate)
	}
}

func BenchmarkMap(b *testing.B) {
	err := newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600))
	fn := func(e *testError) int { return e.code }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Map(err, fn)
	}
}

func BenchmarkFilter(b *testing.B) {
	err := newTestError(400).Wrap(newSpecialError(1)).Wrap(newTestError(500))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Filter[*testError](err)
	}
}

func BenchmarkReduce(b *testing.B) {
	err := newTestError(400).Wrap(newTestError(500)).Wrap(newTestError(600))
	fn := func(e *testError, acc int) int { return acc + e.code }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Reduce(err, 0, fn)
	}
}

func BenchmarkFirstOfType(b *testing.B) {
	err := newSpecialError(1).Wrap(newTestError(400)).Wrap(newTestError(500))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = FirstOfType[*testError](err)
	}
}

func BenchmarkJoinErrors(b *testing.B) {
	errs := []error{New("error 1"), New("error 2"), New("error 3")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = JoinErrors(errs)
	}
}
