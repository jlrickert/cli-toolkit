package toolkit

import (
	"io"
	"os"
)

// Stream models the standard IO streams and common stream properties.
//
// Struct field tags are included for clarity to external consumers that may
// wish to encode some stream metadata. The actual reader and writer fields
// are not suitable for encoding and therefore are tagged to be ignored.
type Stream struct {
	// In is the input stream, typically os.Stdin.
	In io.Reader
	// Out is the output stream, typically os.Stdout.
	Out io.Writer
	// Err is the error stream, typically os.Stderr.
	Err io.Writer

	// IsPiped indicates whether stdin appears to be piped or redirected.
	IsPiped bool
	// IsTTY indicates whether stdout refers to a terminal.
	IsTTY bool
}

// DefaultStream returns a Stream configured with the real process
// standard input, output, and error streams. It detects whether stdin
// is piped and whether stdout is a terminal.
func DefaultStream() *Stream {
	return &Stream{
		In:      os.Stdin,
		Out:     os.Stdout,
		Err:     os.Stderr,
		IsPiped: StdinHasData(os.Stdin),
		IsTTY:   IsInteractiveTerminal(os.Stdout),
	}
}

// OrDefaultStream returns s unless it is nil, in which case DefaultStream() is returned.
func OrDefaultStream(s *Stream) *Stream {
	if s != nil {
		return s
	}
	return DefaultStream()
}
