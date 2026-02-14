# cli-toolkit - Helpers for CLI programs and unit tests

Small, focused helpers for command line programs and tests. The library provides
testable abstractions for environment handling, time, file system operations,
logging, hashing, and common user paths. It is designed to make code easier to
test without touching global process state.

## Highlights

- Testable environment via `TestEnv` that does not modify the OS env.
- Explicit `Runtime` dependency container (Env, FS, clock, logger, stream, hasher).
- Test clock via `TestClock` to deterministically control `Now()`.
- File utilities for safe writes, path resolution, and test stdio.
- Small logging helpers built on `log/slog` and a test handler.
- Simple hashing interface with a default MD5 hasher.
- Helpers for user-scoped directories and a `project` helper for app roots.
- `Sandbox` for comprehensive test setup with jailed filesystem, test clock,
  logger, and environment.
- Pipeline execution with `Process` and `Pipeline` for testing complex I/O
  scenarios.

## Packages

### Core Toolkit (`toolkit`)

Main package with filesystem, environment, and I/O utilities:

- **Environment**: `Env` interface with `OsEnv` and `TestEnv` implementations.
- **Filesystem**: `FileSystem` interface with `OsFS` implementation.
- **Runtime**: Explicit dependency wiring via `toolkit.Runtime`.
- **Streams**: `Stream` struct modeling stdin/stdout/stderr with TTY and pipe
  detection.
- **Utilities**: File operations, editor launching, environment inspection, user
  path helpers.

### App Context (`appctx`)

Application root and configuration management:

- **AppContext struct**: Manages repository root and platform-scoped paths
  (config, data, state, cache).
- **Options**: `WithRoot()`, `WithAutoRootDetect()` for git repository
  detection, and per-path customization.

### Logging (`mylog`)

Structured logging built on `log/slog`:

- **NewLogger()**: Create configured loggers with JSON/text output and level
  control.
- **TestHandler**: Captures log entries for test assertions.
- **Utilities**: `ParseLevel()` for level names, `FindEntries()` and
  `RequireEntry()` for test helpers.

### Clock (`clock`)

Time abstraction for testable code:

- **Clock interface**: Abstract time operations.
- **OsClock**: Production implementation using `time.Now()`.
- **TestClock**: Manual time control for deterministic tests.

### Sandbox (`sandbox`)

Comprehensive test environment bundling common setup:

- **Sandbox**: Combines test logger, environment, clock, hasher, and jailed
  filesystem.
- **Process**: Isolated function execution with configurable I/O streams.
- **Pipeline**: Sequential stage execution with piped I/O.
- **Options**: Configure clock, environment, working directory, and test
  fixtures.

## Install

```
go get github.com/jlrickert/cli-toolkit
```

## Examples

### Testable environment and variable expansion

```go
env := toolkit.NewTestEnv("/tmp/jail", "/home/alice", "alice")
_ = env.Set("FOO", "bar")
rt, _ := toolkit.NewRuntime(
  toolkit.WithRuntimeEnv(env),
  toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
  toolkit.WithRuntimeJail("/tmp/jail"),
)

out := toolkit.ExpandEnv(rt.Env, "$FOO/baz")
// out == "bar/baz" on unix-like platforms
```

### Test clock

```go
tc := clock.NewTestClock(time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC))
ctx := clock.WithClock(context.Background(), tc)

now := clock.ClockFromContext(ctx).Now()
tc.Advance(2 * time.Hour)
// Now reflects advanced time
```

### Atomic file write

```go
env := toolkit.NewTestEnv("/tmp/jail", "", "")
rt, _ := toolkit.NewRuntime(
  toolkit.WithRuntimeEnv(env),
  toolkit.WithRuntimeFileSystem(&toolkit.OsFS{}),
  toolkit.WithRuntimeJail("/tmp/jail"),
)
err := rt.AtomicWriteFile("/tmp/some/file.txt", []byte("data"), 0644)
if err != nil {
    // handle
}
```

### Test logger

```go
lg, th := mylog.NewTestLogger(t, mylog.ParseLevel("debug"))
ctx := mylog.WithLogger(context.Background(), lg)
// use ctx in code under test and assert logs in `th`
```

### App Context helper

```go
aCtx, err := appctx.NewAppContext(rt, "/path/to/repo", "myapp")
cfgRoot := aCtx.ConfigRoot
// cfgRoot == <user-config-dir>/myapp
```

### Sandbox with test setup

```go
sb := sandbox.NewSandbox(t, nil,
  sandbox.WithClock(time.Now()),
  sandbox.WithEnv("DEBUG", "true"))
sb.MustWriteFile("config.txt", []byte("data"), 0644)
// Use sb.Runtime() and sb in test
```

## Testing

Run all tests with:

```
go test ./...
```

Many helpers provide test-friendly variants and fixtures. See
`sandbox.NewSandbox` for comprehensive test setup that wires a `TestEnv`,
`TestClock`, test logger, hasher, and jailed filesystem.

## Contributing

Contributions and issues are welcome. Please open an issue or a pull request
with a short description and tests for new behavior.

## Files to inspect

- `toolkit/` - core helpers (env, filesystem, streams, paths)
- `appctx/` - app path helpers
- `mylog/` - structured logging utilities
- `clock/` - time abstractions
- `sandbox/` - comprehensive test setup

## Notes

- The library aims to be small and easy to audit. Tests avoid touching real OS
  state by using `TestEnv` and `TestClock`.
- See repository files for detailed behavior and examples.
- Runtime dependencies are passed explicitly via `toolkit.Runtime`.

## License

See the repository root for license information.
