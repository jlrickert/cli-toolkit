package toolkit

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/jlrickert/cli-toolkit/mylog"
)

// Runtime is the explicit dependency container for commands and helpers.
//
// Context values are not used for mutable runtime dependencies; callers pass a
// Runtime directly.
type Runtime struct {
	env    Env
	fs     FileSystem
	clock  clock.Clock
	logger *slog.Logger
	stream *Stream
	hasher Hasher

	// jail and wd are canonical state managed by Runtime and applied to both
	// env and filesystem.
	jail string
	wd   string
}

// RuntimeOption mutates Runtime construction.
type RuntimeOption func(*Runtime) error

// NewRuntime constructs a Runtime with defaults and applies options.
func NewRuntime(opts ...RuntimeOption) (*Runtime, error) {
	rt := &Runtime{
		env:    &OsEnv{},
		fs:     &OsFS{},
		clock:  &clock.OsClock{},
		logger: mylog.NewDiscardLogger(),
		stream: DefaultStream(),
		hasher: DefaultHasher,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(rt); err != nil {
			return nil, err
		}
	}

	if err := rt.normalizeState(); err != nil {
		return nil, err
	}

	if err := rt.Validate(); err != nil {
		return nil, err
	}

	return rt, nil
}

func WithRuntimeEnv(env Env) RuntimeOption {
	return func(rt *Runtime) error {
		if env == nil {
			return fmt.Errorf("runtime env cannot be nil")
		}
		rt.env = env
		return nil
	}
}

func WithRuntimeFileSystem(fs FileSystem) RuntimeOption {
	return func(rt *Runtime) error {
		if fs == nil {
			return fmt.Errorf("runtime filesystem cannot be nil")
		}
		rt.fs = fs
		return nil
	}
}

func WithRuntimeClock(c clock.Clock) RuntimeOption {
	return func(rt *Runtime) error {
		if c == nil {
			return fmt.Errorf("runtime clock cannot be nil")
		}
		rt.clock = c
		return nil
	}
}

func WithRuntimeLogger(lg *slog.Logger) RuntimeOption {
	return func(rt *Runtime) error {
		if lg == nil {
			return fmt.Errorf("runtime logger cannot be nil")
		}
		rt.logger = lg
		return nil
	}
}

func WithRuntimeStream(s *Stream) RuntimeOption {
	return func(rt *Runtime) error {
		if s == nil {
			return fmt.Errorf("runtime stream cannot be nil")
		}
		rt.stream = s
		return nil
	}
}

func WithRuntimeHasher(h Hasher) RuntimeOption {
	return func(rt *Runtime) error {
		if h == nil {
			return fmt.Errorf("runtime hasher cannot be nil")
		}
		rt.hasher = h
		return nil
	}
}

func WithRuntimeJail(jail string) RuntimeOption {
	return func(rt *Runtime) error {
		rt.jail = cleanJail(jail)
		return nil
	}
}

func cleanJail(jail string) string {
	if strings.TrimSpace(jail) == "" {
		return ""
	}
	return filepath.Clean(jail)
}

func normalizePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return string(filepath.Separator)
	}
	return filepath.Clean(path)
}

func (rt *Runtime) normalizeState() error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if rt.env == nil {
		return fmt.Errorf("runtime env is nil")
	}
	if rt.fs == nil {
		return fmt.Errorf("runtime filesystem is nil")
	}

	if rt.jail == "" {
		rt.jail = cleanJail(rt.env.GetJail())
		if rt.jail == "" {
			rt.jail = cleanJail(rt.fs.GetJail())
		}
	}

	if err := rt.fs.SetJail(rt.jail); err != nil {
		return err
	}
	if err := rt.env.SetJail(rt.jail); err != nil {
		return err
	}

	if strings.TrimSpace(rt.wd) == "" {
		if cwd, err := rt.env.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
			rt.wd = filepath.Clean(cwd)
		} else if cwd, err := rt.fs.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
			rt.wd = filepath.Clean(cwd)
		} else {
			rt.wd = string(filepath.Separator)
		}
	}

	return rt.applyWorkingDir(rt.wd)
}

func (rt *Runtime) applyWorkingDir(dir string) error {
	target := normalizePath(dir)
	if err := rt.fs.Setwd(target); err != nil {
		return err
	}
	if err := rt.env.Setwd(target); err != nil {
		return err
	}
	if wd, err := rt.fs.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		rt.wd = filepath.Clean(wd)
		return nil
	}
	rt.wd = target
	return nil
}

// Validate ensures required runtime dependencies are present.
func (rt *Runtime) Validate() error {
	if rt == nil {
		return fmt.Errorf("runtime is nil")
	}
	if rt.env == nil {
		return fmt.Errorf("runtime env is nil")
	}
	if rt.fs == nil {
		return fmt.Errorf("runtime filesystem is nil")
	}
	if rt.clock == nil {
		return fmt.Errorf("runtime clock is nil")
	}
	if rt.logger == nil {
		return fmt.Errorf("runtime logger is nil")
	}
	if rt.stream == nil {
		return fmt.Errorf("runtime stream is nil")
	}
	if rt.hasher == nil {
		return fmt.Errorf("runtime hasher is nil")
	}
	return nil
}

// Clone returns a shallow clone of runtime dependencies and deep-copies Env and
// Stream when supported.
func (rt *Runtime) Clone() *Runtime {
	if rt == nil {
		return nil
	}

	clone := *rt

	if rt.env != nil {
		if cloner, ok := rt.env.(EnvCloner); ok {
			clone.env = cloner.CloneEnv()
		}
	}

	if rt.stream != nil {
		streamCopy := *rt.stream
		clone.stream = &streamCopy
	}

	return &clone
}

// Env returns the runtime Env dependency.
func (rt *Runtime) Env() Env { return rt.env }

// FS returns the runtime FileSystem dependency.
func (rt *Runtime) FS() FileSystem { return rt.fs }

// Clock returns the runtime clock dependency.
func (rt *Runtime) Clock() clock.Clock { return rt.clock }

// SetClock updates the runtime clock dependency.
func (rt *Runtime) SetClock(c clock.Clock) error {
	if c == nil {
		return fmt.Errorf("runtime clock cannot be nil")
	}
	rt.clock = c
	return nil
}

// Logger returns the runtime logger dependency.
func (rt *Runtime) Logger() *slog.Logger { return rt.logger }

// SetLogger updates the runtime logger dependency.
func (rt *Runtime) SetLogger(lg *slog.Logger) error {
	if lg == nil {
		return fmt.Errorf("runtime logger cannot be nil")
	}
	rt.logger = lg
	return nil
}

// Stream returns the runtime stream dependency.
func (rt *Runtime) Stream() *Stream { return rt.stream }

// SetStream updates the runtime stream dependency.
func (rt *Runtime) SetStream(s *Stream) error {
	if s == nil {
		return fmt.Errorf("runtime stream cannot be nil")
	}
	rt.stream = s
	return nil
}

// Hasher returns the runtime hasher dependency.
func (rt *Runtime) Hasher() Hasher { return rt.hasher }

// SetHasher updates the runtime hasher dependency.
func (rt *Runtime) SetHasher(h Hasher) error {
	if h == nil {
		return fmt.Errorf("runtime hasher cannot be nil")
	}
	rt.hasher = h
	return nil
}

// --- Env forwarding methods ---

func (rt *Runtime) Name() string {
	if rt == nil || rt.env == nil {
		return "runtime"
	}
	return rt.env.Name()
}

func (rt *Runtime) Get(key string) string {
	if rt == nil || rt.env == nil {
		return ""
	}
	return rt.env.Get(key)
}

func (rt *Runtime) Set(key, value string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	if key == "PWD" {
		return rt.Setwd(value)
	}
	return rt.env.Set(key, value)
}

func (rt *Runtime) Has(key string) bool {
	if rt == nil || rt.env == nil {
		return false
	}
	return rt.env.Has(key)
}

func (rt *Runtime) Environ() []string {
	if rt == nil || rt.env == nil {
		return nil
	}
	return rt.env.Environ()
}

func (rt *Runtime) Unset(key string) {
	if rt == nil || rt.env == nil {
		return
	}
	rt.env.Unset(key)
}

func (rt *Runtime) GetHome() (string, error) {
	if err := rt.Validate(); err != nil {
		return "", err
	}
	return rt.env.GetHome()
}

func (rt *Runtime) SetHome(home string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	return rt.env.SetHome(home)
}

func (rt *Runtime) GetUser() (string, error) {
	if err := rt.Validate(); err != nil {
		return "", err
	}
	return rt.env.GetUser()
}

func (rt *Runtime) SetUser(user string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	return rt.env.SetUser(user)
}

func (rt *Runtime) GetTempDir() string {
	if rt == nil || rt.env == nil {
		return os.TempDir()
	}
	return rt.env.GetTempDir()
}

// GetJail returns the canonical runtime jail.
func (rt *Runtime) GetJail() string {
	if rt == nil {
		return ""
	}
	return rt.jail
}

// SetJail sets the canonical runtime jail and propagates it to both Env and FS.
func (rt *Runtime) SetJail(jail string) error {
	if err := rt.Validate(); err != nil {
		return err
	}

	rt.jail = cleanJail(jail)
	if err := rt.fs.SetJail(rt.jail); err != nil {
		return err
	}
	if err := rt.env.SetJail(rt.jail); err != nil {
		return err
	}

	if strings.TrimSpace(rt.wd) == "" {
		rt.wd = string(filepath.Separator)
	}
	if err := rt.applyWorkingDir(rt.wd); err != nil {
		fallback := string(filepath.Separator)
		if fallbackErr := rt.applyWorkingDir(fallback); fallbackErr != nil {
			return err
		}
	}
	return nil
}

// Getwd returns the canonical runtime working directory.
func (rt *Runtime) Getwd() (string, error) {
	if err := rt.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(rt.wd) == "" {
		if err := rt.normalizeState(); err != nil {
			return "", err
		}
	}
	return rt.wd, nil
}

func (rt *Runtime) resolveWorkingDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" || dir == "." {
		return rt.Getwd()
	}

	expanded := ExpandEnv(rt, dir)
	p, err := ExpandPath(rt, expanded)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(p) {
		cwd, err := rt.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(cwd, p)
	}

	resolved, err := rt.fs.ResolvePath(filepath.Clean(p), false)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// Setwd sets the canonical runtime working directory and propagates it to both
// Env and FileSystem.
func (rt *Runtime) Setwd(dir string) error {
	if err := rt.Validate(); err != nil {
		return err
	}

	resolved, err := rt.resolveWorkingDir(dir)
	if err != nil {
		return err
	}

	return rt.applyWorkingDir(resolved)
}

// --- FileSystem forwarding methods ---

// AbsPath returns a cleaned absolute runtime path based on runtime env/cwd.
func (rt *Runtime) AbsPath(rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", nil
	}
	if err := rt.Validate(); err != nil {
		return "", err
	}

	expanded := ExpandEnv(rt, rel)
	p, err := ExpandPath(rt, expanded)
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(p) {
		cwd, err := rt.Getwd()
		if err != nil {
			return "", err
		}
		p = filepath.Join(cwd, p)
	}

	return filepath.Clean(p), nil
}

// ResolvePath resolves rel to an absolute path and optionally follows symlinks.
func (rt *Runtime) ResolvePath(rel string, follow bool) (string, error) {
	if err := rt.Validate(); err != nil {
		return "", err
	}

	var p string
	if strings.TrimSpace(rel) == "" || rel == "." {
		cwd, err := rt.Getwd()
		if err != nil {
			return "", err
		}
		p = cwd
	} else {
		expanded := ExpandEnv(rt, rel)
		parsed, err := ExpandPath(rt, expanded)
		if err != nil {
			return "", err
		}
		p = parsed
		if !filepath.IsAbs(p) {
			cwd, err := rt.Getwd()
			if err != nil {
				return "", err
			}
			p = filepath.Join(cwd, p)
		}
	}

	return rt.fs.ResolvePath(filepath.Clean(p), follow)
}

// RelativePath returns path relative to basepath. If computation fails, target
// absolute path is returned.
func (rt *Runtime) RelativePath(basepath, path string) string {
	base, err := rt.AbsPath(basepath)
	if err != nil {
		base = basepath
	}
	target, err := rt.AbsPath(path)
	if err != nil {
		target = path
	}

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

func (rt *Runtime) ReadFile(rel string) ([]byte, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return nil, err
	}
	return rt.fs.ReadFile(path)
}

func (rt *Runtime) WriteFile(rel string, data []byte, perm os.FileMode) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	if err := rt.fs.Mkdir(filepath.Dir(path), 0o755, true); err != nil {
		return err
	}
	return rt.fs.WriteFile(path, data, perm)
}

func (rt *Runtime) Mkdir(rel string, perm os.FileMode, all bool) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.fs.Mkdir(path, perm, all)
}

func (rt *Runtime) Remove(rel string, all bool) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.fs.Remove(path, all)
}

func (rt *Runtime) Rename(src, dst string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	srcPath, err := rt.ResolvePath(src, false)
	if err != nil {
		return err
	}
	dstPath, err := rt.ResolvePath(dst, false)
	if err != nil {
		return err
	}
	return rt.fs.Rename(srcPath, dstPath)
}

func (rt *Runtime) Stat(rel string, follow bool) (os.FileInfo, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, follow)
	if err != nil {
		return nil, err
	}
	return rt.fs.Stat(path, follow)
}

func (rt *Runtime) ReadDir(rel string) ([]os.DirEntry, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return nil, err
	}
	return rt.fs.ReadDir(path)
}

func (rt *Runtime) Symlink(oldName, newName string) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	oldPath, err := rt.ResolvePath(oldName, false)
	if err != nil {
		return err
	}
	newPath, err := rt.ResolvePath(newName, false)
	if err != nil {
		return err
	}
	return rt.fs.Symlink(oldPath, newPath)
}

func (rt *Runtime) Glob(pattern string) ([]string, error) {
	if err := rt.Validate(); err != nil {
		return nil, err
	}

	expanded := ExpandEnv(rt, pattern)
	parsed, err := ExpandPath(rt, expanded)
	if err != nil {
		return nil, err
	}

	wd, err := rt.Getwd()
	if err != nil {
		return nil, err
	}

	resolvedPattern := parsed
	if !filepath.IsAbs(resolvedPattern) {
		resolvedPattern = filepath.Join(wd, resolvedPattern)
	}
	resolvedPattern = filepath.Clean(resolvedPattern)

	matches, err := rt.fs.Glob(resolvedPattern)
	if err != nil {
		return nil, err
	}

	if filepath.IsAbs(parsed) {
		return matches, nil
	}

	results := make([]string, 0, len(matches))
	for _, match := range matches {
		relPath, err := filepath.Rel(wd, match)
		if err == nil {
			results = append(results, relPath)
			continue
		}
		results = append(results, match)
	}
	return results, nil
}

func (rt *Runtime) AtomicWriteFile(rel string, data []byte, perm os.FileMode) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	path, err := rt.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	return rt.fs.AtomicWriteFile(path, data, perm)
}

func (rt *Runtime) Rel(basePath, targetPath string) (string, error) {
	if err := rt.Validate(); err != nil {
		return "", err
	}
	baseResolved, err := rt.ResolvePath(basePath, false)
	if err != nil {
		return "", err
	}
	targetResolved, err := rt.ResolvePath(targetPath, false)
	if err != nil {
		return "", err
	}
	return rt.fs.Rel(baseResolved, targetResolved)
}

var _ Env = (*Runtime)(nil)
var _ FileSystem = (*Runtime)(nil)
