// Package toolkit provides testable abstractions and helpers for CLI programs.
//
// The central type is [Runtime], an explicit dependency container that bundles
// environment variables, filesystem operations, clock, logger, streams, and
// hasher. Production code uses [NewRuntime] or [NewOsRuntime]; test code uses
// [NewTestRuntime] which wires in-memory implementations and a jailed
// filesystem.
//
// Key interfaces:
//   - [Env] for environment variable access (implemented by OsEnv and TestEnv)
//   - [FileSystem] for filesystem operations (implemented by OsFS)
//   - [Hasher] for deterministic content hashing
//
// Helper functions provide cross-platform user path resolution
// ([UserConfigPath], [UserDataPath], [UserStatePath], [UserCachePath]) and
// environment variable expansion ([ExpandEnv], [ExpandPath]).
package toolkit
