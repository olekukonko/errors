package errors

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Drain

func TestDrainAllNil(t *testing.T) {
	ch := make(chan error, 3)
	ch <- nil
	ch <- nil
	ch <- nil
	close(ch)
	if err := Drain(ch); err != nil {
		t.Errorf("Drain(all nil) = %v, want nil", err)
	}
}

func TestDrainCollectsErrors(t *testing.T) {
	ch := make(chan error, 3)
	ch <- New("one")
	ch <- nil
	ch <- New("two")
	close(ch)

	err := Drain(ch)
	if err == nil {
		t.Fatal("Drain() = nil, want errors")
	}
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("Drain() type = %T, want *MultiError", err)
	}
	if multi.Count() != 2 {
		t.Errorf("Drain() count = %d, want 2", multi.Count())
	}
}

func TestDrainEmpty(t *testing.T) {
	ch := make(chan error)
	close(ch)
	if err := Drain(ch); err != nil {
		t.Errorf("Drain(empty) = %v, want nil", err)
	}
}

func TestDrainSingleError(t *testing.T) {
	ch := make(chan error, 1)
	ch <- New("only error")
	close(ch)
	err := Drain(ch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "only error") {
		t.Errorf("unexpected message: %q", err.Error())
	}
}

// First

func TestFirstReturnsFirstError(t *testing.T) {
	ch := make(chan error, 3)
	ch <- nil
	ch <- New("first real error")
	ch <- New("second error")
	close(ch)

	err := First(context.Background(), ch)
	if err == nil {
		t.Fatal("First() = nil, want error")
	}
	if !strings.Contains(err.Error(), "first real error") {
		t.Errorf("First() = %q, want 'first real error'", err.Error())
	}
}

func TestFirstChannelClosedNoError(t *testing.T) {
	ch := make(chan error, 2)
	ch <- nil
	ch <- nil
	close(ch)
	if err := First(context.Background(), ch); err != nil {
		t.Errorf("First(no errors) = %v, want nil", err)
	}
}

func TestFirstContextCancelledReturnsCtxErr(t *testing.T) {
	ch := make(chan error) // never sends
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := First(ctx, ch)
	if err != context.Canceled {
		t.Errorf("First(cancelled) = %v, want context.Canceled", err)
	}
}

func TestFirstCallerOwnsCancel(t *testing.T) {
	// Verify the documented pattern: First returns, caller cancels siblings.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan error, 2)
	ch <- New("fail")
	ch <- New("also fail")
	close(ch)

	err := First(ctx, ch)
	if err == nil {
		t.Fatal("expected error")
	}
	// Caller now calls cancel() — this is the correct usage
	cancel()
	// ctx should now be done
	select {
	case <-ctx.Done():
	default:
		t.Error("ctx should be cancelled after caller calls cancel()")
	}
}

func TestFirstContextDeadline(t *testing.T) {
	ch := make(chan error) // never sends
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := First(ctx, ch)
	if err != context.DeadlineExceeded {
		t.Errorf("First(deadline) = %v, want DeadlineExceeded", err)
	}
}

// Collect

func TestCollectUpToN(t *testing.T) {
	ch := make(chan error, 10)
	for i := 0; i < 10; i++ {
		ch <- fmt.Errorf("error %d", i)
	}
	close(ch)

	err := Collect(context.Background(), ch, 3)
	if err == nil {
		t.Fatal("Collect() = nil, want errors")
	}
	if !Is(err, ErrLimitReached) {
		t.Errorf("Collect(limit) should wrap ErrLimitReached, got: %v", err)
	}
}

func TestCollectFewerThanN(t *testing.T) {
	ch := make(chan error, 3)
	ch <- New("a")
	ch <- New("b")
	close(ch)

	err := Collect(context.Background(), ch, 10)
	if err == nil {
		t.Fatal("expected errors")
	}
	// Did not hit limit — should NOT wrap ErrLimitReached
	if Is(err, ErrLimitReached) {
		t.Error("Collect(under limit) should not wrap ErrLimitReached")
	}
}

func TestCollectAllNil(t *testing.T) {
	ch := make(chan error, 3)
	ch <- nil
	ch <- nil
	close(ch)
	if err := Collect(context.Background(), ch, 5); err != nil {
		t.Errorf("Collect(all nil) = %v, want nil", err)
	}
}

func TestCollectContextDone(t *testing.T) {
	ch := make(chan error) // blocks forever
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Collect(ctx, ch, 10)
	if err != nil {
		t.Errorf("Collect(cancelled) = %v, want nil", err)
	}
}

// Fan

func TestFanMergesChannels(t *testing.T) {
	ch1 := make(chan error, 2)
	ch2 := make(chan error, 2)
	ch1 <- New("ch1-a")
	ch1 <- New("ch1-b")
	close(ch1)
	ch2 <- New("ch2-a")
	close(ch2)

	var collected []error
	for err := range Fan(context.Background(), ch1, ch2) {
		if err != nil {
			collected = append(collected, err)
		}
	}
	if len(collected) != 3 {
		t.Errorf("Fan() collected %d errors, want 3", len(collected))
	}
}

func TestFanEmpty(t *testing.T) {
	ch1 := make(chan error)
	ch2 := make(chan error)
	close(ch1)
	close(ch2)

	var count int
	for range Fan(context.Background(), ch1, ch2) {
		count++
	}
	if count != 0 {
		t.Errorf("Fan(empty inputs) received %d items, want 0", count)
	}
}

func TestFanNoInputs(t *testing.T) {
	merged := Fan(context.Background())
	select {
	case _, ok := <-merged:
		if ok {
			t.Error("Fan() with no inputs sent a value before closing")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Fan() with no inputs did not close promptly")
	}
}

func TestFanContextCancellation(t *testing.T) {
	ch := make(chan error) // never closes
	ctx, cancel := context.WithCancel(context.Background())

	merged := Fan(ctx, ch)
	cancel()

	select {
	case <-merged:
		// closed or received — either is fine
	case <-time.After(200 * time.Millisecond):
		t.Error("Fan() did not close after ctx cancellation")
	}
}

// Stream

func TestStreamAllSucceed(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	s := NewStream(context.Background(), items, func(n int) error { return nil })
	if err := s.Wait(); err != nil {
		t.Errorf("Stream(all succeed) = %v, want nil", err)
	}
}

func TestStreamCollectsAllErrors(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	s := NewStream(context.Background(), items, func(n int) error {
		if n%2 == 0 {
			return fmt.Errorf("error %d", n)
		}
		return nil
	})
	err := s.Wait()
	if err == nil {
		t.Fatal("Stream() = nil, want errors")
	}
	multi, ok := err.(*MultiError)
	if !ok {
		t.Fatalf("Stream() type = %T, want *MultiError", err)
	}
	if multi.Count() != 2 {
		t.Errorf("Stream() count = %d, want 2", multi.Count())
	}
}

func TestStreamEach(t *testing.T) {
	items := []string{"a", "b", "c"}
	s := NewStream(context.Background(), items, func(item string) error {
		if item == "b" {
			return New("b failed")
		}
		return nil
	})

	var count int
	s.Each(func(err error) { count++ })
	if count != 1 {
		t.Errorf("Stream.Each() called fn %d times, want 1", count)
	}
}

func TestStreamDoubleConsumePanics(t *testing.T) {
	s := NewStream(context.Background(), []int{1}, func(n int) error { return nil })
	_ = s.Wait()

	defer func() {
		if r := recover(); r == nil {
			t.Error("second Wait() should panic")
		}
	}()
	_ = s.Wait()
}

func TestStreamEachThenWaitPanics(t *testing.T) {
	items := []int{1, 2}
	s := NewStream(context.Background(), items, func(n int) error { return nil })
	s.Each(func(err error) {})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Wait() after Each() should panic")
		}
	}()
	_ = s.Wait()
}

func TestStreamWorkerConcurrency(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	items := make([]int, 20)
	s := NewStream(context.Background(), items, func(n int) error {
		c := concurrent.Add(1)
		for {
			cur := maxConcurrent.Load()
			if c <= cur || maxConcurrent.CompareAndSwap(cur, c) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		concurrent.Add(-1)
		return nil
	}, 4)

	_ = s.Wait()
	if maxConcurrent.Load() > 4 {
		t.Errorf("concurrency exceeded workers: max=%d, want<=4", maxConcurrent.Load())
	}
}

func TestStreamContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	items := make([]int, 100)
	var processed atomic.Int32

	s := NewStream(ctx, items, func(n int) error {
		processed.Add(1)
		if processed.Load() == 5 {
			cancel()
		}
		time.Sleep(time.Millisecond)
		return nil
	}, 2)

	_ = s.Wait()
	if processed.Load() == 100 {
		t.Error("Stream did not respect context cancellation")
	}
}

func TestStreamStop(t *testing.T) {
	items := make([]int, 100)
	var processed atomic.Int32

	s := NewStream(context.Background(), items, func(n int) error {
		processed.Add(1)
		time.Sleep(time.Millisecond)
		return nil
	}, 2)

	time.Sleep(10 * time.Millisecond)
	s.Stop()

	// After Stop, goroutines should not leak — channel is drained by Stop.
	// Give it time to settle.
	time.Sleep(20 * time.Millisecond)
	if processed.Load() == 100 {
		t.Error("Stream.Stop() did not stop processing early")
	}
}

func TestStreamEmpty(t *testing.T) {
	s := NewStream(context.Background(), []string{}, func(s string) error {
		return New("should not be called")
	})
	if err := s.Wait(); err != nil {
		t.Errorf("Stream(empty items) = %v, want nil", err)
	}
}

func TestStreamDefaultWorkers(t *testing.T) {
	s := NewStream(context.Background(), []int{1, 2, 3}, func(n int) error { return nil })
	if err := s.Wait(); err != nil {
		t.Errorf("Stream(default workers) = %v, want nil", err)
	}
}
