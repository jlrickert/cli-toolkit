package env

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/jlrickert/cli-toolkit/toolkit/jail"
)

// TestEnv is an in-memory Env implementation useful for tests. It does not
// touch the real process environment and therefore makes tests hermetic.
//
// The home and user fields satisfy GetHome and GetUser. The data map stores
// other keys. For convenience, setting or unsetting the keys "HOME" and
// "USER" updates the corresponding home and user fields.
type TestEnv struct {
	jail string
	home string // home is an absolute path. Doesn't include the jail.
	user string
	data map[string]string
}

func (env *TestEnv) Name() string {
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
func NewTestEnv(jailPath, home, username string) *TestEnv {
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
		jail: jailPath,
		home: home,
		user: user,
		data: make(map[string]string),
	}

	// Always expose HOME and USER through the map as well for callers that read
	// via Get.
	m.data["HOME"] = home
	m.data["USER"] = user
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
		// Unix-like conventions: XDG_* fallbacks under the home directory.
		xdgConfig := filepath.Join(home, ".config")
		xdgCache := filepath.Join(home, ".cache")
		xdgData := filepath.Join(home, ".local", "share")
		xdgState := filepath.Join(home, ".local", "state")
		m.data["XDG_CONFIG_HOME"] = xdgConfig
		m.data["XDG_CACHE_HOME"] = xdgCache
		m.data["XDG_DATA_HOME"] = xdgData
		m.data["XDG_STATE_HOME"] = xdgState
		m.data["TMPDIR"] = filepath.Join(jailPath, "tmp")
	}

	return m
}

func (env *TestEnv) GetJail() string {
	return env.jail
}

func (env *TestEnv) SetJail(jailPath string) error {
	newJail := cleanJail(jailPath)
	oldJail := env.jail
	env.jail = newJail

	if env.data == nil {
		env.data = make(map[string]string)
	}

	if env.home != "" {
		virtualHome := jail.RemoveJailPrefix(oldJail, env.home)
		if newJail == "" {
			env.home = filepath.Clean(virtualHome)
		} else {
			env.home = filepath.Join(newJail, virtualHome)
		}
		env.data["HOME"] = env.home
	}

	if tmp := env.data["TMPDIR"]; tmp != "" && oldJail != "" && jail.IsInJail(oldJail, tmp) {
		virtualTmp := jail.RemoveJailPrefix(oldJail, tmp)
		if newJail == "" {
			env.data["TMPDIR"] = filepath.Clean(virtualTmp)
		} else {
			env.data["TMPDIR"] = filepath.Join(newJail, virtualTmp)
		}
	}

	return nil
}

// GetHome returns the configured home directory or an error if it is not set.
//
// For TestEnv the returned home is guaranteed to be located within the
// configured jail when possible. This helps keep tests hermetic by ensuring
// paths used for home are under the test temporary area.
func (env *TestEnv) GetHome() (string, error) {
	if env.home == "" {
		return "", errors.New("home not set in TestEnv")
	}
	return jail.RemoveJailPrefix(env.jail, env.home), nil
}

// SetHome sets the TestEnv home directory and updates the "HOME" key in the
// underlying map for callers that read via Get.
func (env *TestEnv) SetHome(rel string) error {
	path, err := env.ResolvePath(rel, false)
	if err != nil {
		return fmt.Errorf("unable to set home: %w", err)
	}
	home := filepath.Join(env.jail, path)
	env.home = home
	if env.data == nil {
		env.data = make(map[string]string)
	}
	env.data["HOME"] = home
	return nil
}

// GetUser returns the configured username or an error if it is not set.
func (env *TestEnv) GetUser() (string, error) {
	if env.user == "" {
		return "", errors.New("user not set in TestEnv")
	}
	return env.user, nil
}

// SetUser sets the TestEnv current user and updates the "USER" key in the
// underlying map for callers that use Get.
func (env *TestEnv) SetUser(username string) error {
	env.user = username
	if env.data == nil {
		env.data = make(map[string]string)
	}
	env.data["USER"] = username
	return nil
}

// Get returns the stored value for key. Reading from a nil map returns the
// zero value, so this method is safe on a zero TestEnv. The special keys HOME
// and USER come from dedicated fields.
func (env TestEnv) Get(key string) string {
	switch key {
	case "HOME":
		return env.home
	case "USER":
		return env.user
	default:
		return env.data[key]
	}
}

// Set stores a key/value pair in the TestEnv. If key is "HOME" or "USER" the
// corresponding dedicated field is updated. Calling Set on a nil receiver
// returns an error rather than panicking.
func (env *TestEnv) Set(key string, value string) error {
	switch key {
	case "HOME":
		return env.SetHome(value)
	case "USER":
		return env.SetUser(value)
	case "PWD":
		return env.Setwd(value)
	default:
		if env.data == nil {
			env.data = make(map[string]string)
		}
		env.data[key] = value
	}
	return nil
}

// Environ returns a slice of "KEY=VALUE" entries representing the environment
// stored in the TestEnv. It guarantees HOME and USER are present when set.
func (env *TestEnv) Environ() []string {
	// Collect keys from the backing map and ensure HOME/USER are present
	// based on dedicated fields so callers get a complete view.
	keys := make([]string, 0, len(env.data)+2)
	seen := make(map[string]struct{}, len(env.data)+2)
	for k := range env.data {
		keys = append(keys, k)
		seen[k] = struct{}{}
	}
	if env.home != "" {
		if _, ok := seen["HOME"]; !ok {
			keys = append(keys, "HOME")
		}
	}
	if env.user != "" {
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
			v = env.home
		case "USER":
			v = env.user
		default:
			v = env.data[k]
		}
		out = append(out, k+"="+v)
	}
	return out
}

// Has reports whether the given key is present in the TestEnv map.
func (env *TestEnv) Has(key string) bool {
	_, ok := env.data[key]
	return ok
}

// Unset removes a key from the TestEnv. If key is "HOME" or "USER" the
// corresponding field is cleared. Calling Unset on a nil receiver is a no-op.
func (env *TestEnv) Unset(key string) {
	switch key {
	case "HOME":
		env.home = ""
		if env.data != nil {
			delete(env.data, "HOME")
		}
	case "USER":
		env.user = ""
		if env.data != nil {
			delete(env.data, "USER")
		}
	default:
		if env.data != nil {
			delete(env.data, key)
		}
	}
}

// GetTempDir returns a temp directory appropriate for the TestEnv. If the
// receiver is nil this falls back to os.TempDir to avoid panics.
//
// The method prefers explicit TMPDIR/TEMP/TMP values stored in the TestEnv.
// On Windows it applies a series of fallbacks: LOCALAPPDATA, APPDATA,
// USERPROFILE, then a home-based default. On Unix-like systems it falls back
// to /tmp.
//
// The returned path will be adjusted to reside inside the configured jail
// when possible to keep test artifacts contained.
func (env *TestEnv) GetTempDir() string {
	// Prefer explicit TMPDIR/TEMP/TMP if provided in the TestEnv.
	if d := env.data["TMPDIR"]; d != "" {
		return d
	}
	if d := env.data["TEMP"]; d != "" {
		return d
	}
	if d := env.data["TMP"]; d != "" {
		return d
	}

	// Platform-specific sensible defaults without consulting the real process env.
	if runtime.GOOS == "windows" {
		// Prefer LOCALAPPDATA, then APPDATA, then USERPROFILE, then a home-based
		// default.
		if local := env.data["LOCALAPPDATA"]; local != "" {
			return filepath.Join(local, "Temp")
		}
		if app := env.data["APPDATA"]; app != "" {
			return filepath.Join(app, "Temp")
		}
		if up := env.data["USERPROFILE"]; up != "" {
			return filepath.Join(up, "Temp")
		}
		if env.home != "" {
			return filepath.Join(env.home, "AppData", "Local", "Temp")
		}
		// No information available in TestEnv; return empty string to indicate
		// unknown.
		return ""
	}

	// Unix-like: fall back to /tmp which is the conventional system temp dir.
	return filepath.Join("/", "tmp")
}

// Getwd returns the TestEnv's PWD value if set, otherwise an error.
func (env *TestEnv) Getwd() (string, error) {
	if env.data != nil {
		if wd := env.data["PWD"]; wd != "" {
			return wd, nil
		}
	}
	return "", errors.New("working directory not set in TestEnv")
}

// Setwd sets the TestEnv's PWD value to the provided directory.
func (env *TestEnv) Setwd(dir string) error {
	if env.data == nil {
		env.data = make(map[string]string)
	}
	path, err := env.ResolvePath(dir, false)
	if err != nil {
		return err
	}
	env.data["PWD"] = path
	return nil
}

// ExpandPath expands a leading tilde in the provided path to the TestEnv home.
// Supported forms:
//
//	"~"
//	"~/rest/of/path"
//	"~\\rest\\of\\path" (Windows)
//
// If the path does not start with a tilde it is returned unchanged. This method
// uses the TestEnv GetHome value. If home is not set, expansion may produce
// an empty or unexpected result.
func (env *TestEnv) ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if p[0] != '~' {
		return p
	}

	// Only expand the simple leading tilde forms: "~" or "~/" or "~\\".
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, _ := env.GetHome()
		if p == "~" {
			return filepath.Clean(home)
		}
		// Trim the "~/" or "~\\" prefix and join with home to produce a
		// well-formed path.
		rest := p[2:]
		return filepath.Join(home, rest)
	}

	// More complex cases like "~username/..." are not supported and are
	// returned unchanged.
	return p
}

func (env *TestEnv) ResolvePath(rel string, follow bool) (string, error) {
	p := filepath.Clean(rel)
	if p == "." || p == "" {
		return env.Getwd()
	}

	// Expand the path (handles ~ and env vars).
	expanded := env.ExpandPath(rel)

	var path string
	if filepath.IsAbs(expanded) {
		path = expanded
	} else {
		wd, err := env.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(wd, expanded)
	}

	if !follow {
		return filepath.Clean(path), nil
	}

	resolved, err := filepath.EvalSymlinks(filepath.Join(env.jail, path))
	if err != nil {
		return "", err
	}
	return jail.RemoveJailPrefix(env.jail, resolved), nil
}

// Clone returns a copy of the TestEnv so tests can modify the returned
// environment without mutating the original. It deep-copies the internal map.
func (env *TestEnv) Clone() *TestEnv {
	if env == nil {
		return nil
	}

	var dataCopy map[string]string
	if env.data != nil {
		dataCopy = make(map[string]string, len(env.data))
		maps.Copy(dataCopy, env.data)
	}

	return &TestEnv{
		jail: env.jail,
		home: env.home,
		user: env.user,
		data: dataCopy,
	}
}

func (env *TestEnv) CloneEnv() Env {
	return env.Clone()
}

// Ensure implementations satisfy the interfaces.
var _ Env = (*TestEnv)(nil)
