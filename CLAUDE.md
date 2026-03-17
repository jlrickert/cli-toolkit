# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with
code in this repository.

## Quick Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./toolkit -v

# Run a single test
go test ./toolkit -run TestEnvGet -v

# Build/check code
go build ./...

# Release process (tags and updates changelog)
task release
```

## Project Overview

**cli-toolkit** is a Go library providing testable abstractions and helpers for
CLI programs. The core design philosophy is to avoid touching global process
state by using an explicit `Runtime` dependency container and test-friendly
implementations.

## Architecture & Key Concepts

### Runtime Dependency Container

The library passes a `*toolkit.Runtime` directly to functions instead of using
`context.Context` for dependency injection. Runtime bundles env, filesystem,
clock, logger, stream, and hasher. Each service has:

- A public interface (e.g., `Env`, `Clock`, `FileSystem`)
- An `OsXxx` implementation for production
- A `TestXxx` implementation for testing

**Example service flow:**

```go
rt, _ := toolkit.NewTestRuntime(jail, home, user) // Create test runtime
env := rt.Env()                                    // Access environment
fs := rt.FS()                                      // Access filesystem
lg := rt.Logger()                                  // Access logger
```

### Package Structure

- **`toolkit/`**: Core helpers - environment variables, filesystem operations,
  TTY detection, user paths, hashing
  - `Env` interface with `OsEnv` (production) and `TestEnv` (testing)
  - `FileSystem` interface for path resolution with optional jail support
  - `Runtime` as the main dependency hub
  - `Stream` struct modeling stdin/stdout/stderr

- **`apppaths/`** (package `appctx`): Application path management
  - `AppPaths` struct holding repository root and platform-scoped paths
    (config, data, state, cache)
  - Auto-detection of git repository roots
  - Public fields for all path roots

- **`mylog/`**: Structured logging on `log/slog`
  - `NewLogger()` for production loggers (JSON/text output)
  - `TestHandler` for capturing log entries in tests
  - `ParseLevel()`, `FindEntries()`, `RequireEntry()` test helpers

- **`clock/`**: Time abstraction
  - `Clock` interface with `OsClock` (production) and `TestClock` (testing)
  - `TestClock` allows deterministic time advancement via `Advance()`

- **`sandbox/`**: Comprehensive test environment
  - `Sandbox` bundles logger, env, clock, hasher, and jailed filesystem
  - `Process` runs individual `Runner` functions in isolation with I/O piping
  - `Pipeline` chains multiple stages with automatic stdout->stdin wiring
  - Fixture support for embedding test data

### Key Design Patterns

1. **No Global State**: All services accessed via `*Runtime`. Tests never
   modify `os.Environ()` or process clock.

2. **Jail/Sandbox Filesystem**: `TestEnv` confines operations to a jail
   directory. `FileSystem.ResolvePath()` respects jail boundaries.

3. **Test Data Embedding**: `Sandbox` supports loading fixtures from embedded
   filesystems.

4. **I/O Pipeline Testing**: `Process` and `Pipeline` let you test CLI programs
   by wiring stdin/stdout between functions.

## Testing Approach

The library provides three layers of test utilities:

1. **Unit Testing**: Use `Sandbox` for basic setup with logger, env, clock,
   hasher, and jailed filesystem.
   ```go
   sb := sandbox.NewSandbox(t, nil, sandbox.WithEnv("DEBUG", "true"))
   ctx := sb.Context()
   ```

2. **Process Isolation**: Use `Process` to run a function with configurable I/O.
   ```go
   p := sandbox.NewProcess(myRunner, false)
   result := p.Run(ctx)
   ```

3. **Pipeline Testing**: Use `Pipeline` to test multi-stage command chains with
   piped I/O.
   ```go
   pipeline := sandbox.NewPipeline(
     sandbox.Stage("producer", producerFn),
     sandbox.Stage("consumer", consumerFn),
   )
   result := pipeline.Run(ctx)
   ```

See `docs/testing.md` for detailed examples.

## Recent Changes

- **v0.2.1**: Documentation updates
- **v0.2.0**: Changelog updates
- **Latest refactor**: `AppContext` API changed from methods to public fields
  (`Root`, `ConfigRoot`, etc.)
- **Earlier**: Removed redundant error logging in `AtomicWriteFile`; renamed
  `project` package to `appctx`

## Important Files to Know

- `toolkit/runtime.go` - `Runtime` dependency container and forwarding methods
- `toolkit/runtime_impl.go` - `NewTestRuntime` and `NewOsRuntime`
- `toolkit/env.go` - `Env` interface (alias to `toolkit/env` subpackage)
- `toolkit/env_testenv.go` - `TestEnv` (alias to `toolkit/env` subpackage)
- `apppaths/context.go` - `AppPaths` struct and initialization
- `sandbox/sandbox.go` - `Sandbox` bundling test utilities
- `sandbox/process.go` - `Process` for function isolation
- `sandbox/pipeline.go` - `Pipeline` for multi-stage chains
- `mylog/logger.go` - Logger creation and configuration
- `clock/clock.go` - `Clock` interface and implementations

## Common Workflows

**Adding a new helper to toolkit:**

1. Define the interface (e.g., `type Foo interface { ... }`)
2. Implement `OsFoo` for production
3. Implement `TestFoo` for testing
4. Add a `WithRuntimeFoo()` option and accessor method to `Runtime`
5. Add tests to package `*_test.go` files using `Sandbox`

**Testing CLI code that uses toolkit:**

1. Use `Sandbox` to set up logger, clock, and jailed filesystem
2. Access the runtime via `sandbox.Runtime()`
3. Call your function with the sandbox runtime
4. Assert on logged entries via `TestHandler` and filesystem state via `Sandbox`
   file methods

**Debugging test failures:**

- Use `toolkit.DumpEnv(rt.Env())` to see the test environment
- Check `TestHandler` log entries: `handler.FindEntries(level)`
- Inspect jailed filesystem contents via `Sandbox.MustReadFile()`

## Testing Guidelines

- Tests use `stretchr/testify` assertions
- File operations in tests should go through `Sandbox.MustWriteFile()` and
  `Sandbox.MustReadFile()` to stay within the jail
- Log assertions use `TestHandler.FindEntries()` or `RequireEntry()`
- Time-sensitive tests use `TestClock.Advance()` for determinism
