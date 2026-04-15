package clock

import "time"

// Ticker is the minimal interface required from a repeating ticker.
// It mirrors the relevant subset of [time.Ticker] behind an interface
// so that implementations (production and test) can be swapped.
type Ticker interface {
	// C returns the channel on which ticks are delivered.
	C() <-chan time.Time
	// Stop turns off the ticker. After Stop, no more ticks will be delivered.
	Stop()
}

// Timer is the minimal interface required from a one-shot timer.
// It mirrors the relevant subset of [time.Timer] behind an interface.
type Timer interface {
	// Stop prevents the timer from firing. It returns true if the call stops
	// the timer, false if the timer has already expired or been stopped.
	Stop() bool
	// Reset changes the timer to expire after duration d. It returns true if
	// the timer had been active, false if the timer had already expired or
	// been stopped. Reset must only be invoked on stopped or expired timers.
	Reset(d time.Duration) bool
}

// SchedulingClock extends [Clock] with scheduling primitives that can be
// controlled deterministically in tests.
//
// Both [OsClock] and [TestClock] implement SchedulingClock: OsClock delegates
// to the standard library time package, TestClock uses a min-heap scheduler
// driven by [TestClock.Advance].
type SchedulingClock interface {
	Clock

	// NewTicker returns a [Ticker] that fires at every interval d.
	// Panics if d <= 0 (same contract as time.NewTicker).
	NewTicker(d time.Duration) Ticker

	// After returns a channel that receives the current time after duration d.
	After(d time.Duration) <-chan time.Time

	// AfterFunc calls f in a new goroutine after duration d and returns a
	// [Timer] that can be used to cancel the call.
	AfterFunc(d time.Duration, f func()) Timer

	// Since returns the time elapsed since t, equivalent to Now().Sub(t).
	Since(t time.Time) time.Duration
}

// --- OS (production) scheduling methods ---
//
// OsClock itself implements SchedulingClock; the methods are defined here to
// keep the scheduling surface grouped in one file.

// NewTicker returns a production ticker backed by [time.NewTicker].
func (OsClock) NewTicker(d time.Duration) Ticker {
	return &osTicker{time.NewTicker(d)}
}

// After returns a channel that receives the time after duration d, backed by
// [time.After].
func (OsClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// AfterFunc calls f in a new goroutine after duration d, backed by
// [time.AfterFunc].
func (OsClock) AfterFunc(d time.Duration, f func()) Timer {
	return &osTimer{time.AfterFunc(d, f)}
}

// Since returns the time elapsed since t, backed by [time.Since].
func (OsClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

var _ SchedulingClock = (*OsClock)(nil)

// osTicker wraps *time.Ticker to satisfy the [Ticker] interface.
type osTicker struct {
	t *time.Ticker
}

func (w *osTicker) C() <-chan time.Time { return w.t.C }
func (w *osTicker) Stop()               { w.t.Stop() }

// osTimer wraps *time.Timer to satisfy the [Timer] interface.
type osTimer struct {
	t *time.Timer
}

func (w *osTimer) Stop() bool                 { return w.t.Stop() }
func (w *osTimer) Reset(d time.Duration) bool { return w.t.Reset(d) }

var (
	_ Ticker = (*osTicker)(nil)
	_ Timer  = (*osTimer)(nil)
)

// ToSchedulingClock upgrades c to a [SchedulingClock]. If c already implements
// SchedulingClock it is returned directly; otherwise it is wrapped in an
// adapter that delegates Now() to c and uses OS-backed scheduling primitives.
//
// Both [OsClock] and [TestClock] implement SchedulingClock directly, so this
// helper is only needed for third-party [Clock] implementations that do not.
// The adapter is not appropriate for deterministic test clocks — use a
// [TestClock] instead.
func ToSchedulingClock(c Clock) SchedulingClock {
	if sc, ok := c.(SchedulingClock); ok {
		return sc
	}
	return &osSchedulingAdapter{c}
}

// osSchedulingAdapter wraps a non-scheduling [Clock] with OS-level scheduling
// primitives. Now() comes from the wrapped clock; scheduling uses stdlib.
type osSchedulingAdapter struct {
	Clock
}

func (a *osSchedulingAdapter) NewTicker(d time.Duration) Ticker {
	return &osTicker{time.NewTicker(d)}
}

func (a *osSchedulingAdapter) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (a *osSchedulingAdapter) AfterFunc(d time.Duration, f func()) Timer {
	return &osTimer{time.AfterFunc(d, f)}
}

func (a *osSchedulingAdapter) Since(t time.Time) time.Duration {
	return a.Now().Sub(t)
}

var _ SchedulingClock = (*osSchedulingAdapter)(nil)
