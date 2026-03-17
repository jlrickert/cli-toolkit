package toolkit_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/jlrickert/cli-toolkit/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessInfo(t *testing.T) {
	t.Parallel()

	tc := clock.NewTestClock(time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC))
	info := toolkit.NewProcessInfo(tc)

	assert.Greater(t, info.PID, 0, "PID should be positive")
	assert.NotEmpty(t, info.Hostname, "hostname should not be empty")
	assert.Equal(t, tc.Now(), info.StartedAt, "StartedAt should match clock")
	assert.NotEmpty(t, info.UID, "UID should not be empty")
}

func TestNewProcessInfo_UIDFormat(t *testing.T) {
	t.Parallel()

	tc := clock.NewTestClock(time.Now())
	info := toolkit.NewProcessInfo(tc)

	// UUID v4 format: 8-4-4-4-12 hex chars.
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	assert.True(t, uuidPattern.MatchString(info.UID),
		"UID should be a valid UUID v4, got: %s", info.UID)
}

func TestNewProcessInfo_UniqueUIDs(t *testing.T) {
	t.Parallel()

	tc := clock.NewTestClock(time.Now())
	info1 := toolkit.NewProcessInfo(tc)
	info2 := toolkit.NewProcessInfo(tc)

	assert.NotEqual(t, info1.UID, info2.UID, "each call should produce a unique UID")
}

func TestWithProcessInfo(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	tc := clock.NewTestClock(time.Now())
	info := toolkit.NewProcessInfo(tc)

	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser",
		toolkit.WithProcessInfo(info),
	)
	require.NoError(t, err)

	got := rt.Process()
	require.NotNil(t, got)
	assert.Equal(t, info.PID, got.PID)
	assert.Equal(t, info.Hostname, got.Hostname)
	assert.Equal(t, info.UID, got.UID)
}

func TestRuntime_Process_NilByDefault(t *testing.T) {
	t.Parallel()

	jail := t.TempDir()
	rt, err := toolkit.NewTestRuntime(jail, "/home/testuser", "testuser")
	require.NoError(t, err)

	assert.Nil(t, rt.Process(), "Process should be nil when not configured")
}
