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

// defaultLogger is the package-level default logger used by Default() and
// OrDefault(). It discards all output.
var defaultLogger = NewDiscardLogger()

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
//   - Host: hostname included with each log entry. If empty, os.Hostname() is used.
//   - PID: process ID included with each log entry. If zero, os.Getpid() is used.
type LoggerConfig struct {
	Version string

	// If Out is nil, stdout is used.
	Out io.Writer

	Level  slog.Level
	JSON   bool // true => JSON output, false => text
	Source bool

	// Host overrides os.Hostname() when set. Use this in tests or when
	// the hostname should be injected rather than discovered at runtime.
	Host string

	// PID overrides os.Getpid() when set. Use this in tests or
	// containerized environments where the OS PID is not meaningful.
	PID int
}

// NewLogger creates a configured *slog.Logger. Host and PID default to
// os.Hostname() and os.Getpid() when not provided via LoggerConfig.
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

	host := cfg.Host
	if host == "" {
		host, _ = os.Hostname()
	}

	pid := cfg.PID
	if pid == 0 {
		pid = os.Getpid()
	}

	logger := slog.New(handler).With(
		slog.String("version", cfg.Version),
		slog.String("host", host),
		slog.Int("pid", pid),
	)

	return logger
}

// NewDiscardLogger returns a logger whose output is discarded. This is useful for
// tests where log output should be suppressed.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// Default returns the package-level default logger (a discard logger).
func Default() *slog.Logger { return defaultLogger }

// OrDefault returns lg unless it is nil, in which case the default logger is returned.
func OrDefault(lg *slog.Logger) *slog.Logger {
	if lg != nil {
		return lg
	}
	return defaultLogger
}

// SlogWriter adapts a slog.Logger to an io.Writer. Each line written is
// logged as a separate entry at the configured level.
type SlogWriter struct {
	lg    *slog.Logger
	level slog.Level

	// CallerDepth controls the stack skip for runtime.Caller when
	// attaching caller information. Zero disables caller attribution.
	// The default (when created via NewSlogWriter) is 5 which matches
	// the typical log.Logger -> Write -> slog pipeline depth.
	CallerDepth int
}

// NewSlogWriter creates a SlogWriter with sensible defaults.
func NewSlogWriter(lg *slog.Logger, level slog.Level) *SlogWriter {
	return &SlogWriter{lg: lg, level: level, CallerDepth: 5}
}

func (w SlogWriter) Write(p []byte) (int, error) {
	// split into lines to avoid merging multi-line writes
	for line := range strings.SplitSeq(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		if w.CallerDepth > 0 {
			if _, file, lineNo, ok := runtime.Caller(w.CallerDepth); ok {
				caller := fmt.Sprintf("%s:%d", file, lineNo)
				w.lg.With("caller", caller).Log(context.Background(), w.level, line)
				continue
			}
		}
		w.lg.Log(context.Background(), w.level, line)
	}
	return len(p), nil
}
