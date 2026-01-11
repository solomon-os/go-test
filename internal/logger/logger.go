// Package logger provides structured logging functionality.
package logger

import (
	"io"
	"log/slog"
	"os"
)

// Level represents the logging level.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Logger wraps slog.Logger for structured logging.
type Logger struct {
	*slog.Logger
}

var defaultLogger = New(os.Stderr, LevelInfo)

// New creates a new Logger with the specified output and level.
func New(w io.Writer, level Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(w, opts)
	return &Logger{Logger: slog.New(handler)}
}

// NewJSON creates a new Logger with JSON output format.
func NewJSON(w io.Writer, level Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(w, opts)
	return &Logger{Logger: slog.New(handler)}
}

// Default returns the default logger.
func Default() *Logger {
	return defaultLogger
}

// SetDefault sets the default logger.
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// With returns a new Logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...)}
}

// WithGroup returns a new Logger with the given group name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{Logger: l.Logger.WithGroup(name)}
}
