// Package appctx provides application path resolution for repository-backed
// CLI applications.
//
// [AppPaths] holds the repository root and platform-scoped paths for config,
// data, state, and cache directories. [NewAppPaths] fills missing roots using
// the user's platform defaults (XDG on Unix, APPDATA on Windows).
// [NewGitAppPaths] auto-detects the git repository root via [FindGitRoot]
// which tries the git CLI first and falls back to upward filesystem scanning.
package appctx
