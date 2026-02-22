package appctx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jlrickert/cli-toolkit/toolkit"
)

// AppPaths holds paths and configuration roots for a repository-backed app
// context. Root is the repository root. Other roots default to platform
// user-scoped locations when not provided.
type AppPaths struct {
	Appname string

	// Root is the path to the root of the context.
	Root string

	// configRoot is the base directory for user configuration files.
	ConfigRoot string

	// stateRoot holds transient state files for the context.
	StateRoot string

	// dataRoot is for programmatically managed data shipped with the program.
	DataRoot string

	// cacheRoot is for cache artifacts.
	CacheRoot string

	// localConfigRoot is the repo-local override location
	LocalConfigRoot string
}

func NewGitAppPaths(ctx context.Context, rt *toolkit.Runtime, appname string) (*AppPaths, error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime is nil")
	}
	cwd, err := rt.Getwd()
	if err != nil {
		return nil, err
	}
	root := FindGitRoot(ctx, rt, cwd)
	aCtx, err := NewAppPaths(rt, root, appname)
	return aCtx, err
}

// NewAppPaths constructs app paths and fills missing roots using platform
// defaults derived from the provided context.
//
// Behavior:
//   - If an option sets a value it is used as-is.
//   - If Root is not set it is inferred from Runtime.Getwd().
//   - ConfigRoot, DataRoot, StateRoot and CacheRoot use the corresponding
//     user-scoped platform paths and are joined with DefaultAppName.
func NewAppPaths(rt *toolkit.Runtime, root, appname string) (*AppPaths, error) {
	if rt == nil {
		return nil, fmt.Errorf("runtime is nil")
	}
	p := &AppPaths{Appname: appname}

	p.Root = filepath.Clean(root)

	if path, err := toolkit.UserConfigPath(rt); err != nil {
		return nil, fmt.Errorf(
			"unable to find user config path: %w",
			os.ErrNotExist,
		)
	} else {
		p.ConfigRoot = filepath.Join(path, p.Appname)
	}

	if path, err := toolkit.UserDataPath(rt); err != nil {
		return nil, fmt.Errorf(
			"unable to find user data path: %w",
			os.ErrNotExist,
		)
	} else {
		p.DataRoot = filepath.Join(path, p.Appname)
	}

	if path, err := toolkit.UserStatePath(rt); err != nil {
		return nil, fmt.Errorf(
			"unable to find user state root: %w",
			os.ErrNotExist,
		)
	} else {
		p.StateRoot = filepath.Join(path, p.Appname)
	}

	if path, err := toolkit.UserCachePath(rt); err != nil {
		return nil, fmt.Errorf(
			"unable to find user cache root: %w",
			os.ErrNotExist,
		)
	} else {
		p.CacheRoot = filepath.Join(path, p.Appname)
	}

	p.LocalConfigRoot = filepath.Join(p.Root, "."+appname)

	return p, nil
}
