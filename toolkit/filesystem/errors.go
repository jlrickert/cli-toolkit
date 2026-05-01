package filesystem

import "errors"

// ErrSubprocessNotSupported is the sentinel returned by FileSystem
// implementations that cannot launch host-level subprocesses for the
// caller. Phase 1 defines and exports this error but does not return it
// from any production codepath; Phase 2 of the runtime-aware subprocess
// work will wire it into a Command builder for non-host filesystems.
var ErrSubprocessNotSupported = errors.New("filesystem: subprocess execution not supported")
