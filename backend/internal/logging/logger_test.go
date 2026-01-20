package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		level: levelInfo,
		json:  false,
		std:   log.New(&buf, "", 0),
	}
	l.Debugf("ignored")
	l.Infof("hello %s", "world")
	if !strings.Contains(buf.String(), "[INFO] hello world") {
		t.Fatalf("expected info log, got %q", buf.String())
	}
}

func TestJSONFormatting(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		level: levelDebug,
		json:  true,
		std:   log.New(&buf, "", 0),
	}
	l.Warnf("oops")
	out := buf.String()
	if !strings.Contains(out, `"level":"warn"`) || !strings.Contains(out, `"msg":"oops"`) {
		t.Fatalf("expected json log, got %q", out)
	}
}

func TestNopLogger(t *testing.T) {
	nop := Nop()
	var buf bytes.Buffer
	nop.std.SetOutput(&buf)
	nop.Infof("skip me")
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := &Logger{
		level: levelInfo,
		json:  false,
		std:   log.New(&buf, "", 0),
	}
	l.Debugf("skip")
	l.SetLevel("debug")
	l.Debugf("now visible")
	if !strings.Contains(buf.String(), "now visible") {
		t.Fatalf("expected debug log after level change, got %q", buf.String())
	}
}

func TestNewParsesLevelsAndErrorf(t *testing.T) {
	var buf bytes.Buffer
	logger := New("warn", false)
	logger.std.SetOutput(&buf)

	logger.Infof("should skip")
	if buf.Len() != 0 {
		t.Fatalf("expected info to be filtered at warn level")
	}

	logger.Errorf("boom %d", 1)
	if !strings.Contains(buf.String(), "BOOM") && !strings.Contains(strings.ToLower(buf.String()), "boom 1") {
		t.Fatalf("expected error log, got %q", buf.String())
	}

	jsonLogger := New("badlevel", true) // falls back to info
	jsonLogger.std.SetOutput(&buf)
	jsonLogger.Infof("json ok")
	if !strings.Contains(buf.String(), `"level":"error"`) && !strings.Contains(buf.String(), `"level":"info"`) {
		t.Fatalf("expected json output, got %q", buf.String())
	}
}
