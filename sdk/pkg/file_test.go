package pkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileCreateWritesContent(t *testing.T) {
	name := filepath.Join(t.TempDir(), "out.go")
	if err := FileCreate(*bytes.NewBufferString("package main\n"), name); err != nil {
		t.Fatalf("FileCreate: %v", err)
	}
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package main\n" {
		t.Fatalf("content = %q, want %q", got, "package main\n")
	}
}

func TestFileCreateTruncatesAnExistingFile(t *testing.T) {
	name := filepath.Join(t.TempDir(), "out.go")
	if err := os.WriteFile(name, []byte("a much longer previous body\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := FileCreate(*bytes.NewBufferString("short\n"), name); err != nil {
		t.Fatalf("FileCreate: %v", err)
	}
	got, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "short\n" {
		t.Fatalf("content = %q, want %q", got, "short\n")
	}
}

// A file that cannot be created is reported to the caller. Before FileCreate
// returned an error, this case closed a nil file and called log.Fatalln, so
// reaching the assertion at all is part of what this test checks.
func TestFileCreateReportsAFileThatCannotBeCreated(t *testing.T) {
	name := filepath.Join(t.TempDir(), "missing-dir", "out.go")
	if err := FileCreate(*bytes.NewBufferString("x"), name); err == nil {
		t.Fatal("FileCreate into a missing directory returned nil")
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatalf("stat after failed FileCreate: %v, want not-exist", err)
	}
}
