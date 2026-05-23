package historyimport

import (
	"context"
	"testing"
	"time"
)

func TestHeartbeatLoop_TouchesRunUntilContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		heartbeatLoop(ctx, 5*time.Millisecond, func(context.Context) error {
			called <- struct{}{}
			cancel()
			return nil
		})
		close(done)
	}()

	select {
	case <-called:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("heartbeatLoop did not invoke touch before timeout")
	}

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("heartbeatLoop did not stop after context cancellation")
	}
}

func TestSweepStaleRunsOnce_UsesThresholdAndInterruptedMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 1, 16, 30, 0, 0, time.UTC)
	var (
		calls          int
		gotStaleBefore time.Time
		gotMessage     string
	)

	if err := sweepStaleRunsOnce(context.Background(), now, 2*time.Minute, func(_ context.Context, staleBefore time.Time, message string) (int64, error) {
		calls++
		gotStaleBefore = staleBefore
		gotMessage = message
		return 1, nil
	}); err != nil {
		t.Fatalf("sweepStaleRunsOnce returned error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	wantStaleBefore := now.Add(-2 * time.Minute)
	if !gotStaleBefore.Equal(wantStaleBefore) {
		t.Fatalf("staleBefore = %v, want %v", gotStaleBefore, wantStaleBefore)
	}
	if gotMessage != staleRunInterruptedMessage {
		t.Fatalf("message = %q, want %q", gotMessage, staleRunInterruptedMessage)
	}
}
