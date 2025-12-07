package toolkit

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
)

// FileSystem defines the contract for filesystem operations.
// Implementations may reflect the real filesystem (OsEnv) or provide
// an in-memory view suitable for tests (TestEnv).
type FileSystem interface {
	// ReadFile reads the named file.
	ReadFile(rel string) ([]byte, error)

	// WriteFile writes data to a file with the given permissions.
	WriteFile(rel string, data []byte, perm os.FileMode) error

	// Mkdir creates a directory. If all is true, MkdirAll is used.
	Mkdir(rel string, perm os.FileMode, all bool) error

	// Remove removes the named file or directory. If all is true, RemoveAll is used.
	Remove(rel string, all bool) error

	// Rename renames (moves) a file or directory.
	Rename(src, dst string) error

	// Stat returns the os.FileInfo for the named file.
	Stat(name string, followSymlinks bool) (os.FileInfo, error)

	// ReadDir reads the directory and returns a list of entries.
	ReadDir(rel string) ([]os.DirEntry, error)

	Symlink(oldname, newname string) error

	Glob(pattern string) ([]string, error)

	AtomicWriteFile(rel string, data []byte, perm os.FileMode) error
}

func AtomicWriteFile(ctx context.Context, rel string, data []byte, perm os.FileMode) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)

	err := env.AtomicWriteFile(rel, data, perm)
	if err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"AtomicWriteFile failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return err
	}
	lg.Log(
		ctx,
		slog.LevelDebug,
		"AtomicWriteFile succeed",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("rel", rel),
	)

	return nil
}

// AbsPath returns a cleaned absolute path for the provided path. Behavior:
// - If path is empty, returns empty string.
// - Expands a leading tilde using ExpandPath with the Env from ctx.
// - Expands environment variables from the injected env in ctx.
// - If the path is not absolute, attempts to convert it to an absolute path.
// - Returns a cleaned path in all cases.
//
// If ExpandPath fails (for example when HOME is not available) AbsPath falls
// back to the original input and proceeds with expansion of environment
// variables and cleaning.
func AbsPath(ctx context.Context, rel string) string {
	if rel == "" {
		return ""
	}

	// Expand leading tilde, if present.
	p, err := ExpandPath(ctx, rel)
	if err != nil {
		// If expansion fails (for example: no HOME), fall back to the original
		// input.
		p = rel
	}

	// If the path is already absolute, just clean and return it.
	if filepath.IsAbs(p) {
		res := filepath.Clean(p)
		return res
	}

	// If a PWD is provided by the injected Env, use it as the base for relative
	// paths. Otherwise fall back to filepath.Abs which uses the process working
	// directory.
	env := EnvFromContext(ctx)
	if cwd, err := env.Getwd(); err == nil && cwd != "" {
		// Ensure cwd is absolute.
		if !filepath.IsAbs(cwd) {
			if absCwd, err := filepath.Abs(cwd); err == nil {
				cwd = absCwd
			}
		}
		joined := filepath.Join(cwd, p)
		res := filepath.Clean(joined)
		return res
	}

	// Fall back to using the process working directory.
	if abs, err := filepath.Abs(p); err == nil {
		res := filepath.Clean(abs)
		return res
	}

	// As a last resort, return the cleaned original.
	res := filepath.Clean(p)
	return res
}

// ResolvePath returns the absolute path with symlinks evaluated. If symlink
// evaluation fails the absolute path returned by AbsPath is returned instead.
func ResolvePath(ctx context.Context, rel string, follow bool) (string, error) {
	env := EnvFromContext(ctx)

	return env.ResolvePath(rel, follow)
}

// RelativePath returns a path relative to basepath. If path is empty an
// empty string is returned. If computing the relative path fails the
// absolute target path is returned.
func RelativePath(ctx context.Context, basepath, path string) string {
	base := AbsPath(ctx, basepath)
	target := AbsPath(ctx, path)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		// Unrelated paths should return the absolute path
		return target
	}
	return rel
}

// ReadFile reads the named file using the Env stored in ctx. This ensures the
// filesystem view can be controlled by an injected TestEnv.
func ReadFile(ctx context.Context, rel string) ([]byte, error) {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)

	b, err := env.ReadFile(rel)
	if err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"ReadFile failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return nil, err
	}

	lg.Log(ctx, slog.LevelDebug,
		"ReadFile succeed",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("rel", rel),
	)
	return b, err
}

// WriteFile writes data to a file using the Env stored in ctx. The perm
// argument is the file mode to apply to the written file.
func WriteFile(ctx context.Context, rel string, data []byte, perm os.FileMode) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)
	path, err := ExpandPath(ctx, rel)
	if err != nil {
		return err
	}
	path = AbsPath(ctx, path)

	dir := filepath.Dir(path)
	err = Mkdir(ctx, dir, 0o755, true)
	if err != nil {
		return err
	}

	err = env.WriteFile(path, data, perm)
	if err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"WriteFile failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.String("path", path),
			slog.Any("error", err),
		)
		return err
	}
	lg.Log(
		ctx,
		slog.LevelDebug,
		"WriteFile succeed",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("rel", rel),
		slog.String("path", path),
		slog.Any("error", err),
	)
	return err
}

// Mkdir creates a directory using the Env stored in ctx. If all is true
// MkdirAll is used.
func Mkdir(ctx context.Context, rel string, perm os.FileMode, all bool) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)
	if err := env.Mkdir(rel, perm, all); err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"Mkdir failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.Bool("all", all),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return err
	}
	lg.Log(
		ctx,
		slog.LevelDebug,
		"Mkdir success",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.Bool("all", all),
		slog.String("rel", rel),
	)

	return nil
}

// Remove removes the named file or directory using the Env stored in ctx. If
// all is true RemoveAll is used.
func Remove(ctx context.Context, rel string, all bool) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)

	if err := env.Remove(rel, all); err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"Remove failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return err
	}

	lg.Log(
		ctx,
		slog.LevelDebug,
		"Remove success",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("rel", rel),
	)

	return nil
}

// Rename renames (moves) a file or directory using the Env stored in ctx.
func Rename(ctx context.Context, src, dst string) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)
	if err := env.Rename(src, dst); err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"Rename failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("src", src),
			slog.String("dst", dst),
			slog.Any("error", err),
		)
		return err
	}
	lg.Log(
		ctx,
		slog.LevelDebug,
		"Rename success",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("src", src),
		slog.String("dst", dst),
	)
	return nil
}

// Stat returns the os.FileInfo for the named file. The path is expanded using
// ExpandPath with the Env from ctx before calling os.Stat.
func Stat(ctx context.Context, rel string, followSymlinks bool) (os.FileInfo, error) {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)
	info, err := env.Stat(rel, followSymlinks)
	if err != nil {
		lg.Log(ctx, slog.LevelError, "Stat failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return nil, err
	}
	lg.Log(ctx, slog.LevelDebug, "Stat success",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("rel", rel),
	)
	return info, nil
}

// ReadDir reads the directory named by name and returns a list of entries. The
// path is expanded using ExpandPath with the Env from ctx before calling
// os.ReadDir.
func ReadDir(ctx context.Context, rel string) ([]os.DirEntry, error) {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)
	entries, err := env.ReadDir(rel)
	if err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"ReadDir failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Any("error", err),
		)
		return nil, err
	} else {
		lg.Log(
			ctx,
			slog.LevelDebug,
			"ReadDir success",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("rel", rel),
			slog.Int("count", len(entries)),
		)
	}

	return entries, nil
}

// Glob returns the names of all files matching the pattern using the Env stored
// in ctx. This enables glob patterns to work within sandboxed test environments.
// The pattern is expanded (handling ~) and then glob-matched. For TestEnv, results
// are restricted to within the jail boundary.
func Glob(ctx context.Context, pattern string) ([]string, error) {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)

	matches, err := env.Glob(pattern)
	if err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"Glob failed",
			slog.String("envType", env.Name()),
			slog.String("pattern", pattern),
			slog.Any("error", err),
		)
		return nil, err
	}

	lg.Log(
		ctx,
		slog.LevelDebug,
		"Glob success",
		slog.String("envType", env.Name()),
		slog.String("pattern", pattern),
		slog.Int("count", len(matches)),
	)
	return matches, nil
}

// Symlink creates a symbolic link pointing to oldname named newname.
// The oldname and newname are expanded using ExpandPath with the Env
// from ctx before creating the symlink. If symlink creation fails an
// error is returned.
func Symlink(ctx context.Context, oldname, newname string) error {
	env := EnvFromContext(ctx)
	lg := getTookitLogger(ctx)

	if err := env.Symlink(oldname, newname); err != nil {
		lg.Log(
			ctx,
			slog.LevelError,
			"Symlink failed",
			slog.String("envType", env.Name()),
			slog.String("pwd", env.Get("PWD")),
			slog.String("oldname", oldname),
			slog.String("newname", newname),
			slog.Any("error", err),
		)
		return err
	}

	lg.Log(
		ctx,
		slog.LevelDebug,
		"Symlink success",
		slog.String("envType", env.Name()),
		slog.String("pwd", env.Get("PWD")),
		slog.String("oldname", oldname),
		slog.String("newname", newname),
	)

	return nil
}
