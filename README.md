# cli-toolkit - Helpers for CLI programs and unit tests

Small, focused helpers for command line programs and tests. The library provides
testable abstractions for environment handling, time, file system operations,
logging, hashing, and common user paths.

Recent changes moved dependency wiring to an explicit `Runtime` model. Context
values are no longer used for runtime dependencies like
clock/logger/stream/hasher.

## Highlights

- Testable environment via `TestEnv` without mutating OS process env.
- Explicit `Runtime` dependency container (`Env`, `FS`, clock, logger, stream,
  hasher).
- Defaulting helpers: `clock.OrDefault`, `mylog.OrDefault`,
  `toolkit.OrDefaultStream`, and `toolkit.OrDefaultHasher`.
- User-path helpers for config/data/state/cache plus repository-aware app paths.
- `Sandbox` test harness with jailed filesystem, test clock, logger, and env.
- `Process` and `Pipeline` helpers for testing CLI-style I/O flows.

## Packages

### Core Toolkit (`toolkit`)

- `Env` interface with `OsEnv` and `TestEnv` implementations.
- `FileSystem` interface with `OsFS` implementation.
- `Runtime` as the main dependency hub (`NewRuntime`, `NewTestRuntime`,
  `NewOsRuntime`).
- `Stream` model for stdin/stdout/stderr with TTY/piped metadata.
- Path/file helpers (`ResolvePath`, `AbsPath`, `AtomicWriteFile`, `Glob`, etc.).

### App Paths (`apppaths`, package name `appctx`)

- `AppPaths` struct for repository and platform-scoped app roots.
- `NewAppPaths(rt, root, appname)` for explicit root wiring.
- `NewGitAppPaths(ctx, rt, appname)` for git-root discovery with fallback
  scanning.

### Logging (`mylog`)

- `NewLogger` for text/JSON slog logger setup.
- `NewTestLogger` + `TestHandler` for log assertions in tests.
- `ParseLevel`, `FindEntries`, and `RequireEntry` test helpers.
- `Default`/`OrDefault` logger helpers.

### Clock (`clock`)

- `Clock` interface with `OsClock` and `TestClock`.
- `Default`/`OrDefault` clock helpers.

### Sandbox (`sandbox`)

- `NewSandbox` for end-to-end test setup.
- `WithEnv`, `WithEnvMap`, `WithWd`, `WithClock`, `WithFixture` options.
- `Process` and `Pipeline` for isolated execution and piped stage testing.

## Install

```sh
go get github.com/jlrickert/cli-toolkit
```

## Examples

### Runtime + environment expansion

```go
rt, _ := toolkit.NewTestRuntime("/tmp/jail", "/home/alice", "alice")
_ = rt.Set("FOO", "bar")

out := toolkit.ExpandEnv(rt, "$FOO/baz")
// out == "bar/baz" on unix-like platforms
```

### Test clock

```go
tc := clock.NewTestClock(time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC))
c := clock.OrDefault(tc)

now := c.Now()
tc.Advance(2 * time.Hour)
later := c.Now()
_ = now
_ = later
```

### Atomic file write

```go
rt, _ := toolkit.NewTestRuntime("/tmp/jail", "/home/alice", "alice")
err := rt.AtomicWriteFile("some/file.txt", []byte("data"), 0o644)
if err != nil {
	// handle
}
```

### Test logger

```go
lg, th := mylog.NewTestLogger(t, mylog.ParseLevel("debug"))
rt, _ := toolkit.NewTestRuntime(
	"/tmp/jail",
	"/home/alice",
	"alice",
	toolkit.WithRuntimeLogger(lg),
)

rt.Logger().Debug("example message")
_ = th // assert captured entries in tests
```

### App paths helper

```go
paths, err := appctx.NewAppPaths(rt, "/path/to/repo", "myapp")
if err != nil {
	// handle
}
cfgRoot := paths.ConfigRoot
// cfgRoot == <user-config-dir>/myapp
```

### Sandbox with test setup

```go
sb := sandbox.NewSandbox(
	t,
	nil,
	sandbox.WithClock(time.Now()),
	sandbox.WithEnv("DEBUG", "true"),
)
sb.MustWriteFile("config.txt", []byte("data"), 0o644)
// Use sb.Runtime() and sb in tests
```

## Migration notes

Context-based runtime dependency helpers were removed in favor of explicit
runtime wiring:

- `clock.WithClock` / `clock.ClockFromContext` -> use `toolkit.Runtime.Clock()`
  or `clock.OrDefault`.
- `mylog.WithLogger` / `mylog.LoggerFromContext` -> use
  `toolkit.Runtime.Logger()` or `mylog.OrDefault`.
- `toolkit.WithHasher` / `toolkit.HasherFromContext` -> use
  `toolkit.Runtime.Hasher()` or `toolkit.OrDefaultHasher`.
- `toolkit.WithStream` / `toolkit.StreamFromContext` -> use
  `toolkit.Runtime.Stream()` or `toolkit.OrDefaultStream`.

## Testing

Run all tests with:

```sh
go test ./...
```

Many helpers provide test-friendly variants and fixtures. `sandbox.NewSandbox`
wires `TestEnv`, `TestClock`, test logger, hasher, and jailed filesystem.

## Contributing

Contributions and issues are welcome. Please open an issue or pull request with
a short description and tests for new behavior.

## Files to inspect

- `toolkit/` - core helpers (env, filesystem, runtime, streams, paths)
- `apppaths/` - app path and git-root helpers (package name `appctx`)
- `mylog/` - structured logging utilities
- `clock/` - time abstractions
- `sandbox/` - comprehensive test setup

## Notes

- The library aims to stay small and easy to audit.
- Tests avoid touching real OS state via `TestEnv`, `TestClock`, and jailed
  paths.
- Runtime dependencies are passed explicitly via `toolkit.Runtime`.

## License

See the repository root for license information.
