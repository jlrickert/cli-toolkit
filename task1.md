Goal

- Make the test suite robust and correct for Env / TestEnv / OsEnv across
  Windows, FreeBSD, Linux, and macOS. Fix failing tests and add additional unit
  tests that cover platform-specific behavior and edge cases for NewTestEnv,
  TestEnv, and OsEnv. Change production code behavior is fine.

Repository context

- Package: pkg (import path: github.com/jlrickert/go-std/pkg)
- Test framework: testing + github.com/stretchr/testify/assert & require
- Existing tests to update: pkg/env_test.go (compilation and cross-platform
  issues)
- Files to add: new tests under pkg/*_test.go as described below

High-level requirements

1. Fix failing/incorrect tests in pkg/env_test.go so they compile and are
   cross-platform.
   - Ensure the package import is aliased as std when calling package functions
     in tests that live in package std_test.
   - Replace any hard-coded path strings using forward slashes with
     platform-correct constructs (filepath.Join, os.PathSeparator, etc).
   - Use std.EnsureInJail(m.Root(), path) to compute expected jailed paths, not
     assumptions about jail being empty.
   - Replace incorrect testify usages (e.g., assert.Subset used on strings) with
     appropriate assertions.

2. Add focused tests to cover Env / TestEnv / OsEnv behavior (high-priority
   tests first).
   - NewTestEnv defaults
     - Test that NewTestEnv("", "") returns a TestEnv with non-empty home and
       username.
     - Assert platform-specific keys are set: On Unix-like, XDG_CONFIG_HOME,
       XDG_CACHE_HOME, XDG_DATA_HOME, XDG_STATE_HOME, TMPDIR; on Windows,
       APPDATA, LOCALAPPDATA, TMPDIR under Local.
     - Compute expected values using filepath.Join and
       std.EnsureInJail(m.Root(), ...) instead of hard-coded strings.

   - GetHome / SetHome / Unset("HOME")
     - Test that GetHome returns EnsureInJail(jail, home) when a home was set
       during NewTestEnv or via Set("HOME", ...).
     - After Set("HOME", newHome), subsequent GetHome returns the new (jailed)
       value.
     - After Unset("HOME"), GetHome returns an error.
     - Use filepath and EnsureInJail for expected values.

   - GetUser / SetUser / Unset("USER")
     - Test GetUser errors when user not set (nil/zero TestEnv).
     - Test Set("USER", name) updates both m.user and m.data["USER"].
     - Test Unset removes user info and GetUser returns error.

   - Nil receiver behavior
     - Tests asserting behavior on nil and zero-value TestEnv:
       - m := std.TestEnv{}; m.Get("KEY") returns "" (no panic)
       - var mnil *std.TestEnv = nil; mnil.Set(...) returns an error
       - var mnil *std.TestEnv = nil; mnil.Unset(...) is a no-op (no panic)
       - GetWd on zero-value or nil returns error

   - GetTempDir precedence and platform fallbacks
     - Cover precedence: TMPDIR -> TEMP/TMP -> (Windows) LOCALAPPDATA -> APPDATA
       -> USERPROFILE -> home-derived default -> "" (Windows no info) ; (Unix)
       fallback EnsureInJail(jail, "/tmp").
     - Test nil receiver returns os.TempDir().
     - Build test keys in m.data and assert expected path (use
       EnsureInJail(m.Root(), value) when appropriate).

   - Mkdir / WriteFile jailed path behavior
     - Test that calling m.Mkdir or m.WriteFile uses EnsureInJail(m.jail, path)
       and creates files/dirs under the jail.
     - Use t.Cleanup to remove created files.
     - Read back contents for WriteFile to assert correct write.

   - OsEnv platform-specific SetHome / SetUser
     - OsEnv.SetHome(home) should call os.Setenv("HOME", home) and, on Windows,
       also set USERPROFILE = home.
     - OsEnv.SetUser(username) should set USER and, on Windows, also USERNAME.
     - Tests should set/restore real OS env values (use t.Cleanup). Do not run
       these tests in parallel with others (they mutate global state); keep them
       sequential.

   - EnvFromContext / WithEnv / ExpandEnv interactions
     - WithEnv(ctx, env) should cause EnvFromContext(ctx) to return the same env
       pointer.
     - ExpandEnv(std.WithEnv(ctx, env), "$FOO/bar") should expand using env.Get.
     - ExpandEnv with no env in context should fall back to real OsEnv—test by
       setting an OS env var temporarily.

3. Cross-platform/CI hygiene
   - Use filepath.Join, filepath.Base, filepath.Clean, os.PathSeparator when
     constructing or asserting paths.
   - Use std.EnsureInJail(m.Root(), path) to compute expected jailed outputs.
   - Use t.Cleanup to restore any mutated os.Setenv values and to remove created
     files.
   - Mark tests that mutate the real process environment (OsEnv.SetHome/SetUser)
     as non-parallel (do not call t.Parallel()).
   - Use runtime.GOOS branches in tests to assert platform-specific
     expectations. Tests must run on windows, freebsd, linux, macosx.
   - Avoid tests that require admin privileges (e.g., creating symlinks on
     Windows) — skip with t.Skip when necessary.
   - Where a test might be sensitive to permissions or external tools, prefer to
     test fallback logic or use temporary directories in the TestEnv jail.

4. Test naming and organization
   - Fix and keep existing pkg/env_test.go updated for the corrected tests.
   - Add additional tests to new files grouped by theme:
     - pkg/env_test.go (fixes + general ExpandEnv/GetDefault/GetWd tests)
     - pkg/testenv_test.go (NewTestEnv, GetTempDir precedence, Mkdir/WriteFile,
       nil receiver)
     - pkg/osenv_test.go (OsEnv.SetHome/SetUser real-OS env mutation tests; run
       sequentially)
   - Use clear test names (TestNewTestEnvDefaults,
     TestTestEnvGetTempDirPrecedence, TestTestEnvSetHomeUnset,
     TestOsEnvSetHomeWindowsBehavior, etc.).

5. Assertions & idioms
   - Use require.NoError(t, err) for setup operations that must succeed.
   - Use assert.Equal/NotEmpty/Contains as appropriate.
   - For path equality, use assert.Equal(t, filepath.Clean(expected),
     filepath.Clean(actual)).
   - Prefer computing expected path with std.EnsureInJail(m.Root(), provided) to
     avoid mistakes.

6. Lint, format, run tests
   - Run go test ./... on Linux initially and ensure tests pass.
   - Run golangci-lint or go vet if available.
   - Ensure tests compile and pass on other platforms (Windows, FreeBSD, macOS).
     If the coding agent has CI, run tests on those platforms or at least run
     with GOOS=windows for compile verification.

Deliverables

- A branch or patch that:
  - Fixes pkg/env_test.go compile/cross-platform problems.
  - Adds the new tests under pkg/ as described.
  - Ensures OsEnv tests restore original OS env values with t.Cleanup and are
    not run in parallel.
  - Includes a short commit message that explains the changes and the reason
    (e.g., "tests: fix env_test cross-platform issues; add TestEnv/OsEnv
    coverage for platform-specific behavior").
- A short summary comment in the PR/commit describing anything important
  discovered (e.g., if you found a real bug in production code, describe it and
  include the minimal fix — only if necessary).

Non-goals / constraints

- Do not replace testify with a different assertion library.
- Do not globally change package names or the public APIs unless a genuine bug
  prevents testing and you document the change.
- Avoid introducing flaky tests — use t.Cleanup and restore global state where
  mutated.
