// Package sandbox provides comprehensive test environment setup for CLI
// application testing.
//
// [Sandbox] bundles a jailed filesystem, test environment, test clock, logger,
// hasher, and stream into a single test harness. Use [NewSandbox] with
// functional options ([WithEnv], [WithClock], [WithFixture], [WithWd]) to
// configure the test environment.
//
// [Process] runs a [Runner] function in isolation with configurable I/O
// streams. [Pipeline] chains multiple stages with automatic stdout-to-stdin
// wiring between stages, enabling realistic multi-stage CLI testing.
package sandbox
