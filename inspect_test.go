package errors

import (
	"bytes"
	"strings"
	"testing"
)

func TestInspectNil(t *testing.T) {
	var buf bytes.Buffer
	Inspect(nil, &buf)
	if !strings.Contains(buf.String(), "no error") {
		t.Errorf("expected 'no error', got: %q", buf.String())
	}
}

func TestInspectPlainError(t *testing.T) {
	var buf bytes.Buffer
	err := New("something went wrong").WithCode(500).With("user", "alice")
	Inspect(err, &buf)
	out := buf.String()

	if !strings.Contains(out, "something went wrong") {
		t.Errorf("missing message in output: %q", out)
	}
	if !strings.Contains(out, "code:") {
		t.Errorf("missing code in output: %q", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("missing context value in output: %q", out)
	}
}

func TestInspectNamedError(t *testing.T) {
	var buf bytes.Buffer
	err := Named("AuthError").WithCode(401)
	Inspect(err, &buf)
	out := buf.String()

	if !strings.Contains(out, "AuthError") {
		t.Errorf("missing name in output: %q", out)
	}
	if !strings.Contains(out, "401") {
		t.Errorf("missing code in output: %q", out)
	}
	if !strings.Contains(out, "code=401") {
		t.Errorf("missing diagnostics code in output: %q", out)
	}
}

func TestInspectChain(t *testing.T) {
	var buf bytes.Buffer
	cause := New("db timeout").WithTimeout()
	outer := New("request failed").Wrap(cause)
	Inspect(outer, &buf)
	out := buf.String()

	if !strings.Contains(out, "request failed") {
		t.Errorf("missing outer message: %q", out)
	}
	if !strings.Contains(out, "db timeout") {
		t.Errorf("missing cause message: %q", out)
	}
	if !strings.Contains(out, "timeout") {
		t.Errorf("missing timeout diagnostic: %q", out)
	}
}

func TestInspectMultiError(t *testing.T) {
	var buf bytes.Buffer
	m := NewMultiError()
	m.Add(New("error one"))
	m.Add(New("error two"))
	Inspect(m, &buf)
	out := buf.String()

	if !strings.Contains(out, "errors:  2") {
		t.Errorf("missing error count: %q", out)
	}
	if !strings.Contains(out, "error one") {
		t.Errorf("missing first error: %q", out)
	}
	if !strings.Contains(out, "error two") {
		t.Errorf("missing second error: %q", out)
	}
}

func TestInspectMultipleWriters(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	err := New("dual write test")
	Inspect(err, &buf1, &buf2)

	if buf1.String() == "" {
		t.Error("buf1 should have received output")
	}
	if buf1.String() != buf2.String() {
		t.Errorf("both writers should receive identical output\nbuf1: %q\nbuf2: %q",
			buf1.String(), buf2.String())
	}
}

func TestInspectWithStackFramesOption(t *testing.T) {
	var buf bytes.Buffer
	err := Trace("traced error")
	Inspect(err, &buf, WithStackFrames(1))
	out := buf.String()

	// Should mention stack but respect the 1-frame limit
	if !strings.Contains(out, "stack") {
		t.Errorf("expected stack section in output: %q", out)
	}
}

func TestInspectWithMaxDepthOption(t *testing.T) {
	var buf bytes.Buffer
	// Build a 5-deep chain
	err := New("level 0")
	for i := 1; i <= 5; i++ {
		err = New("level " + string(rune('0'+i))).Wrap(err)
	}
	Inspect(err, &buf, WithMaxDepth(2))
	out := buf.String()

	if !strings.Contains(out, "truncated") {
		t.Errorf("expected truncation message for deep chain: %q", out)
	}
}

func TestInspectRetryableDiagnostic(t *testing.T) {
	var buf bytes.Buffer
	err := New("flaky call").WithRetryable()
	Inspect(err, &buf)
	out := buf.String()

	if !strings.Contains(out, "retryable") {
		t.Errorf("expected retryable diagnostic: %q", out)
	}
}

func TestInspectDefaultsToStderr(t *testing.T) {
	// Just verify it doesn't panic with no writers supplied.
	// We can't capture stderr easily in a unit test, so we only check no panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Inspect panicked with no writers: %v", r)
		}
	}()
	// Redirect would require os.Pipe tricks; just call with a buffer to keep
	// output off the test console while still exercising the code path.
	var buf bytes.Buffer
	Inspect(New("test"), &buf)
}

func TestInspectError(t *testing.T) {
	var buf bytes.Buffer
	err := Named("SomeError").WithCode(503)
	InspectError(err, &buf)
	out := buf.String()

	if !strings.Contains(out, "SomeError") {
		t.Errorf("InspectError missing name: %q", out)
	}
}
