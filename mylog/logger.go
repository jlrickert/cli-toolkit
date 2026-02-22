package mylog

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"log/slog"
)

// Package std provides small logging utilities built on slog for tests and
// simple applications. It includes helpers to create configured loggers,
// utilities to store/retrieve loggers from contexts, and a test handler that
// captures structured log entries for assertions in tests.

var (
	DefaultLogger = NewDiscardLogger()
)

// ParseLevel maps common textual level names to slog.Level. The function is
// case-insensitive and ignores surrounding whitespace. If an unrecognized value
// is provided, slog.LevelInfo is returned.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// LoggerConfig is a minimal, convenient set of options for creating a new
// slog.Logger.
//
// Fields:
//   - Version: application or build version included with each log entry.
//   - Out: destination writer for log output. If nil, os.Stdout is used.
//   - Level: minimum logging level.
//   - JSON: when true, output is JSON; otherwise, human-readable text is used.
type LoggerConfig struct {
	Version string

	// If Out is nil, stdout is used.
	Out io.Writer

	Level  slog.Level
	JSON   bool // true => JSON output, false => text
	Source bool
}

// NewLogger creates a configured *slog.Logger and a shutdown function.
// The shutdown function is a no-op in this implementation but is returned to
// make it easy to add asynchronous or file-based writers later. Call the
// shutdown function on process exit if you add asynchronous writers.
func NewLogger(cfg LoggerConfig) *slog.Logger {
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(
			out,
			&slog.HandlerOptions{Level: cfg.Level, AddSource: cfg.Source})
	} else {
		handler = slog.NewTextHandler(
			out,
			&slog.HandlerOptions{Level: cfg.Level, AddSource: cfg.Source})
	}

	hn, _ := os.Hostname()

	logger := slog.New(handler).With(
		slog.String("version", cfg.Version),
		slog.String("host", hn),
		slog.Int("pid", os.Getpid()),
	)

	// shutdown noop for now
	return logger
}

// NewDiscardLogger returns a logger whose output is discarded. This is useful for
// tests where log output should be suppressed.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

///////////////////////////////////////////////////////////////////////////////
// Context helpers
///////////////////////////////////////////////////////////////////////////////

var defaultLogger = NewDiscardLogger()

// Default returns the process default logger.
func Default() *slog.Logger { return defaultLogger }

// OrDefault returns lg unless it is nil, in which case the default logger is returned.
func OrDefault(lg *slog.Logger) *slog.Logger {
	if lg != nil {
		return lg
	}
	return defaultLogger
}

type SlogWriter struct {
	lg    *slog.Logger
	level slog.Level
}

func (w SlogWriter) Write(p []byte) (int, error) {
	// split into lines to avoid merging multi-line writes
	for line := range strings.SplitSeq(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		// optionally include caller info
		if _, file, lineNo, ok := runtime.Caller(5); ok {
			caller := fmt.Sprintf("%s:%d", file, lineNo)
			w.lg.With("caller", caller).Log(context.Background(), w.level, line)
		} else {
			w.lg.Log(context.Background(), w.level, line)
		}
	}
	return len(p), nil
}
