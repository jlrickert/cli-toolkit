// Package clock provides time abstractions for testable code.
//
// # Core interfaces
//
// [Clock] exposes a single Now() method. [OsClock] implements it for
// production (delegates to time.Now); [TestClock] implements it for tests
// (allows deterministic time advancement via [TestClock.Advance]).
//
// [SchedulingClock] extends [Clock] with scheduling primitives:
// [SchedulingClock.NewTicker], [SchedulingClock.After],
// [SchedulingClock.AfterFunc], and [SchedulingClock.Since].
// Both [OsClock] and [TestClock] implement [SchedulingClock]: OsClock
// delegates to the standard library's time package, while TestClock drives
// all scheduling operations from [TestClock.Advance] for fully deterministic
// test scenarios.
//
// # Upgrading a Clock to SchedulingClock
//
// [ToSchedulingClock] upgrades any [Clock] to a [SchedulingClock]. If the
// value already implements [SchedulingClock], it is returned as-is. Otherwise
// it is wrapped in an adapter that provides OS-backed scheduling while
// delegating Now() to the original clock.
package clock
