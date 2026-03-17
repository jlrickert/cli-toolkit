Update `pkg/user_test.go` to use the correct `NewTestEnv` signature and make
path expectations robust across platforms.

Please apply the following edits to `pkg/user_test.go`:

1. Fix `NewTestEnv` calls

- The project `NewTestEnv` signature is
  `NewTestEnv(jail, home, username string)`.
- Replace two-argument calls like: `std.NewTestEnv("/home/alice", "alice")` with
  an explicit three-argument call. Choose one of these patterns depending on the
  test intent:

  - If the test expects raw, unjailed home paths (most existing expectations
    that assert exact "/home/..." or Windows absolute strings), pass an empty
    jail so values are returned unchanged:
    `env := std.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")`

  - If the test should exercise jailed behavior, create a temp jail:
    ```
    jail := t.TempDir()
    env := std.NewTestEnv(jail, filepath.Join("home", "alice"), "alice")
    ```
    Then compute expected values using `std.EnsureInJail(env.Root(), ...)`.

2. Compute expected paths portably

- Replace hard-coded expected strings where appropriate with calls that compute
  the expected path using `filepath` and `EnsureInJail`. Examples:

  - For a raw XDG override test:
    ```
    require.NoError(t, env.Set("XDG_CONFIG_HOME", "/real/xdg"))
    cfg, err := std.UserConfigPath(std.WithEnv(context.Background(), env))
    require.NoError(t, err)
    assert.Equal(t, "/real/xdg", cfg)
    ```

  - For fallback to `$HOME/.config` on Unix-like:
    ```
    env := std.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")
    cfg, err := std.UserConfigPath(std.WithEnv(context.Background(), env))
    require.NoError(t, err)
    expected := filepath.Join("/home/alice", ".config")
    assert.Equal(t, expected, cfg)
    ```

  - For tests using a jailed TestEnv:
    ```
    jail := t.TempDir()
    env := std.NewTestEnv(jail, filepath.Join("home", "alice"), "alice")
    cfg, err := std.UserConfigPath(std.WithEnv(context.Background(), env))
    require.NoError(t, err)
    expected := std.EnsureInJail(env.Root(), filepath.Join("home", "alice", ".config"))
    assert.Equal(t, filepath.Clean(expected), filepath.Clean(cfg))
    ```

3. Use `filepath.FromSlash` and `filepath.Join`

- Where tests currently use forward-slash literals for paths, wrap them with
  `filepath.FromSlash` or build them with `filepath.Join` so tests compile and
  behave the same on Windows and Unix.

4. Ensure `require.NoError` before asserting results

- When a test calls `env.Set(...)` or other setup that can return an error, use
  `require.NoError(t, err)` to make test failures clearer.

5. Keep platform-specific branches using `runtime.GOOS`

- Continue to guard Windows-only expectations with:
  `if runtime.GOOS == "windows" { ... } else { ... }`

6. Update `ExpandPath` tests that depend on jail behavior

- `ExpandPath` tests that assert on the jailed expansion should use a
  `jail := t.TempDir()` and construct the TestEnv with that jail so the returned
  home path includes the jail. Compute expectations via
  `std.EnsureInJail(env.Root(), ...)` or `strings.HasSuffix` on the result (the
  latter is acceptable as in the existing test where the exact jail path is not
  asserted).

7. Example replacements

- Replace this incorrect two-arg construct:
  `env := std.NewTestEnv("/home/alice", "alice")` With either:
  `env := std.NewTestEnv("", filepath.FromSlash("/home/alice"), "alice")` or:
  ```
  jail := t.TempDir()
  env := std.NewTestEnv(jail, filepath.Join("home", "alice"), "alice")
  ```

8. Run tests locally

- After making changes run: `go test ./pkg -run TestUser -v` to verify the
  modified `user` tests pass. Run the whole test suite `go test ./...` if
  convenient.

Notes and rationale

- The primary problem is mismatched `NewTestEnv` usage. Passing an explicit
  `jail` argument controls whether TestEnv adjusts home paths.
- Using `EnsureInJail` and `filepath` helpers avoids platform-specific
  assumptions and keeps tests stable across OSes.

