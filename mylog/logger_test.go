package mylog_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jlrickert/cli-toolkit/mylog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"  Debug  ", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"trace", slog.LevelInfo},
		{"  ", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, mylog.ParseLevel(tt.input))
		})
	}
}

func TestNewLogger_TextOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	lg := mylog.NewLogger(mylog.LoggerConfig{
		Version: "1.0.0",
		Out:     &buf,
		Level:   slog.LevelInfo,
		JSON:    false,
		Host:    "testhost",
		PID:     42,
	})

	lg.Info("hello world")
	output := buf.String()

	assert.Contains(t, output, "hello world")
	assert.Contains(t, output, "version=1.0.0")
	assert.Contains(t, output, "host=testhost")
	assert.Contains(t, output, "pid=42")
}

func TestNewLogger_JSONOutput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	lg := mylog.NewLogger(mylog.LoggerConfig{
		Version: "2.0.0",
		Out:     &buf,
		Level:   slog.LevelDebug,
		JSON:    true,
		Host:    "jsonhost",
		PID:     99,
	})

	lg.Info("json test")
	output := buf.String()

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &parsed))

	assert.Equal(t, "json test", parsed["msg"])
	assert.Equal(t, "2.0.0", parsed["version"])
	assert.Equal(t, "jsonhost", parsed["host"])
	assert.EqualValues(t, 99, parsed["pid"])
}

func TestNewLogger_LevelFiltering(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	lg := mylog.NewLogger(mylog.LoggerConfig{
		Out:   &buf,
		Level: slog.LevelWarn,
		Host:  "testhost",
		PID:   1,
	})

	lg.Info("should be filtered")
	lg.Debug("also filtered")
	assert.Empty(t, buf.String(), "info and debug should be filtered at warn level")

	lg.Warn("should appear")
	assert.Contains(t, buf.String(), "should appear")
}

func TestNewLogger_HostAndPIDDefaults(t *testing.T) {
	t.Parallel()

	// When Host and PID are not provided, they should default to OS values.
	// We just verify the logger produces output with non-empty host and non-zero pid.
	var buf bytes.Buffer
	lg := mylog.NewLogger(mylog.LoggerConfig{
		Out:  &buf,
		JSON: true,
		PID:  0,
		Host: "",
	})

	lg.Info("defaults test")

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	host, ok := parsed["host"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, host, "host should have a default value")

	pid, ok := parsed["pid"].(float64)
	require.True(t, ok)
	assert.Greater(t, pid, float64(0), "pid should be a positive number")
}

func TestNewDiscardLogger(t *testing.T) {
	t.Parallel()

	lg := mylog.NewDiscardLogger()
	require.NotNil(t, lg)

	// Logging to a discard logger should not panic.
	lg.Info("this goes nowhere")
	lg.Error("this also goes nowhere")
}

func TestDefault(t *testing.T) {
	t.Parallel()

	lg := mylog.Default()
	require.NotNil(t, lg)

	// Should be usable without panicking.
	lg.Info("test")
}

func TestOrDefault_WithLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	custom := mylog.NewLogger(mylog.LoggerConfig{
		Out:  &buf,
		Host: "custom",
		PID:  1,
	})

	result := mylog.OrDefault(custom)
	assert.Equal(t, custom, result, "OrDefault should return the provided logger")
}

func TestOrDefault_NilReturnsDefault(t *testing.T) {
	t.Parallel()

	result := mylog.OrDefault(nil)
	require.NotNil(t, result)
	// Should be the default discard logger.
	assert.Equal(t, mylog.Default(), result)
}

func TestTestHandler_CapturesEntries(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)

	lg.Info("first message")
	lg.Debug("second message", slog.String("key", "value"))
	lg.Warn("third message")

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	assert.Len(t, entries, 3)
}

func TestTestHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	th := mylog.NewTestHandler(t)
	handler := th.WithAttrs([]slog.Attr{slog.String("service", "myapp")})

	lg := slog.New(handler)
	lg.Info("with attrs test")

	// Entries from WithAttrs handlers are stored on the root handler.
	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	require.Len(t, entries, 1)
	assert.Equal(t, "with attrs test", entries[0].Msg)
	assert.Equal(t, "myapp", entries[0].Attrs["service"])

	// The child handler should also see the entries via FindEntries on the
	// root handler.
	newTH, ok := handler.(*mylog.TestHandler)
	require.True(t, ok)
	_ = newTH // child handler delegates to root
}

func TestTestHandler_WithGroup(t *testing.T) {
	t.Parallel()

	th := mylog.NewTestHandler(t)
	grouped := th.WithGroup("mygroup")

	lg := slog.New(grouped)
	lg.Info("grouped test")

	// Entries from grouped handlers are stored on the root handler.
	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	require.Len(t, entries, 1)
	assert.Equal(t, "grouped test", entries[0].Msg)

	_ = grouped.(*mylog.TestHandler) // type assertion should succeed
}

func TestFindEntries_Predicate(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)

	lg.Debug("debug msg")
	lg.Info("info msg")
	lg.Warn("warn msg")
	lg.Error("error msg")

	warns := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Level == slog.LevelWarn
	})
	require.Len(t, warns, 1)
	assert.Equal(t, "warn msg", warns[0].Msg)

	errors := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Level == slog.LevelError
	})
	require.Len(t, errors, 1)
	assert.Equal(t, "error msg", errors[0].Msg)

	noneFound := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Msg == "nonexistent"
	})
	assert.Empty(t, noneFound)
}

func TestFindEntries_EmptyHandler(t *testing.T) {
	t.Parallel()

	th := mylog.NewTestHandler(t)
	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	assert.Empty(t, entries)
}

func TestRequireEntry_Found(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)

	lg.Info("target message", slog.String("id", "abc123"))

	entry := mylog.RequireEntry(t, th, func(e mylog.LoggedEntry) bool {
		return e.Msg == "target message"
	}, 1*time.Second)

	assert.Equal(t, "target message", entry.Msg)
	assert.Equal(t, slog.LevelInfo, entry.Level)
}

func TestNewTestLogger_HasTestAttribute(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)
	lg.Info("test message")

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Msg == "test message"
	})
	require.Len(t, entries, 1)
	assert.Equal(t, "true", entries[0].Attrs["test"])
}

func TestSlogWriter_SingleLine(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)

	// CallerDepth=0 disables caller attribution for predictable test output.
	w := &mylog.SlogWriter{CallerDepth: 0}
	// Use NewSlogWriter then override CallerDepth.
	w = mylog.NewSlogWriter(lg, slog.LevelWarn)
	w.CallerDepth = 0

	n, err := w.Write([]byte("warning line\n"))
	require.NoError(t, err)
	assert.Equal(t, len("warning line\n"), n)

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return e.Msg == "warning line"
	})
	require.Len(t, entries, 1)
	assert.Equal(t, slog.LevelWarn, entries[0].Level)
}

func TestSlogWriter_MultipleLines(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)
	w := mylog.NewSlogWriter(lg, slog.LevelInfo)
	w.CallerDepth = 0

	_, err := w.Write([]byte("line1\nline2\nline3\n"))
	require.NoError(t, err)

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	require.Len(t, entries, 3)

	msgs := make([]string, len(entries))
	for i, e := range entries {
		msgs[i] = e.Msg
	}
	assert.Contains(t, msgs, "line1")
	assert.Contains(t, msgs, "line2")
	assert.Contains(t, msgs, "line3")
}

func TestSlogWriter_EmptyLinesSkipped(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)
	w := mylog.NewSlogWriter(lg, slog.LevelInfo)
	w.CallerDepth = 0

	_, err := w.Write([]byte("content\n\n\n"))
	require.NoError(t, err)

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return true
	})
	assert.Len(t, entries, 1, "empty lines should be skipped")
}

func TestSlogWriter_CallerDepthEnabled(t *testing.T) {
	t.Parallel()

	lg, th := mylog.NewTestLogger(t, slog.LevelDebug)
	w := mylog.NewSlogWriter(lg, slog.LevelInfo)
	// Default CallerDepth=5 -- should produce caller info.

	_, err := w.Write([]byte("with caller\n"))
	require.NoError(t, err)

	entries := mylog.FindEntries(th, func(e mylog.LoggedEntry) bool {
		return strings.Contains(e.Msg, "with caller")
	})
	require.Len(t, entries, 1)
	// When CallerDepth > 0 and Caller succeeds, a "caller" attr should be present.
	caller, hasCaller := entries[0].Attrs["caller"]
	if hasCaller {
		callerStr, ok := caller.(string)
		assert.True(t, ok)
		assert.Contains(t, callerStr, ":", "caller should contain a file:line reference")
	}
}

func TestTestHandler_Enabled_AllLevels(t *testing.T) {
	t.Parallel()

	th := mylog.NewTestHandler(t)
	assert.True(t, th.Enabled(nil, slog.LevelDebug))
	assert.True(t, th.Enabled(nil, slog.LevelInfo))
	assert.True(t, th.Enabled(nil, slog.LevelWarn))
	assert.True(t, th.Enabled(nil, slog.LevelError))
}
