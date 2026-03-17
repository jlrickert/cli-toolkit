// Package clock provides a time abstraction for testable code.
//
// The [Clock] interface has a single method Now() and is implemented by
// [OsClock] for production (delegates to time.Now) and [TestClock] for tests
// (allows deterministic time advancement via Advance).
package clock
