package logging

import (
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Logger wraps logrus with a narrowed interface for the app.
type Logger struct {
	entry   *logrus.Entry
	enabled bool
}

// New creates a Logger with the given level and formatter.
func New(level string, json bool) *Logger {
	base := logrus.New()
	base.SetLevel(parseLevel(level))
	if json {
		base.SetFormatter(&logrus.JSONFormatter{})
	}
	base.SetOutput(os.Stdout)
	return &Logger{entry: logrus.NewEntry(base), enabled: true}
}

// Nop returns a logger that discards output.
func Nop() *Logger {
	base := logrus.New()
	base.SetOutput(io.Discard)
	return &Logger{entry: logrus.NewEntry(base), enabled: false}
}

// WithFields returns a derived logger with structured fields.
func (l *Logger) WithFields(fields logrus.Fields) *Logger {
	if l == nil {
		return l
	}
	return &Logger{entry: l.entry.WithFields(fields), enabled: l.enabled}
}

// WithUser adds a user_id field when available.
func (l *Logger) WithUser(id uuid.UUID) *Logger {
	if id == uuid.Nil {
		return l
	}
	return l.WithFields(logrus.Fields{"user_id": id.String()})
}

func (l *Logger) SetLevel(level string) {
	if l == nil {
		return
	}
	l.entry.Logger.SetLevel(parseLevel(level))
}

func (l *Logger) SetOutput(w io.Writer) {
	if l == nil {
		return
	}
	l.entry.Logger.SetOutput(w)
}

func (l *Logger) Debugf(format string, args ...any) {
	if l != nil && l.enabled {
		l.entry.Debugf(format, args...)
	}
}
func (l *Logger) Infof(format string, args ...any) {
	if l != nil && l.enabled {
		l.entry.Infof(format, args...)
	}
}
func (l *Logger) Warnf(format string, args ...any) {
	if l != nil && l.enabled {
		l.entry.Warnf(format, args...)
	}
}
func (l *Logger) Errorf(format string, args ...any) {
	if l != nil && l.enabled {
		l.entry.Errorf(format, args...)
	}
}

func parseLevel(level string) logrus.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return logrus.DebugLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}
