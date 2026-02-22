package toolkit

import (
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/jlrickert/cli-toolkit/mylog"
)

// NewTestRuntime constructs a runtime configured for tests with in-memory env
// and a jailed filesystem. Callers may override defaults via runtime options.
func NewTestRuntime(jail, home, user string, opts ...RuntimeOption) (*Runtime, error) {
	env := NewTestEnv(jail, home, user)
	wd, err := env.Getwd()
	if err != nil {
		return nil, err
	}
	fs, err := NewOsFS(jail, wd)
	if err != nil {
		return nil, err
	}

	baseOpts := []RuntimeOption{
		WithRuntimeEnv(env),
		WithRuntimeFileSystem(fs),
		WithRuntimeClock(clock.NewTestClock(time.Date(2025, 10, 15, 12, 30, 0, 0, time.UTC))),
		WithRuntimeLogger(mylog.NewDiscardLogger()),
		WithRuntimeStream(DefaultStream()),
		WithRuntimeHasher(&MD5Hasher{}),
		WithRuntimeJail(jail),
	}
	baseOpts = append(baseOpts, opts...)
	return NewRuntime(baseOpts...)
}

func NewOsRuntime() (*Runtime, error) {
	return NewRuntime()
}
