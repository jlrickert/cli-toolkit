package mylog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// Test handler (simple, thread-safe)
///////////////////////////////////////////////////////////////////////////////

// LoggedEntry represents a captured structured log entry for assertions in
// tests. It contains the timestamp, level, message, attributes, source
// location, and performance details.
type LoggedEntry struct {
	Time   time.Time      `json:"time"`
	Level  slog.Level     `json:"level"`
	Msg    string         `json:"msg"`
	Attrs  map[string]any `json:"attrs"`
	Source *slog.Source   `json:"source"`
	PC     uintptr        `json:"pc"`
}

// // MarshalJSON implements json.Marshaler for LoggedEntry.
//
//	func (le LoggedEntry) MarshalJSON() ([]byte, error) {
//		type Alias LoggedEntry
//		return json.Marshal(&struct {
//			Level string `json:"level"`
//			*Alias
//		}{
//			Level: le.Level.String(),
//			Alias: (*Alias)(&le),
//		})
//	}
//
// testingT is a tiny subset of *testing.T used for optional logging from the
// test handler. Only Logf is required.
type testingT interface {
	Logf(format string, args ...any)
}

// TestHandler captures structured entries so tests can assert on logs. It is
// safe for concurrent use.
type TestHandler struct {
	mu      sync.Mutex
	Entries []LoggedEntry
	T       testingT
	attrs   []slog.Attr
	filter  func(entry LoggedEntry) bool
}

// NewTestHandler creates an empty TestHandler. Optionally pass a testing.T
// to have the handler echo captured entries to the test log (via Logf).
func NewTestHandler(t testingT) *TestHandler {
	return &TestHandler{T: t}
}

// Enabled returns true for all levels. Filtering is expected to be handled by
// the caller or the logger's handler options.
func (h *TestHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle captures the provided record as a LoggedEntry and appends it to the
// handler's Entries slice. If a testingT was provided, a human-readable line
// is also logged to the test output.
func (h *TestHandler) Handle(ctx context.Context, r slog.Record) error {
	e := LoggedEntry{
		Time:   r.Time,
		Level:  r.Level,
		Msg:    r.Message,
		Attrs:  map[string]any{},
		Source: r.Source(),
		PC:     r.PC,
	}

	// Add stored attributes from WithAttrs
	for _, attr := range h.attrs {
		e.Attrs[attr.Key] = attr.Value.Any()
	}

	// Add attributes from the record
	r.Attrs(func(a slog.Attr) bool {
		e.Attrs[a.Key] = a.Value.Any()
		return true
	})

	h.mu.Lock()
	h.Entries = append(h.Entries, e)
	h.mu.Unlock()

	if h.filter == nil {
		h.filter = func(entry LoggedEntry) bool { return false }
	}
	if h.T != nil && h.filter(e) {
		data, _ := json.Marshal(e)
		indedentedData := bytes.Buffer{}
		json.Indent(&indedentedData, data, "", "\t")
		h.T.Logf(indedentedData.String())
	}
	return nil
}

// WithAttrs returns a new handler with the provided attributes appended to
// subsequent log entries.
func (h *TestHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &TestHandler{
		T:     h.T,
		attrs: newAttrs,
	}
}

// WithGroup returns a new handler. Grouping is not directly modeled, but the
// handler is returned as a new instance to maintain consistency with slog.Handler.
func (h *TestHandler) WithGroup(_ string) slog.Handler {
	return &TestHandler{
		T:     h.T,
		attrs: h.attrs,
	}
}

// NewTestLogger returns a *slog.Logger that writes to a TestHandler and the
// handler itself for assertions. The returned logger has a default attribute
// ("test"="true") to make it easier to identify test logs.
func NewTestLogger(t testingT, level slog.Level) (
	*slog.Logger, *TestHandler) {
	th := NewTestHandler(t)
	logger := slog.New(th).With(slog.String("test", "true"))
	return logger, th
}

var _ slog.Handler = (*TestHandler)(nil)

///////////////////////////////////////////////////////////////////////////////
// Small helpers for tests
///////////////////////////////////////////////////////////////////////////////

// FindEntries returns a copy of entries from the TestHandler that satisfy the
// provided predicate. The handler's internal slice is copied under lock to
// avoid races.
func FindEntries(th *TestHandler,
	pred func(LoggedEntry) bool) []LoggedEntry {
	th.mu.Lock()
	entries := append([]LoggedEntry(nil), th.Entries...)
	th.mu.Unlock()

	out := make([]LoggedEntry, 0)
	for _, e := range entries {
		if pred(e) {
			out = append(out, e)
		}
	}
	return out
}

// RequireEntry fails the test if a matching entry isn't observed within the
// given timeout. When a matching entry is found it is returned. If the timeout
// elapses, the test is failed and the captured entries are included in the
// failure message to aid debugging.
func RequireEntry(t *testing.T, th *TestHandler,
	pred func(LoggedEntry) bool, timeout time.Duration) LoggedEntry {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		th.mu.Lock()
		for _, e := range th.Entries {
			if pred(e) {
				out := e
				th.mu.Unlock()
				return out
			}
		}
		th.mu.Unlock()
		if time.Now().After(deadline) {
			th.mu.Lock()
			entries := append([]LoggedEntry(nil), th.Entries...)
			th.mu.Unlock()
			t.Fatalf(
				"required log entry not found in %s; captured %d entries: %#v",
				timeout, len(entries), entries,
			)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
