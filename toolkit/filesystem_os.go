package toolkit

import filesystempkg "github.com/jlrickert/cli-toolkit/toolkit/filesystem"

// OsFS is retained for backward compatibility.
type OsFS = filesystempkg.OsFS

// NewOsFS is retained for backward compatibility.
func NewOsFS(jail, wd string) (*OsFS, error) {
	return filesystempkg.NewOsFS(jail, wd)
}
