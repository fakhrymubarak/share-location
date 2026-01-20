package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

// Logger provides leveled logging with optional JSON output.
type Logger struct {
	level int32
	json  bool
	std   *log.Logger
}

const (
	levelDebug = iota
	levelInfo
	levelWarn
	levelError
)

// New creates a leveled logger. Level accepts debug, info, warn, error.
func New(level string, json bool) *Logger {
	l := &Logger{
		level: parseLevel(level),
		json:  json,
		std:   log.New(os.Stdout, "", log.LstdFlags),
	}
	return l
}

// Nop returns a logger that drops all logs.
func Nop() *Logger {
	return &Logger{
		level: levelError + 1,
		std:   log.New(os.Stdout, "", 0),
	}
}

func (l *Logger) SetLevel(level string) {
	atomic.StoreInt32(&l.level, parseLevel(level))
}

func (l *Logger) Debugf(format string, args ...any) { l.logf(levelDebug, "debug", format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.logf(levelInfo, "info", format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.logf(levelWarn, "warn", format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.logf(levelError, "error", format, args...) }

func (l *Logger) logf(lvl int, label, format string, args ...any) {
	if l == nil {
		return
	}
	if lvl < int(atomic.LoadInt32(&l.level)) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if l.json {
		l.std.Printf(`{"level":"%s","msg":%q}`, label, msg)
		return
	}
	l.std.Printf("[%s] %s", strings.ToUpper(label), msg)
}

func parseLevel(level string) int32 {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return levelDebug
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}
