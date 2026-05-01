package filesystem

import "errors"

// ErrSubprocessNotSupported is the sentinel returned by FileSystem
// implementations that cannot launch host-level subprocesses for the
// caller (e.g. an in-memory FS with no real host mapping). No
// production codepath returns it yet; it is exported for future
// consumers that need to distinguish "no host available" from other
// failure modes.
var ErrSubprocessNotSupported = errors.New("filesystem: subprocess execution not supported")
