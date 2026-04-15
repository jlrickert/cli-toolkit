package clock_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seed is a fixed reference time used across scheduling tests.
var seed = time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)

// --- OsClock scheduling ---

// TestOsClock_NewTicker verifies that the production clock delivers at least
// one tick within a generous wall-clock window. Skipped in -short mode to keep
// CI fast.
func TestOsClock_NewTicker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-time ticker test in -short mode")
	}

	osc := &clock.OsClock{}
	ticker := osc.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	select {
	case got := <-ticker.C():
		assert.False(t, got.IsZero(), "tick time should not be zero")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no tick received within 200ms")
	}
}

// --- TestClock scheduling ---

// TestTestClock_NewTicker_Basic checks that Advance delivers ticks in order.
func TestTestClock_NewTicker_Basic(t *testing.T) {
	tc := clock.NewTestClock(seed)
	ticker := tc.NewTicker(time.Second)
	defer ticker.Stop()

	// Nothing should be on the channel before Advance.
	select {
	case <-ticker.C():
		t.Fatal("unexpected tick before Advance")
	default:
	}

	// Advance by exactly one period — should receive exactly one tick.
	tc.Advance(time.Second)
	select {
	case got := <-ticker.C():
		assert.Equal(t, seed.Add(time.Second), got)
	default:
		t.Fatal("expected a tick after advancing one period")
	}

	// Advance by another period.
	tc.Advance(time.Second)
	select {
	case got := <-ticker.C():
		assert.Equal(t, seed.Add(2*time.Second), got)
	default:
		t.Fatal("expected a second tick")
	}
}

// TestTestClock_NewTicker_MultiPeriods checks that a single large Advance
// fires the ticker for each elapsed period up to capacity-1 (subsequent ticks
// are dropped because the channel buffer is 1).
func TestTestClock_NewTicker_MultiPeriods(t *testing.T) {
	tc := clock.NewTestClock(seed)
	ticker := tc.NewTicker(time.Second)
	defer ticker.Stop()

	// Advance 5 seconds: 5 ticks scheduled, but channel buffer is 1, so at
	// most 1 is buffered; the rest are dropped.
	tc.Advance(5 * time.Second)

	count := 0
	for {
		select {
		case <-ticker.C():
			count++
		default:
			goto done
		}
	}
done:
	// We should see at least 1 tick (the first one) and no more than 1 (buffer
	// size is 1 and subsequent non-blocking sends dropped on full buffer).
	assert.Equal(t, 1, count, "only 1 tick should be buffered (channel buf=1, rest dropped)")
}

// TestTestClock_NewTicker_DropOnFullBuffer verifies non-blocking send: if the
// receiver doesn't drain the channel, ticks are dropped rather than blocking.
func TestTestClock_NewTicker_DropOnFullBuffer(t *testing.T) {
	tc := clock.NewTestClock(seed)
	ticker := tc.NewTicker(time.Second)
	defer ticker.Stop()

	// Advance 3 periods without reading — should not deadlock or panic.
	tc.Advance(time.Second)
	tc.Advance(time.Second)
	tc.Advance(time.Second)

	// Exactly one tick should be in the buffer.
	count := 0
	select {
	case <-ticker.C():
		count++
	default:
	}
	assert.Equal(t, 1, count)
}

// TestTestClock_AfterFunc_GoroutineSemantics checks that AfterFunc callbacks
// run in goroutines (not inline) and fire after the correct Advance.
func TestTestClock_AfterFunc_GoroutineSemantics(t *testing.T) {
	tc := clock.NewTestClock(seed)

	var fired atomic.Bool
	done := make(chan struct{})

	tc.AfterFunc(time.Second, func() {
		fired.Store(true)
		close(done)
	})

	// Before Advance: callback must not have run yet.
	assert.False(t, fired.Load(), "callback should not fire before Advance")

	tc.Advance(time.Second)

	// Wait for the goroutine launched by AfterFunc to complete.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("AfterFunc callback did not execute within 1s")
	}

	assert.True(t, fired.Load())
}

// TestTestClock_Advance_MultipleDeadlines checks that multiple timers with
// different deadlines are each fired during a single Advance that covers all
// of them.
func TestTestClock_Advance_MultipleDeadlines(t *testing.T) {
	tc := clock.NewTestClock(seed)

	// Collect fired IDs via a buffered channel to avoid shared-slice races.
	results := make(chan int, 3)

	record := func(n int) func() {
		return func() { results <- n }
	}

	tc.AfterFunc(3*time.Second, record(3))
	tc.AfterFunc(1*time.Second, record(1))
	tc.AfterFunc(2*time.Second, record(2))

	// One Advance to cover all three deadlines.
	tc.Advance(3 * time.Second)

	// Collect results with a timeout.
	got := make([]int, 0, 3)
	deadline := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case v := <-results:
			got = append(got, v)
		case <-deadline:
			t.Fatalf("timed out waiting for callbacks; got %v", got)
		}
	}

	// All three must have fired; order across goroutines is non-deterministic
	// so we just check that each ID appears exactly once.
	require.ElementsMatch(t, []int{1, 2, 3}, got)
}

// TestTestClock_Ticker_Stop ensures that Stop prevents future ticks from
// being delivered.
func TestTestClock_Ticker_Stop(t *testing.T) {
	tc := clock.NewTestClock(seed)
	ticker := tc.NewTicker(time.Second)

	tc.Advance(time.Second)
	// Drain the one buffered tick.
	<-ticker.C()

	ticker.Stop()

	// Advance again after stopping — no tick should arrive.
	tc.Advance(time.Second)
	select {
	case <-ticker.C():
		t.Fatal("received tick after Stop")
	default:
	}
}

// TestTestClock_Timer_Stop verifies that Stop() returns true when the timer
// hasn't fired yet, and false when already stopped.
func TestTestClock_Timer_Stop(t *testing.T) {
	tc := clock.NewTestClock(seed)

	timer := tc.AfterFunc(time.Second, func() {})

	// Stopping before the deadline: should return true.
	assert.True(t, timer.Stop(), "Stop before deadline should return true")

	// Stopping again: should return false (already stopped).
	assert.False(t, timer.Stop(), "Stop after already stopped should return false")

	// Advance past deadline — callback must not fire (we already stopped it).
	var fired atomic.Bool
	timer2 := tc.AfterFunc(time.Second, func() { fired.Store(true) })
	assert.True(t, timer2.Stop())
	tc.Advance(2 * time.Second)

	// Give any goroutine a moment to (incorrectly) run.
	time.Sleep(50 * time.Millisecond)
	assert.False(t, fired.Load(), "stopped timer callback must not fire")
}

// TestTestClock_After checks that After returns a channel that receives the
// clock time after the requested duration.
func TestTestClock_After(t *testing.T) {
	tc := clock.NewTestClock(seed)

	ch := tc.After(500 * time.Millisecond)

	// Nothing yet.
	select {
	case <-ch:
		t.Fatal("received value before Advance")
	default:
	}

	tc.Advance(500 * time.Millisecond)

	select {
	case got := <-ch:
		assert.Equal(t, seed.Add(500*time.Millisecond), got)
	default:
		t.Fatal("expected value on After channel after Advance")
	}
}

// TestTestClock_Since checks that Since returns the elapsed duration relative
// to the test clock's current time.
func TestTestClock_Since(t *testing.T) {
	tc := clock.NewTestClock(seed)

	past := seed.Add(-10 * time.Minute)
	assert.Equal(t, 10*time.Minute, tc.Since(past))

	tc.Advance(5 * time.Minute)
	assert.Equal(t, 15*time.Minute, tc.Since(past))
}

// TestOsClock_SatisfiesSchedulingInterface is a compile-time check that
// OsClock satisfies SchedulingClock. Runs as a no-op test.
func TestOsClock_SatisfiesSchedulingInterface(t *testing.T) {
	var _ clock.SchedulingClock = (*clock.OsClock)(nil)
}

// TestTestClock_SatisfiesSchedulingInterface verifies TestClock satisfies
// SchedulingClock at runtime.
func TestTestClock_SatisfiesSchedulingInterface(t *testing.T) {
	var _ clock.SchedulingClock = clock.NewTestClock(seed)
}
