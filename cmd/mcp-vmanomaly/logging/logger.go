// Package logging configures the process logger and the HTTP access log.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
)

// Logger wraps slog.Logger and provides a log.Logger adapter for APIs that require it.
type Logger struct {
	slogLogger *slog.Logger
	stdLogger  *log.Logger
}

// New creates a logger that writes to w in the given format ("text" or "json")
// at the given level ("debug", "info", "warn" or "error"), and installs it as the
// slog default and as the output of the standard log package.
func New(w io.Writer, format, level string) (*Logger, error) {
	opts := &slog.HandlerOptions{Level: ParseLevel(level)}
	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("unknown log format %q", format)
	}

	slogger := slog.New(handler)
	slog.SetDefault(slogger)

	return &Logger{
		slogLogger: slogger,
		stdLogger:  slog.NewLogLogger(handler, slog.LevelError),
	}, nil
}

// SlogLogger returns the underlying slog.Logger.
func (l *Logger) SlogLogger() *slog.Logger {
	return l.slogLogger
}

// StdLogger returns a log.Logger that writes errors to the underlying slog handler.
func (l *Logger) StdLogger() *log.Logger {
	return l.stdLogger
}

// ParseLevel converts a configured level name to slog.Level; unknown names mean info.
func ParseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
