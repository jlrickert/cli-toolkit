package toolkit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Package std provides helpers for locating user-specific directories
// (config, cache, data, state) in a cross-platform, testable way.

// ExpandPath expands a leading tilde in the provided path to the user's home
// directory obtained from env.
func ExpandPath(env Env, p string) (string, error) {
	if p == "" {
		return p, nil
	}
	if p[0] != '~' {
		return p, nil
	}

	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if env == nil {
			env = &OsEnv{}
		}
		home, err := env.GetHome()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return filepath.Clean(home), nil
		}
		rest := p[2:]
		return filepath.Join(home, rest), nil
	}

	return p, nil
}

// UserConfigPath returns the directory that should be used to store per-user
// configuration files.
func UserConfigPath(env Env) (string, error) {
	if env == nil {
		env = &OsEnv{}
	}
	if xdg := env.Get("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Clean(xdg), nil
	}
	if app := env.Get("APPDATA"); app != "" {
		return filepath.Clean(app), nil
	}
	home, err := env.GetHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// UserCachePath returns the directory that should be used to store per-user
// cache files.
func UserCachePath(env Env) (string, error) {
	if env == nil {
		env = &OsEnv{}
	}
	if xdg := env.Get("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Clean(xdg), nil
	}
	if local := env.Get("LOCALAPPDATA"); local != "" {
		return filepath.Clean(local), nil
	}
	home, err := env.GetHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// UserDataPath returns the directory that should be used to store per-user
// application data.
func UserDataPath(env Env) (string, error) {
	if env == nil {
		env = &OsEnv{}
	}
	if runtime.GOOS == "windows" {
		if localAppData := env.Get("LOCALAPPDATA"); localAppData != "" {
			return filepath.Clean(filepath.Join(localAppData, "data")), nil
		}
		return "", fmt.Errorf("LOCALAPPDATA environment variable not set: %w", ErrNoEnvKey)
	}
	if xdg := env.Get("XDG_DATA_HOME"); xdg != "" {
		return filepath.Clean(xdg), nil
	}
	home, err := env.GetHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// UserStatePath returns the directory that should be used to store per-user
// state files for an application.
func UserStatePath(env Env) (string, error) {
	if env == nil {
		env = &OsEnv{}
	}
	if runtime.GOOS == "windows" {
		if localAppData := env.Get("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "state"), nil
		}
		return "", fmt.Errorf("LOCALAPPDATA environment variable not set: %w", ErrNoEnvKey)
	}
	if xdg := env.Get("XDG_STATE_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := env.GetHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state"), nil
}

var DefaultEditor = "nano"

// Edit launches the user's editor to edit the provided file path.
func Edit(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("empty filepath")
	}

	editor := os.Getenv("VISUAL")
	if strings.TrimSpace(editor) == "" {
		editor = os.Getenv("EDITOR")
	}
	if strings.TrimSpace(editor) == "" {
		editor = DefaultEditor
	}

	parts := strings.Fields(editor)
	name := parts[0]
	args := append(parts[1:], path)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running editor %q: %w", editor, err)
	}
	return nil
}
