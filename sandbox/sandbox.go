package sandbox

import (
	"context"
	"embed"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/jlrickert/cli-toolkit/mylog"
	"github.com/jlrickert/cli-toolkit/toolkit"
)

// Option is a function used to modify a Sandbox during construction.
type Option func(f *Sandbox)

// Sandbox bundles common test setup used by package tests.
type Sandbox struct {
	t *testing.T

	data embed.FS
	ctx  context.Context
	rt   *toolkit.Runtime
}

// Options holds optional settings provided to NewSandbox.
type Options struct {
	Data embed.FS
	Home string
	User string
}

// NewSandbox constructs a Sandbox and applies given options.
func NewSandbox(t *testing.T, options *Options, opts ...Option) *Sandbox {
	t.Helper()
	jail := t.TempDir()

	var home string
	var user string
	var data embed.FS
	if options != nil {
		home = options.Home
		user = options.User
		data = options.Data
	}

	lg, _ := mylog.NewTestLogger(t, mylog.ParseLevel("debug"))
	clk := clock.NewTestClock(time.Date(2025, 10, 15, 12, 30, 0, 0, time.UTC))
	hasher := &toolkit.MD5Hasher{}
	stream := toolkit.DefaultStream()

	rt, err := toolkit.NewTestRuntime(
		jail,
		home,
		user,
		toolkit.WithRuntimeClock(clk),
		toolkit.WithRuntimeLogger(lg),
		toolkit.WithRuntimeStream(stream),
		toolkit.WithRuntimeHasher(hasher),
	)
	if err != nil {
		t.Fatalf("NewSandbox: runtime init failed: %v", err)
	}

	ctx := t.Context()

	f := &Sandbox{
		t:    t,
		ctx:  ctx,
		data: data,
		rt:   rt,
	}

	for _, opt := range opts {
		opt(f)
	}

	t.Cleanup(func() { f.cleanup() })
	return f
}

// WithEnv returns an Option that sets a single environment variable
// in the sandbox's Env.
func WithEnv(key, val string) Option {
	return func(f *Sandbox) {
		f.t.Helper()
		if err := f.runtimeEnv().Set(key, val); err != nil {
			f.t.Fatalf("WithEnv failed to set %s: %v", key, err)
		}
	}
}

// WithWd returns an Option that sets the sandbox working directory.
func WithWd(rel string) Option {
	return func(sandbox *Sandbox) {
		sandbox.t.Helper()
		path, err := sandbox.ResolvePath(rel)
		if err != nil {
			sandbox.t.Fatalf("WithWd: resolve %q failed: %v", rel, err)
		}
		if err := sandbox.rt.Setwd(path); err != nil {
			sandbox.t.Fatalf("WithWd: setwd %q failed: %v", path, err)
		}
	}
}

// WithClock returns an Option that sets the test clock to the provided time.
func WithClock(t0 time.Time) Option {
	return func(f *Sandbox) {
		f.t.Helper()
		f.testClock().Set(t0)
	}
}

// WithEnvMap returns an Option that seeds multiple environment variables.
func WithEnvMap(m map[string]string) Option {
	return func(f *Sandbox) {
		f.t.Helper()
		for k, v := range m {
			if err := f.runtimeEnv().Set(k, v); err != nil {
				f.t.Fatalf("WithEnvMap set %s failed: %v", k, err)
			}
		}
	}
}

// WithFixture copies an embedded fixture directory into the sandbox jail.
func WithFixture(fixture string, path string) Option {
	return func(f *Sandbox) {
		f.t.Helper()

		src := filepath.Join("data", fixture)
		if _, err := iofs.Stat(f.data, src); err != nil {
			f.t.Fatalf("WithFixture: source %s not found: %v", src, err)
		}

		p, err := f.ResolvePath(path)
		if err != nil {
			f.t.Fatalf("WithFixture: resolve %s failed: %v", path, err)
		}
		dst := filepath.Join(f.GetJail(), p)
		if err := copyEmbedDir(f.data, src, dst); err != nil {
			f.t.Fatalf("WithFixture: copy %s -> %s failed: %v", src, dst, err)
		}
	}
}

func (sandbox *Sandbox) GetJail() string {
	if sandbox.rt == nil {
		return ""
	}
	return sandbox.rt.GetJail()
}

// Context returns the sandbox context.
func (sandbox *Sandbox) Context() context.Context {
	return sandbox.ctx
}

// Runtime returns the sandbox runtime.
func (sandbox *Sandbox) Runtime() *toolkit.Runtime {
	return sandbox.rt
}

// AbsPath returns a runtime absolute path.
func (sandbox *Sandbox) AbsPath(rel string) (string, error) {
	sandbox.t.Helper()
	return sandbox.rt.AbsPath(rel)
}

// ReadFile reads a file located under the sandbox jail.
func (sandbox *Sandbox) ReadFile(rel string) ([]byte, error) {
	sandbox.t.Helper()
	return sandbox.rt.ReadFile(rel)
}

// MustReadFile reads a file under the jail and fails the test on error.
func (sandbox *Sandbox) MustReadFile(rel string) []byte {
	sandbox.t.Helper()
	b, err := sandbox.ReadFile(rel)
	if err != nil {
		sandbox.t.Fatalf("MustReadFile %s failed: %v", rel, err)
	}
	return b
}

func (sandbox *Sandbox) AtomicWriteFile(rel string, data []byte, perm os.FileMode) error {
	sandbox.t.Helper()
	if sandbox.GetJail() == "" {
		return fmt.Errorf("no jail set")
	}
	return sandbox.rt.AtomicWriteFile(rel, data, perm)
}

// WriteFile writes data to a path under the sandbox jail.
func (sandbox *Sandbox) WriteFile(rel string, data []byte, perm os.FileMode) error {
	sandbox.t.Helper()
	return sandbox.rt.WriteFile(rel, data, perm)
}

// MustWriteFile writes data under the jail and fails the test on error.
func (sandbox *Sandbox) MustWriteFile(path string, data []byte, perm os.FileMode) {
	sandbox.t.Helper()
	if err := sandbox.WriteFile(path, data, perm); err != nil {
		sandbox.t.Fatalf("MustWriteFile %s failed: %v", path, err)
	}
}

func (sandbox *Sandbox) Mkdir(rel string, all bool) error {
	sandbox.t.Helper()
	return sandbox.rt.Mkdir(rel, 0o755, all)
}

// ResolvePath returns an absolute runtime path with optional symlink resolution.
func (sandbox *Sandbox) ResolvePath(rel string) (string, error) {
	sandbox.t.Helper()
	return sandbox.rt.ResolvePath(rel, false)
}

func (sandbox *Sandbox) cleanup() {}

func (sandbox *Sandbox) runtimeEnv() toolkit.Env {
	sandbox.t.Helper()
	if sandbox.rt == nil {
		sandbox.t.Fatalf("sandbox runtime env is nil")
	}
	return sandbox.rt
}

func (sandbox *Sandbox) testClock() *clock.TestClock {
	sandbox.t.Helper()
	if sandbox.rt != nil {
		if tc, ok := sandbox.rt.Clock().(*clock.TestClock); ok && tc != nil {
			return tc
		}
	}
	sandbox.t.Fatalf("sandbox test clock is not available")
	return nil
}

// DumpJailTree logs a tree of files and directories rooted at the sandbox jail.
func (sandbox *Sandbox) DumpJailTree(maxDepth int) {
	sandbox.t.Helper()
	if sandbox.GetJail() == "" {
		sandbox.t.Log("DumpJailTree: no jail set")
		return
	}

	sandbox.t.Logf("Jail tree: %s", sandbox.GetJail())

	type pathInfo struct {
		path  string
		isDir bool
		depth int
	}
	var paths []pathInfo
	hasDirChild := make(map[string]bool)

	err := filepath.WalkDir(sandbox.GetJail(), func(p string, d iofs.DirEntry, err error) error {
		if err != nil {
			sandbox.t.Logf("  error: %v", err)
			return nil
		}

		var path string
		if p == "." {
			path = "/"
		} else {
			runtimePath := toolkit.RemoveJailPrefix(sandbox.GetJail(), p)
			resolved, err := sandbox.rt.ResolvePath(runtimePath, false)
			if err != nil {
				sandbox.t.Logf("  resolve error for %s: %v", p, err)
				return nil
			}
			path = resolved
		}

		if maxDepth > 0 {
			depth := strings.Count(path, string(os.PathSeparator)) + 1
			if depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			depth := strings.Count(path, string(os.PathSeparator)) + 1
			paths = append(paths, pathInfo{path: path, isDir: true, depth: depth})
		} else {
			paths = append(paths, pathInfo{path: path, isDir: false})
			parent := filepath.Dir(path)
			hasDirChild[parent] = true
		}

		return nil
	})

	for _, pi := range paths {
		if !pi.isDir {
			sandbox.t.Logf("  %s", pi.path)
		} else if !hasDirChild[pi.path] {
			sandbox.t.Logf("  %s/", pi.path)
		}
	}

	if err != nil {
		sandbox.t.Logf("DumpJailTree walk error: %v", err)
	}
}

// DumpFileContent reads and logs the content of a file in the sandbox.
func (sandbox *Sandbox) DumpFileContent(rel string) {
	sandbox.t.Helper()
	content, err := sandbox.ReadFile(rel)
	if err != nil {
		sandbox.t.Logf("DumpFileContent %s failed: %v", rel, err)
		return
	}
	sandbox.t.Logf("File content: %s\n%s", rel, string(content))
}

// Advance advances the sandbox test clock by the given duration.
func (sandbox *Sandbox) Advance(d time.Duration) {
	sandbox.t.Helper()
	sandbox.testClock().Advance(d)
}

// Now returns the current time from the sandbox test clock.
func (sandbox *Sandbox) Now() time.Time {
	sandbox.t.Helper()
	return sandbox.testClock().Now()
}

// Getwd returns the sandbox working directory.
func (sandbox *Sandbox) Getwd() (string, error) {
	sandbox.t.Helper()
	return sandbox.rt.Getwd()
}

// Setwd sets the sandbox working directory.
func (sandbox *Sandbox) Setwd(dir string) error {
	sandbox.t.Helper()
	path, err := sandbox.ResolvePath(dir)
	if err != nil {
		return err
	}
	return sandbox.rt.Setwd(path)
}

// GetHome returns the sandbox home.
func (sandbox *Sandbox) GetHome() (string, error) {
	sandbox.t.Helper()
	return sandbox.runtimeEnv().GetHome()
}

// copyEmbedDir recursively copies a directory tree from an embedded FS to dst.
func copyEmbedDir(fsys embed.FS, src, dst string) error {
	entries, err := iofs.ReadDir(fsys, src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyEmbedDir(fsys, s, d); err != nil {
				return err
			}
			continue
		}
		data, err := fsys.ReadFile(s)
		if err != nil {
			return err
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
