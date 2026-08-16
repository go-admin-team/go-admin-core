package logger

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// Caller resolution is only performed when WithEnableCaller is set. Log
// messages deliberately omit "caller_test.go" so the assertions match the
// caller field rather than the message body.

func TestLogrusCallerInfo(t *testing.T) {
	buf := &bytes.Buffer{}

	// WithPath selects the JSON formatter and writes to disk; use a temp dir
	// to keep the repository clean.
	log := NewLogrusLogger(
		WithLevel(DebugLevel),
		WithOutput(buf),
		WithPath(filepath.Join(t.TempDir(), "test.log")),
		WithEnableCaller(true),
	)

	log.Log(InfoLevel, "message from business code")

	output := buf.String()
	t.Logf("Log output: [%s]", output)

	if len(output) == 0 {
		t.Fatal("Log output is empty!")
	}

	if !strings.Contains(output, "caller") {
		t.Error("Log should contain caller field")
	}

	if !strings.Contains(output, "caller_test.go") {
		t.Error("Caller should show caller_test.go (actual caller)")
	}

	if strings.Contains(output, "logrus.go") {
		t.Error("Caller should not show logrus.go (internal logger file)")
	}
}

func TestLogrusCallerWithLogf(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogrusLogger(
		WithLevel(DebugLevel),
		WithOutput(buf),
		WithEnableCaller(true),
	)

	log.Logf(InfoLevel, "formatted message: %s", "value")

	output := buf.String()
	t.Logf("Log output: %s", output)

	if strings.Contains(output, "logrus.go") {
		t.Error("Logf caller should not show logrus.go")
	}

	if !strings.Contains(output, "caller_test.go") {
		t.Error("Logf caller should show caller_test.go")
	}
}

// helperFunction adds a call frame between the test and the logger.
func helperFunction(log Logger) {
	log.Log(InfoLevel, "message from helper")
}

func TestLogrusCallerWithHelper(t *testing.T) {
	buf := &bytes.Buffer{}
	log := NewLogrusLogger(
		WithLevel(DebugLevel),
		WithOutput(buf),
		WithEnableCaller(true),
	)

	helperFunction(log)

	output := buf.String()
	t.Logf("Log output: %s", output)

	if strings.Contains(output, "logrus.go") {
		t.Error("Helper caller should not show logrus.go")
	}

	if !strings.Contains(output, "caller_test.go") {
		t.Error("Helper caller should show caller_test.go")
	}
}
