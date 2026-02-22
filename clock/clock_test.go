package clock_test

import (
	"testing"
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestClockBasics(t *testing.T) {
	initial := time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC)
	c := clock.NewTestClock(initial)

	// Now should return the seeded time
	assert.Equal(t, initial, c.Now())

	// Advance moves the clock forward
	c.Advance(2 * time.Hour)
	assert.Equal(t, initial.Add(2*time.Hour), c.Now())

	// Set replaces the current time
	newt := time.Date(2021, time.February, 2, 3, 4, 5, 0, time.UTC)
	c.Set(newt)
	assert.Equal(t, newt, c.Now())
}

func TestOrDefaultUsesProvidedClock(t *testing.T) {
	initial := time.Date(2019, time.March, 3, 4, 5, 6, 0, time.UTC)
	tc := clock.NewTestClock(initial)

	got := clock.OrDefault(tc)

	// Ensure OrDefault returns the same TestClock when provided.
	gotTC, ok := got.(*clock.TestClock)
	require.True(t, ok, "expected OrDefault to return *clock.TestClock")
	require.Equal(t, tc, gotTC)
	assert.Equal(t, initial, gotTC.Now())
}

func TestOrDefaultReturnsDefaultWhenNil(t *testing.T) {
	c := clock.OrDefault(nil)
	now := time.Now()
	d := c.Now().Sub(now)
	if d < 0 {
		d = -d
	}
	assert.True(t, d < time.Second, "default clock Now() should be close to time.Now()")
}
