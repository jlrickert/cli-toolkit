package toolkit

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	"github.com/jlrickert/cli-toolkit/clock"
)

// ProcessInfo identifies the current process for lock ownership and stale lock
// detection.
type ProcessInfo struct {
	PID       int       // os.Getpid()
	Hostname  string    // os.Hostname()
	StartedAt time.Time // process start time
	UID       string    // unique instance ID (UUID)
}

// NewProcessInfo creates ProcessInfo for the current OS process.
func NewProcessInfo(c clock.Clock) ProcessInfo {
	hostname, _ := os.Hostname()
	return ProcessInfo{
		PID:       os.Getpid(),
		Hostname:  hostname,
		StartedAt: c.Now(),
		UID:       generateUID(),
	}
}

// generateUID returns a random UUID v4 string.
func generateUID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
