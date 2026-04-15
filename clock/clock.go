package clock

import (
	"time"
)

// Clock is a small interface abstracting time operations the package needs.
// Use this to inject testable/fake clocks in unit tests rather than relying on
// time.Now directly.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// OsClock is a production Clock that delegates to time.Now.
type OsClock struct{}

// Now returns the current wall-clock time.
func (OsClock) Now() time.Time { return time.Now() }

// TestClock is a mutex-protected, manually-advancable clock useful for tests.
// It allows deterministic control of Now() by setting an initial time and
// advancing it as needed.
//
// TestClock implements [SchedulingClock]: tickers, timers, and AfterFunc
// callbacks are driven by [TestClock.Advance] rather than wall-clock time,
// making test behaviour fully deterministic.
type TestClock struct {
	now   time.Time
	sched *scheduler
}

// NewTestClock constructs a TestClock seeded to the provided time.
func NewTestClock(initial time.Time) *TestClock {
	c := &TestClock{now: initial}
	c.sched = newScheduler(&c.now)
	return c
}

// Now returns the current time of the TestClock.
func (c *TestClock) Now() time.Time {
	c.sched.mu.Lock()
	defer c.sched.mu.Unlock()
	return c.now
}

// Advance moves the TestClock forward by d, firing any tickers, timers, or
// AfterFunc callbacks whose deadline falls within [old_now, old_now+d].
// Events are fired in deadline order; equal-deadline events are fired in
// registration order (FIFO).
//
// The mutex is released before delivering tick values or launching AfterFunc
// goroutines, so callbacks that call Stop, Reset, or Advance are safe.
func (c *TestClock) Advance(d time.Duration) {
	c.sched.advance(d)
}

// Set sets the TestClock to a specific time without firing any scheduled
// events. Use Advance when you want events to fire.
func (c *TestClock) Set(t time.Time) {
	c.sched.mu.Lock()
	c.now = t
	c.sched.mu.Unlock()
}

// NewTicker returns a [Ticker] that fires every d. The tick channel has a
// buffer of 1; if the receiver is slow, ticks are dropped (matching stdlib
// behaviour).
func (c *TestClock) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("clock: NewTicker called with non-positive interval")
	}
	c.sched.mu.Lock()
	ts := c.sched.addTicker(d)
	c.sched.mu.Unlock()
	return &testTicker{ts: ts, sched: c.sched}
}

// After returns a channel that receives the current time after duration d.
// The send is non-blocking (buffered channel, size 1).
func (c *TestClock) After(d time.Duration) <-chan time.Time {
	c.sched.mu.Lock()
	e := c.sched.addAfter(d)
	c.sched.mu.Unlock()
	return e.afterC
}

// AfterFunc calls f in a new goroutine after duration d and returns a [Timer]
// that can cancel the call.
func (c *TestClock) AfterFunc(d time.Duration, f func()) Timer {
	c.sched.mu.Lock()
	e := c.sched.addAfterFunc(d, f)
	c.sched.mu.Unlock()
	// Reset needs to swap in a freshly-pushed heap entry without mutating the
	// original pointer that other callers may hold. Store a **schedEntry so
	// Reset can overwrite the slot atomically; eRef is the heap-allocated slot.
	eRef := e
	return &testTimer{entryRef: &eRef, sched: c.sched}
}

// Since returns the time elapsed since t, equivalent to c.Now().Sub(t).
func (c *TestClock) Since(t time.Time) time.Duration {
	return c.Now().Sub(t)
}

var _ Clock = (*OsClock)(nil)
var _ Clock = (*TestClock)(nil)
var _ SchedulingClock = (*TestClock)(nil)

var defaultClock Clock = &OsClock{}

// Default returns the process default clock.
func Default() Clock { return defaultClock }

// OrDefault returns c unless it is nil, in which case the default clock is returned.
func OrDefault(c Clock) Clock {
	if c != nil {
		return c
	}
	return defaultClock
}
