// Package mylog provides structured logging utilities built on log/slog.
//
// For production use, [NewLogger] creates a configured slog.Logger with
// text or JSON output, injectable host/PID, and level filtering. For tests,
// [NewTestLogger] returns a logger backed by [TestHandler] which captures
// structured entries for assertions via [FindEntries] and [RequireEntry].
//
// [SlogWriter] adapts a slog.Logger to an io.Writer for use with packages
// that expect traditional writers (e.g., net/http error logs).
package mylog
