package errors

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPStatusCodeNilError(t *testing.T) {
	if got := HTTPStatusCode(nil, 500); got != 500 {
		t.Errorf("nil error: got %d, want 500", got)
	}
}

func TestHTTPStatusCodeWithValidCode(t *testing.T) {
	err := New("not found").WithCode(404)
	if got := HTTPStatusCode(err, 500); got != 404 {
		t.Errorf("got %d, want 404", got)
	}
}

func TestHTTPStatusCodeOutOfRange(t *testing.T) {
	// Code below 100 is not a valid HTTP status — should use fallback.
	err := New("bad").WithCode(50)
	if got := HTTPStatusCode(err, 500); got != 500 {
		t.Errorf("out-of-range code: got %d, want 500", got)
	}
	// Code above 599 also invalid.
	err2 := New("bad").WithCode(600)
	if got := HTTPStatusCode(err2, 503); got != 503 {
		t.Errorf("code 600: got %d, want 503", got)
	}
}

func TestHTTPStatusCodeNonErrorType(t *testing.T) {
	// stdlib error has no code — should use fallback.
	err := fmt.Errorf("plain error")
	if got := HTTPStatusCode(err, 502); got != 502 {
		t.Errorf("plain error: got %d, want 502", got)
	}
}

func TestHTTPErrorDefaultBehaviour(t *testing.T) {
	w := httptest.NewRecorder()
	err := New("something broke").WithCode(422)
	HTTPError(w, err)

	if w.Code != 422 {
		t.Errorf("status: got %d, want 422", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type: got %q, want text/plain prefix", ct)
	}
	if !strings.Contains(w.Body.String(), "something broke") {
		t.Errorf("body missing error message: %q", w.Body.String())
	}
}

func TestHTTPErrorFallbackCode(t *testing.T) {
	w := httptest.NewRecorder()
	err := New("upstream unavailable") // no code set
	HTTPError(w, err, WithFallbackCode(http.StatusBadGateway))

	if w.Code != http.StatusBadGateway {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestHTTPErrorNoBody(t *testing.T) {
	w := httptest.NewRecorder()
	err := New("internal error").WithCode(500)
	HTTPError(w, err, WithBody(false))

	if w.Code != 500 {
		t.Errorf("status: got %d, want 500", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("body should be empty, got %q", w.Body.String())
	}
}

func TestHTTPErrorCustomBodyFunc(t *testing.T) {
	w := httptest.NewRecorder()
	err := New("bad input").WithCode(400)
	HTTPError(w, err, WithBodyFunc(func(e error) string {
		return fmt.Sprintf(`{"error":%q}`, e.Error())
	}))

	if w.Code != 400 {
		t.Errorf("status: got %d, want 400", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"bad input"`) {
		t.Errorf("JSON body missing error: %q", body)
	}
}

func TestHTTPErrorNilError(t *testing.T) {
	w := httptest.NewRecorder()
	HTTPError(w, nil, WithFallbackCode(200))

	if w.Code != 200 {
		t.Errorf("nil error: got %d, want 200", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("nil error body should be empty, got %q", w.Body.String())
	}
}

func TestHTTPErrorSentinelError(t *testing.T) {
	ErrForbidden := Const("forbidden", "access denied")
	w := httptest.NewRecorder()
	// Sentinel has no code — uses fallback.
	HTTPError(w, ErrForbidden, WithFallbackCode(http.StatusForbidden))

	if w.Code != http.StatusForbidden {
		t.Errorf("sentinel: got %d, want %d", w.Code, http.StatusForbidden)
	}
}
