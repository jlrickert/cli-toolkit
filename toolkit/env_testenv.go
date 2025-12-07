package toolkit

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// TestEnv is an in-memory Env implementation useful for tests. It does not
// touch the real process environment and therefore makes tests hermetic.
//
// The home and user fields satisfy GetHome and GetUser. The data map stores
// other keys. For convenience, setting or unsetting the keys "HOME" and
// "USER" updates the corresponding home and user fields.
type TestEnv struct {
	jail string
	home string // home is an absolute path. Doesn't include the jail
	user string
	data map[string]string
}

func (o *TestEnv) Name() string {
	return "test-env"
}

// NewTestEnv constructs a TestEnv populated with sensible defaults for tests.
// It sets HOME and USER and also sets platform specific variables so functions
// that prefer XDG_* on Unix or APPDATA/LOCALAPPDATA on Windows will pick them
// up.
//
// If home or username are empty, reasonable defaults are chosen:
//   - home defaults to EnsureInJailFor(jail, "/home/<username>")
//   - username defaults to "testuser"
//
// The function does not create directories on disk. It only sets environment
// values in the returned TestEnv.
func NewTestEnv(jail, home, username string) *TestEnv {
	cwd := "/"
	user := username
	if user == "" {
		user = "testuser"
	}

	if home == "" && user == "root" {
		home = filepath.Join("/", ".root")
		cwd = "/"
	} else if home == "" {
		home = filepath.Join("/", "home", user)
		cwd = home
	} else {
		cwd = home
	}

	m := &TestEnv{
		jail: jail,
		home: home,
		user: username,
		data: make(map[string]string),
	}

	// Always expose HOME and USER through the map as well for callers that read
	// via Get.
	m.data["HOME"] = home
	m.data["USER"] = username
	m.data["PWD"] = cwd

	// Populate platform specific defaults so callers that query these keys get
	// consistent results in tests.
	if runtime.GOOS == "windows" {
		// Windows conventions: APPDATA (Roaming) and LOCALAPPDATA (Local).
		appdata := filepath.Join(home, "AppData", "Roaming")
		local := filepath.Join(home, "AppData", "Local")
		m.data["APPDATA"] = appdata
		m.data["LOCALAPPDATA"] = local
		m.data["TMPDIR"] = filepath.Join(local, "Temp")
	} else {
		// Unix like conventions: XDG_* fallbacks under the home directory.
		xdgConfig := filepath.Join(home, ".config")
		xdgCache := filepath.Join(home, ".cache")
		xdgData := filepath.Join(home, ".local", "share")
		xdgState := filepath.Join(home, ".local", "state")
		m.data["XDG_CONFIG_HOME"] = xdgConfig
		m.data["XDG_CACHE_HOME"] = xdgCache
		m.data["XDG_DATA_HOME"] = xdgData
		m.data["XDG_STATE_HOME"] = xdgState
		m.data["TMPDIR"] = filepath.Join(jail, "tmp")
	}

	return m
}

func (m *TestEnv) GetJail() string {
	return m.jail
}

// GetHome returns the configured home directory or an error if it is not set.
//
// For TestEnv the returned home is guaranteed to be located within the
// configured jail when possible. This helps keep tests hermetic by ensuring
// paths used for home are under the test temporary area.
func (m *TestEnv) GetHome() (string, error) {
	if m.home == "" {
		return "", errors.New("home not set in TestEnv")
	}
	return RemoveJailPrefix(m.jail, m.home), nil
}

// SetHome sets the TestEnv home directory and updates the "HOME" key in the
// underlying map for callers that read via Get.
func (m *TestEnv) SetHome(rel string) error {
	path, err := m.ResolvePath(rel, false)
	if err != nil {
		return fmt.Errorf("unable to set home: %w", err)
	}
	home := filepath.Join(m.jail, path)
	m.home = home
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data["HOME"] = home
	return nil
}

// GetUser returns the configured username or an error if it is not set.
func (m *TestEnv) GetUser() (string, error) {
	if m.user == "" {
		return "", errors.New("user not set in TestEnv")
	}
	return m.user, nil
}

// SetUser sets the TestEnv current user and updates the "USER" key in the
// underlying map for callers that use Get.
func (m *TestEnv) SetUser(username string) error {
	m.user = username
	if m.data == nil {
		m.data = make(map[string]string)
	}
	m.data["USER"] = username
	return nil
}

// Get returns the stored value for key. Reading from a nil map returns the
// zero value, so this method is safe on a zero TestEnv. The special keys HOME
// and USER come from dedicated fields.
func (m TestEnv) Get(key string) string {
	switch key {
	case "HOME":
		return m.home
	case "USER":
		return m.user
	default:
		return m.data[key]
	}
}

// Set stores a key/value pair in the TestEnv. If key is "HOME" or "USER" the
// corresponding dedicated field is updated. Calling Set on a nil receiver
// returns an error rather than panicking.
func (m *TestEnv) Set(key string, value string) error {
	switch key {
	case "HOME":
		return m.SetHome(value)
	case "USER":
		return m.SetUser(value)
	case "PWD":
		m.Setwd(value)
		return nil
	default:
		if m.data == nil {
			m.data = make(map[string]string)
		}
		m.data[key] = value
	}
	return nil
}

// Environ returns a slice of "KEY=VALUE" entries representing the environment
// stored in the TestEnv. It guarantees HOME and USER are present when set.
func (m *TestEnv) Environ() []string {
	// Collect keys from the backing map and ensure HOME/USER are present
	// based on dedicated fields so callers get a complete view.
	keys := make([]string, 0, len(m.data)+2)
	seen := make(map[string]struct{}, len(m.data)+2)
	for k := range m.data {
		keys = append(keys, k)
		seen[k] = struct{}{}
	}
	if m.home != "" {
		if _, ok := seen["HOME"]; !ok {
			keys = append(keys, "HOME")
		}
	}
	if m.user != "" {
		if _, ok := seen["USER"]; !ok {
			keys = append(keys, "USER")
		}
	}

	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		var v string
		switch k {
		case "HOME":
			v = m.home
		case "USER":
			v = m.user
		default:
			v = m.data[k]
		}
		out = append(out, k+"="+v)
	}
	return out
}

// Has reports whether the given key is present in the TestEnv map.
func (m *TestEnv) Has(key string) bool {
	_, ok := m.data[key]
	return ok
}

// Unset removes a key from the TestEnv. If key is "HOME" or "USER" the
// corresponding field is cleared. Calling Unset on a nil receiver is a no-op.
func (m *TestEnv) Unset(key string) {
	switch key {
	case "HOME":
		m.home = ""
		if m.data != nil {
			delete(m.data, "HOME")
		}
	case "USER":
		m.user = ""
		if m.data != nil {
			delete(m.data, "USER")
		}
	default:
		if m.data != nil {
			delete(m.data, key)
		}
	}
}

// GetTempDir returns a temp directory appropriate for the TestEnv. If the
// receiver is nil this falls back to os.TempDir to avoid panics.
//
// The method prefers explicit TMPDIR/TEMP/TMP values stored in the TestEnv.
// On Windows it applies a series of fallbacks: LOCALAPPDATA, APPDATA,
// USERPROFILE, then a home based default. On Unix like systems it falls back
// to /tmp.
//
// The returned path will be adjusted to reside inside the configured jail
// when possible to keep test artifacts contained.
func (m *TestEnv) GetTempDir() string {
	// Prefer explicit TMPDIR/TEMP/TMP if provided in the TestEnv.
	if d := m.data["TMPDIR"]; d != "" {
		return d
	}
	if d := m.data["TEMP"]; d != "" {
		return d
	}
	if d := m.data["TMP"]; d != "" {
		return d
	}

	// Platform specific sensible defaults without consulting the real process env.
	if runtime.GOOS == "windows" {
		// Prefer LOCALAPPDATA, then APPDATA, then USERPROFILE, then a home based
		// default.
		if local := m.data["LOCALAPPDATA"]; local != "" {
			return filepath.Join(local, "Temp")
		}
		if app := m.data["APPDATA"]; app != "" {
			return filepath.Join(app, "Temp")
		}
		if up := m.data["USERPROFILE"]; up != "" {
			return filepath.Join(up, "Temp")
		}
		if m.home != "" {
			return filepath.Join(m.home, "AppData", "Local", "Temp")
		}
		// No information available in TestEnv; return empty string to indicate
		// unknown.
		return ""
	}

	// Unix like: fall back to /tmp which is the conventional system temp dir.
	return filepath.Join("/", "tmp")
}

// Getwd returns the TestEnv's PWD value if set, otherwise an error.
func (m *TestEnv) Getwd() (string, error) {
	if m.data != nil {
		if wd := m.data["PWD"]; wd != "" {
			return wd, nil
		}
	}
	return "", errors.New("working directory not set in TestEnv")
}

// Setwd sets the TestEnv's PWD value to the provided directory.
func (m *TestEnv) Setwd(dir string) {
	if m.data == nil {
		m.data = make(map[string]string)
	}
	path, _ := m.ResolvePath(dir, false)
	m.data["PWD"] = path
}

// ReadFile reads the named file from the filesystem view held by this TestEnv.
// When the receiver is nil the real filesystem is used.
func (m *TestEnv) ReadFile(rel string) ([]byte, error) {
	resolved, err := m.ResolvePath(rel, false)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(m.jail, resolved)
	if !IsInJail(m.jail, path) {
		return nil, fmt.Errorf("ReadFile outside of jail %s: %w", path, ErrEscapeAttempt)
	}
	return os.ReadFile(path)
}

// Remove removes the named file or directory. If all is true RemoveAll is used.
// When the receiver is nil the real filesystem is affected.
func (m *TestEnv) Remove(rel string, all bool) error {
	path, err := m.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	path = filepath.Join(m.jail, path)
	if !IsInJail(m.jail, path) {
		return fmt.Errorf("Remove outside of jail %s: %w", path, ErrEscapeAttempt)
	}
	if all {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

// Rename renames (moves) a file or directory. When the receiver is nil the
// operation is performed on the real filesystem.
func (m *TestEnv) Rename(src string, dst string) error {
	a, err := m.ResolvePath(src, false)
	if err != nil {
		return err
	}
	a = filepath.Join(m.jail, a)
	if !IsInJail(m.jail, a) {
		return fmt.Errorf("Rename src outside of jail %s: %w", a, ErrEscapeAttempt)
	}
	b, err := m.ResolvePath(dst, false)
	if err != nil {
		return err
	}
	b = filepath.Join(m.jail, b)
	if !IsInJail(m.jail, b) {
		return fmt.Errorf("Rename dst outside of jail %s, %w", b, ErrEscapeAttempt)
	}
	return os.Rename(a, b)
}

// Mkdir creates a directory. If all is true MkdirAll is used.
func (m *TestEnv) Mkdir(rel string, perm os.FileMode, all bool) error {
	resolved, err := m.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	p := filepath.Join(m.jail, resolved)
	if !IsInJail(m.jail, p) {
		return fmt.Errorf("Mkdir outside of jail: %s, %w", p, ErrEscapeAttempt)
	}
	if all {
		return os.MkdirAll(p, perm)
	}
	return os.Mkdir(p, perm)
}

// WriteFile writes data to a file in the filesystem view held by this TestEnv.
func (m *TestEnv) WriteFile(name string, data []byte, perm os.FileMode) error {
	p, err := m.ResolvePath(name, false)
	if err != nil {
		return err
	}
	path := filepath.Join(m.jail, p)
	if !IsInJail(m.jail, path) {
		return fmt.Errorf("WriteFile outside of jail %s: %w", path, ErrEscapeAttempt)
	}
	return os.WriteFile(path, data, perm)
}

// ReadDir implements FileSystem.
func (m *TestEnv) ReadDir(name string) ([]os.DirEntry, error) {
	p, err := m.ResolvePath(name, false)
	if err != nil {
		return nil, err
	}
	p = filepath.Join(m.jail, p)
	if !IsInJail(m.jail, p) {
		return nil, fmt.Errorf("ReadDir outside of jail %s: %w", p, ErrEscapeAttempt)
	}
	return os.ReadDir(p)
}

// Stat implements FileSystem.
func (m *TestEnv) Stat(rel string, followSymlinks bool) (os.FileInfo, error) {
	path, err := m.ResolvePath(rel, followSymlinks)
	if err != nil {
		return nil, err
	}
	path = filepath.Join(m.jail, path)
	if !IsInJail(m.jail, path) {
		return nil, fmt.Errorf("Stat outside of jail %s: %w", path, ErrEscapeAttempt)
	}
	return os.Stat(path)
}

// ExpandPath expands a leading tilde in the provided path to the TestEnv home.
// Supported forms:
//
//	"~"
//	"~/rest/of/path"
//	"~\rest\of\path" (Windows)
//
// If the path does not start with a tilde it is returned unchanged. This method
// uses the TestEnv GetHome value. If home is not set, expansion may produce
// an empty or unexpected result.
func (m *TestEnv) ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if p[0] != '~' {
		return p
	}

	// Only expand the simple leading tilde forms: "~" or "~/" or "~\"
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := m.GetHome()
		if p == "~" {
			return filepath.Clean(home)
		}
		// Trim the "~/" or "~\" prefix and join with home to produce a
		// well formed path.
		rest := p[2:]
		return filepath.Join(home, rest)
	}

	// More complex cases like "~username/..." are not supported and are
	// returned unchanged.
	return p
}

func (m *TestEnv) ResolvePath(rel string, follow bool) (string, error) {
	p := filepath.Clean(rel)
	if p == "." || p == "" {
		return m.Getwd()
	}

	// Expand the path (handles ~ and env vars)
	expanded := m.ExpandPath(rel)

	var path string
	if filepath.IsAbs(expanded) {
		path = expanded
	} else {
		wd, err := m.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(wd, expanded)
	}

	if !follow {
		return filepath.Clean(path), nil
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(m.jail, path))
	if err != nil {
		return "", err
	}
	return RemoveJailPrefix(m.jail, resolved), nil
}

// Clone returns a copy of the TestEnv so tests can modify the returned
// environment without mutating the original. It deep copies the internal map
// and makes a copy of the Stream struct.
func (m *TestEnv) Clone() *TestEnv {
	if m == nil {
		return nil
	}

	var dataCopy map[string]string
	if m.data != nil {
		dataCopy = make(map[string]string, len(m.data))
		maps.Copy(dataCopy, m.data)
	}

	return &TestEnv{
		home: m.home,
		user: m.user,
		data: dataCopy,
	}
}

func (o *TestEnv) Symlink(oldname string, newname string) error {
	oldPath := o.ExpandPath(oldname)
	newPath := o.ExpandPath(newname)
	return os.Symlink(oldPath, newPath)
}

func (m *TestEnv) Glob(pattern string) ([]string, error) {
	// Resolve the pattern relative to jail
	expanded := m.ExpandPath(pattern)
	var globPattern string
	if filepath.IsAbs(expanded) {
		globPattern = filepath.Join(m.jail, expanded)
	} else {
		wd, err := m.Getwd()
		if err != nil {
			return nil, err
		}
		globPattern = filepath.Join(m.jail, wd, expanded)
	}

	// Perform glob on the jailed path
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, err
	}

	// Remove jail prefix from results and filter to ensure they're in jail
	var results []string
	wd, _ := m.Getwd()
	for _, match := range matches {
		if !IsInJail(m.jail, match) {
			continue
		}
		// Get the path relative to jail
		jailedPath := RemoveJailPrefix(m.jail, match)
		// If the original pattern was relative, return path relative to cwd
		if !filepath.IsAbs(expanded) {
			// Return path relative to the working directory
			relPath, err := filepath.Rel(wd, jailedPath)
			if err == nil {
				results = append(results, relPath)
			} else {
				results = append(results, jailedPath)
			}
		} else {
			results = append(results, jailedPath)
		}
	}
	return results, nil
}

func (m *TestEnv) AtomicWriteFile(rel string, data []byte, perm os.FileMode) error {
	resolved, err := m.ResolvePath(rel, false)
	if err != nil {
		return err
	}
	path := filepath.Join(m.jail, resolved)
	if !IsInJail(m.jail, path) {
		return fmt.Errorf("AtomicWriteFile outside of jail %s: %w", path, ErrEscapeAttempt)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomic write: mkdirall %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp("", ".tmp-"+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("atomic write: create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("atomic write: write temp file %q: %w", tmpName, err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("atomic write: close temp file %q: %w", tmpName, err)
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		// Not fatal: continue anyway
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("atomic write: rename %q -> %q: %w", tmpName, path, err)
	}

	return nil
}

// Ensure implementations satisfy the interfaces.
var _ Env = (*TestEnv)(nil)
var _ FileSystem = (*TestEnv)(nil)
