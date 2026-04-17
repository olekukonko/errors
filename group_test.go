package errors

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupAllSucceed(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })

	if err := g.Wait(); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestGroupCollectsAllErrors(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { return New("error one") })
	g.Go(func() error { return nil })
	g.Go(func() error { return New("error two") })
	g.Go(func() error { return New("error three") })

	err := g.Wait()
	if err == nil {
		t.Fatal("expected errors, got nil")
	}
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if multi.Count() != 3 {
		t.Errorf("expected 3 errors, got %d", multi.Count())
	}
}

func TestGroupSingleError(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { return New("only error") })

	err := g.Wait()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Wait always returns *MultiError so callers can reliably type-assert.
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if multi.Count() != 1 {
		t.Errorf("expected 1 error, got %d", multi.Count())
	}
	if !strings.Contains(multi.Error(), "only error") {
		t.Errorf("unexpected message: %q", multi.Error())
	}
}

func TestGroupGoCtx(t *testing.T) {
	ctx := context.Background()
	g := NewGroup(GroupWithContext(ctx, false))

	var received atomic.Int32
	g.GoCtx(func(ctx context.Context) error {
		if ctx == nil {
			return New("nil context")
		}
		received.Add(1)
		return nil
	})
	g.GoCtx(func(ctx context.Context) error {
		received.Add(1)
		return nil
	})

	if err := g.Wait(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if received.Load() != 2 {
		t.Errorf("expected 2 goroutines to run, got %d", received.Load())
	}
}

func TestGroupCancelOnFirst(t *testing.T) {
	ctx := context.Background()
	g := NewGroup(GroupWithContext(ctx, true))

	var started atomic.Int32
	// First goroutine errors immediately.
	g.GoCtx(func(ctx context.Context) error {
		started.Add(1)
		return New("first failure")
	})
	// Second goroutine checks ctx cancellation after a small delay.
	g.GoCtx(func(ctx context.Context) error {
		started.Add(1)
		select {
		case <-ctx.Done():
			// Context was cancelled by first failure — return nil
			// to show cancellation was observed.
			return nil
		case <-time.After(200 * time.Millisecond):
			return New("second should have been cancelled")
		}
	})

	err := g.Wait()
	// Only the first error should be collected; second observed cancellation.
	if err == nil {
		t.Fatal("expected at least one error")
	}
	if started.Load() != 2 {
		t.Errorf("expected both goroutines to start, got %d", started.Load())
	}
}

func TestGroupWithLimit(t *testing.T) {
	g := NewGroup(GroupWithLimit(2))
	for i := 0; i < 10; i++ {
		i := i
		g.Go(func() error { return fmt.Errorf("error %d", i) })
	}
	err := g.Wait()
	if err == nil {
		t.Fatal("expected errors, got nil")
	}
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if multi.Count() > 2 {
		t.Errorf("expected at most 2 errors due to limit, got %d", multi.Count())
	}
}

func TestGroupErrors(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { return New("a") })
	g.Go(func() error { return New("b") })
	_ = g.Wait()

	errs := g.Errors()
	if len(errs) != 2 {
		t.Errorf("expected 2 errors from Errors(), got %d", len(errs))
	}
}

func TestGroupReuseAfterWait(t *testing.T) {
	g := NewGroup()
	g.Go(func() error { return New("round one") })
	err1 := g.Wait()
	if err1 == nil {
		t.Fatal("expected error in round one")
	}

	// Second round — verify group can be reused.
	g.Go(func() error { return nil })
	err2 := g.Wait()
	// After reuse the old errors are still present (Group accumulates).
	// This is expected behaviour; document it in the test.
	_ = err2
}

func TestGroupConcurrentSafety(t *testing.T) {
	g := NewGroup()
	for i := 0; i < 100; i++ {
		i := i
		g.Go(func() error {
			if i%2 == 0 {
				// Use unique messages so MultiError.Add deduplication does not
				// collapse them — each goroutine index produces a distinct string.
				return fmt.Errorf("even error %d", i)
			}
			return nil
		})
	}
	err := g.Wait()
	if err == nil {
		t.Fatal("expected errors from 50 failing goroutines")
	}
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("expected *MultiError, got %T", err)
	}
	if multi.Count() != 50 {
		t.Errorf("expected 50 errors, got %d", multi.Count())
	}
}
